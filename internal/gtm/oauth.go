// Package gtm は認証と Google Tag Manager API v2 クライアントの生成・操作を担う。
//
// 認証は OAuth2 Installed App（ローカルループバック）で refresh token を得る方式。
package gtm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// ReadOnlyScopes は参照だけに使うスコープ（`auth login --readonly`）。
// 預かった他社コンテナの調査など、書き込み権限を持たせたくないときに使う。
var ReadOnlyScopes = []string{
	"https://www.googleapis.com/auth/tagmanager.readonly",
}

// Scopes は既定で要求するスコープ。参照・ワークスペースの編集・バージョン作成・公開・
// ワークスペースの削除まで。アカウント/ユーザー管理とコンテナの削除は要求しない。
var Scopes = []string{
	"https://www.googleapis.com/auth/tagmanager.readonly",
	"https://www.googleapis.com/auth/tagmanager.edit.containers",
	"https://www.googleapis.com/auth/tagmanager.edit.containerversions",
	"https://www.googleapis.com/auth/tagmanager.publish",
	"https://www.googleapis.com/auth/tagmanager.delete.containers",
}

// oauthConfig はクライアントID/シークレットから oauth2.Config を組み立てる。
// redirectURL は認証フローでのみ使用するため引数で受ける。
func oauthConfig(clientID, clientSecret, redirectURL string, scopes []string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  redirectURL,
		Scopes:       scopes,
	}
}

// TokenSource は refresh token から自動更新されるトークンソースを返す。
// 更新時にスコープは送られない（refresh token に許可済みのものがそのまま効く）。
func TokenSource(ctx context.Context, clientID, clientSecret, refreshToken string) oauth2.TokenSource {
	cfg := oauthConfig(clientID, clientSecret, "", Scopes)
	return cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
}

// LoginResult は対話的認証フローの結果。
type LoginResult struct {
	RefreshToken string
}

// InteractiveLogin はローカルループバックを使った Installed App 認証フローを実行する。
//
//  1. 127.0.0.1 の空きポートでコールバック用 HTTP サーバを起動
//  2. 認証URLをブラウザで開く（開けなければURLを表示）
//  3. リダイレクトで受け取った認可コードをトークンに交換
//  4. refresh token を取り出して返す
func InteractiveLogin(ctx context.Context, clientID, clientSecret string, scopes []string, openBrowser bool) (*LoginResult, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start the callback server: %w", err)
	}
	defer ln.Close()

	redirectURL := fmt.Sprintf("http://%s/", ln.Addr().String())
	cfg := oauthConfig(clientID, clientSecret, redirectURL, scopes)

	state, err := randomState()
	if err != nil {
		return nil, err
	}

	// refresh token を確実に得るため offline + consent を要求する。
	authURL := cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			fmt.Fprintf(w, "Authentication failed: %s. Please return to the terminal.", e)
			errCh <- fmt.Errorf("error from the authorization server: %s", e)
			return
		}
		if q.Get("state") != state {
			http.Error(w, "state does not match", http.StatusBadRequest)
			errCh <- fmt.Errorf("state mismatch: possible CSRF")
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "no authorization code", http.StatusBadRequest)
			errCh <- fmt.Errorf("failed to receive an authorization code")
			return
		}
		fmt.Fprint(w, "Authentication complete. You can close this tab and return to the terminal.")
		codeCh <- code
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Println("Open the following URL in your browser to authenticate:")
	fmt.Println()
	fmt.Println("  " + authURL)
	fmt.Println()
	if openBrowser {
		_ = browse(authURL)
	}
	fmt.Println("Waiting for authentication to complete...")

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("authentication timed out (5 minutes)")
	}

	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange the authorization code for a token: %w", err)
	}
	if tok.RefreshToken == "" {
		return nil, fmt.Errorf("failed to obtain a refresh token. Revoke this app's access in your Google account, then try again")
	}
	return &LoginResult{RefreshToken: tok.RefreshToken}, nil
}

// browse は OS 既定のブラウザでURLを開く。
func browse(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	return exec.Command(cmd, args...).Start()
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}
	return hex.EncodeToString(b), nil
}
