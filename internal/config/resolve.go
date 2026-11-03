package config

import (
	"fmt"
	"os"
	"strings"
)

// Overrides はコマンドラインフラグ由来の上書き値。空文字は「指定なし」。
type Overrides struct {
	Account      string
	Container    string
	Workspace    string
	ClientID     string
	ClientSecret string
}

// Resolved は各種ソースをマージした実効設定。
//
// Container は数字のコンテナID、または公開ID（GTM-XXXX）のまま残る。
// 公開IDの解決は API 呼び出し（containers.lookup）が要るため gtm パッケージで行う。
type Resolved struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
	Scopes       []string

	AccountID   string
	Container   string
	WorkspaceID string
}

// Resolve は「フラグ > 環境変数 > ファイル」の優先度で実効設定を組み立てる。
//
// ワークスペースだけは注意が要る。保存済みのワークスペースIDは保存済みの
// コンテナに属するので、フラグや環境変数で別のコンテナを指したときには使わない
// （別コンテナのワークスペースIDを渡すと 404 になるだけでなく、偶然一致すると
// 意図しないコンテナを編集する）。
func Resolve(o Overrides) (*Resolved, error) {
	creds, err := LoadCredentials()
	if err != nil {
		return nil, err
	}

	var fileClientID, fileClientSecret string
	if oc, err := LoadOAuthClient(); err == nil {
		fileClientID, fileClientSecret = oc.ClientID, oc.ClientSecret
	}

	r := &Resolved{
		ClientID:     firstNonEmpty(o.ClientID, os.Getenv(envClientID), fileClientID),
		ClientSecret: firstNonEmpty(o.ClientSecret, os.Getenv(envClientSecret), fileClientSecret),
		RefreshToken: creds.RefreshToken,
		Scopes:       creds.Scopes,
	}

	// コンテナ指定は "accounts/1/containers/2" のパスでもよい。その場合はアカウントも決まる。
	containerArg := firstNonEmpty(o.Container, os.Getenv(envContainerID))
	accountArg := firstNonEmpty(o.Account, os.Getenv(envAccountID))
	if containerArg != "" {
		acc, cont := ParseContainerRef(containerArg)
		r.Container = cont
		r.AccountID = firstNonEmpty(acc, NormalizeID(accountArg))
		// 保存済みのアカウントは、指定されたコンテナが保存済みと同じときだけ補う。
		if r.AccountID == "" && cont == creds.ContainerID {
			r.AccountID = creds.AccountID
		}
	} else {
		r.Container = creds.ContainerID
		r.AccountID = firstNonEmpty(NormalizeID(accountArg), creds.AccountID)
	}

	wsArg := firstNonEmpty(o.Workspace, os.Getenv(envWorkspaceID))
	switch {
	case wsArg != "":
		r.WorkspaceID = NormalizeID(wsArg)
	case r.Container != "" && r.Container == creds.ContainerID:
		r.WorkspaceID = creds.WorkspaceID
	}
	return r, nil
}

// RequireForAPI は API 呼び出しに最低限必要な値（認証）が揃っているか検証する。
func (r *Resolved) RequireForAPI() error {
	var missing []string
	if r.RefreshToken == "" {
		missing = append(missing, "refresh token (run `gtmc auth login`)")
	}
	if r.ClientID == "" || r.ClientSecret == "" {
		missing = append(missing, "OAuth client (oauth2-client.json or --client-id/--client-secret)")
	}
	if len(missing) > 0 {
		return fmt.Errorf("credentials are missing:\n  - %s", strings.Join(missing, "\n  - "))
	}
	return nil
}

// IsPublicID は GTM-XXXX 形式（コンテナの公開ID）かどうかを返す。
func IsPublicID(s string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s)), "GTM-")
}

// ParseContainerRef はコンテナ指定を（アカウントID, コンテナ）に分解する。
//
//	"accounts/1/containers/2/..." → ("1", "2")
//	"2"                           → ("",  "2")
//	"GTM-ABC123"                  → ("",  "GTM-ABC123")
func ParseContainerRef(s string) (account, container string) {
	s = strings.TrimSpace(s)
	if IsPublicID(s) {
		return "", strings.ToUpper(s)
	}
	if strings.Contains(s, "/") {
		parts := strings.Split(strings.Trim(s, "/"), "/")
		for i := 0; i+1 < len(parts); i++ {
			switch parts[i] {
			case "accounts":
				account = parts[i+1]
			case "containers":
				container = parts[i+1]
			}
		}
		return account, container
	}
	return "", NormalizeID(s)
}

// NormalizeID は "accounts/123" のようなリソース名や空白混じりから末尾の数字IDを取り出す。
func NormalizeID(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return s
}
