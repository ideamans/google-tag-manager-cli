// Package config は CLI の設定ファイルと認証情報の読み書きを担う。
//
// 設定は ~/.config/google-tag-manager/ 配下に保存する。
//
//	oauth2-client.json  ... Google Cloud の OAuth2 クライアント情報（Desktop app）
//	credentials.json    ... 認証で得た refresh token と、既定のアカウント/コンテナ/ワークスペース
//
// 各値の解決優先度は「コマンドラインフラグ > 環境変数 > 設定ファイル」とする。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	appDirName          = "google-tag-manager"
	oauthClientFileName = "oauth2-client.json"
	credentialsFileName = "credentials.json"
	envAccountID        = "GTM_ACCOUNT_ID"
	envContainerID      = "GTM_CONTAINER_ID"
	envWorkspaceID      = "GTM_WORKSPACE_ID"
	envClientID         = "GTM_CLIENT_ID"
	envClientSecret     = "GTM_CLIENT_SECRET"
	envConfigDir        = "GTM_HOME"
)

// OAuthClient は Google Cloud で発行した OAuth2 クライアント情報。
type OAuthClient struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// Credentials は認証フローの結果と既定値を保持する。
type Credentials struct {
	RefreshToken string   `json:"refresh_token,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	AccountID    string   `json:"account_id,omitempty"`
	ContainerID  string   `json:"container_id,omitempty"`
	// PublicID はコンテナの公開ID（GTM-XXXX）。表示用。
	PublicID    string `json:"public_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

type installedClientFile struct {
	Installed *OAuthClient `json:"installed"`
	Web       *OAuthClient `json:"web"`
}

// Dir は設定ディレクトリの絶対パスを返す。
// 優先度: GTM_HOME > $XDG_CONFIG_HOME/google-tag-manager > ~/.config/google-tag-manager。
func Dir() (string, error) {
	if d := os.Getenv(envConfigDir); d != "" {
		return d, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to determine the home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, appDirName), nil
}

func ensureDir() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create the config directory (%s): %w", dir, err)
	}
	return nil
}

// OAuthClientPath は oauth2-client.json のパスを返す。
func OAuthClientPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, oauthClientFileName), nil
}

// CredentialsPath は credentials.json のパスを返す。
func CredentialsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, credentialsFileName), nil
}

// LoadOAuthClient は oauth2-client.json を読み込む。
// Google がダウンロードさせる {"installed": {...}} 形式と、
// フラットな {"client_id":..., "client_secret":...} 形式の両方を受け付ける。
func LoadOAuthClient() (*OAuthClient, error) {
	path, err := OAuthClientPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("OAuth client credentials not found: %s\n  Create a Desktop app client in the Google Cloud Console and save the JSON to this path", path)
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	return parseOAuthClient(path, data)
}

func parseOAuthClient(path string, data []byte) (*OAuthClient, error) {
	var wrapped installedClientFile
	if err := json.Unmarshal(data, &wrapped); err == nil {
		if wrapped.Installed != nil && wrapped.Installed.ClientID != "" {
			return wrapped.Installed, nil
		}
		if wrapped.Web != nil && wrapped.Web.ClientID != "" {
			return wrapped.Web, nil
		}
	}
	var flat OAuthClient
	if err := json.Unmarshal(data, &flat); err != nil {
		return nil, fmt.Errorf("failed to parse the JSON in %s: %w", path, err)
	}
	if flat.ClientID == "" {
		return nil, fmt.Errorf("%s has no client_id", path)
	}
	return &flat, nil
}

// LoadCredentials は credentials.json を読み込む。存在しなければ空の値を返す。
func LoadCredentials() (*Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Credentials{}, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("failed to parse the JSON in %s: %w", path, err)
	}
	return &c, nil
}

// SaveCredentials は credentials.json を書き出す（パーミッション 0600）。
func SaveCredentials(c *Credentials) error {
	if err := ensureDir(); err != nil {
		return err
	}
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to generate JSON: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
