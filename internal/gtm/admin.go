package gtm

import (
	"context"
	"fmt"
	"strings"

	tm "google.golang.org/api/tagmanager/v2"

	"github.com/ideamans/google-tag-manager-cli/internal/config"
)

// SetTarget は操作対象を差し替え、解決結果のキャッシュを捨てる。
// `container use <ref>` のように、引数でコンテナを受けるコマンドが使う。
func (c *Client) SetTarget(account, container, workspace string) {
	c.accountID, c.container, c.workspaceID = account, container, workspace
	c.resolvedContainer = nil
}

// WorkspaceID は解決済み（または指定済み）のワークスペースIDを返す。
func (c *Client) WorkspaceID() string { return c.workspaceID }

// ListAccounts はアクセスできる GTM アカウントを返す。
func (c *Client) ListAccounts(ctx context.Context) ([]*tm.Account, error) {
	var out []*tm.Account
	err := c.do(ctx, "accounts.list", func() error {
		out = nil
		return c.API.Accounts.List().Pages(ctx, func(r *tm.ListAccountsResponse) error {
			out = append(out, r.Account...)
			return nil
		})
	})
	return out, err
}

// GetAccount はアカウントを1つ返す。
func (c *Client) GetAccount(ctx context.Context, path string) (*tm.Account, error) {
	var v *tm.Account
	err := c.do(ctx, "accounts.get", func() (e error) {
		v, e = c.API.Accounts.Get(path).Context(ctx).Do()
		return e
	})
	return v, err
}

// ListContainers はアカウント配下のコンテナを返す。
func (c *Client) ListContainers(ctx context.Context, accountPath string) ([]*tm.Container, error) {
	var out []*tm.Container
	err := c.do(ctx, "containers.list", func() error {
		out = nil
		return c.API.Accounts.Containers.List(accountPath).Pages(ctx, func(r *tm.ListContainersResponse) error {
			out = append(out, r.Container...)
			return nil
		})
	})
	return out, err
}

// ListAllContainers はアクセスできる全アカウントのコンテナを返す。
func (c *Client) ListAllContainers(ctx context.Context) ([]*tm.Container, error) {
	accounts, err := c.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	var out []*tm.Container
	for _, a := range accounts {
		cs, err := c.ListContainers(ctx, a.Path)
		if err != nil {
			return nil, err
		}
		out = append(out, cs...)
	}
	return out, nil
}

// ---- ワークスペース ----

// GetWorkspace はワークスペースを1つ返す。
func (c *Client) GetWorkspace(ctx context.Context, path string) (*tm.Workspace, error) {
	var v *tm.Workspace
	err := c.do(ctx, "workspaces.get", func() (e error) {
		v, e = c.API.Accounts.Containers.Workspaces.Get(path).Context(ctx).Do()
		return e
	})
	return v, err
}

// CreateWorkspace はワークスペースを作る。
func (c *Client) CreateWorkspace(ctx context.Context, containerPath, name, description string) (*tm.Workspace, error) {
	var v *tm.Workspace
	err := c.do(ctx, "workspaces.create", func() (e error) {
		v, e = c.API.Accounts.Containers.Workspaces.Create(containerPath, &tm.Workspace{Name: name, Description: description}).Context(ctx).Do()
		return e
	})
	return v, err
}

// DeleteWorkspace はワークスペースを削除する（未保存の変更ごと消える）。
func (c *Client) DeleteWorkspace(ctx context.Context, path string) error {
	return c.do(ctx, "workspaces.delete", func() error {
		return c.API.Accounts.Containers.Workspaces.Delete(path).Context(ctx).Do()
	})
}

// WorkspaceStatus はワークスペースの未保存の変更と競合を返す。
func (c *Client) WorkspaceStatus(ctx context.Context, path string) (*tm.GetWorkspaceStatusResponse, error) {
	var v *tm.GetWorkspaceStatusResponse
	err := c.do(ctx, "workspaces.getStatus", func() (e error) {
		v, e = c.API.Accounts.Containers.Workspaces.GetStatus(path).Context(ctx).Do()
		return e
	})
	return v, err
}

// SyncWorkspace は最新バージョンの変更をワークスペースへ取り込む。
func (c *Client) SyncWorkspace(ctx context.Context, path string) (*tm.SyncWorkspaceResponse, error) {
	var v *tm.SyncWorkspaceResponse
	err := c.do(ctx, "workspaces.sync", func() (e error) {
		v, e = c.API.Accounts.Containers.Workspaces.Sync(path).Context(ctx).Do()
		return e
	})
	return v, err
}

// ---- バージョン ----

// ListVersionHeaders はバージョンの見出し（中身なし）を返す。
func (c *Client) ListVersionHeaders(ctx context.Context, containerPath string, includeDeleted bool) ([]*tm.ContainerVersionHeader, error) {
	var out []*tm.ContainerVersionHeader
	err := c.do(ctx, "version_headers.list", func() error {
		out = nil
		return c.API.Accounts.Containers.VersionHeaders.List(containerPath).IncludeDeleted(includeDeleted).Pages(ctx, func(r *tm.ListContainerVersionsResponse) error {
			out = append(out, r.ContainerVersionHeader...)
			return nil
		})
	})
	return out, err
}

// CreateVersion はワークスペースからバージョンを作る。
//
// GTM の仕様で、バージョンにしたワークスペースは消費される（消える）。
// 応答の NewWorkspacePath が次に使うワークスペース。
func (c *Client) CreateVersion(ctx context.Context, wsPath, name, notes string) (*tm.CreateContainerVersionResponse, error) {
	var v *tm.CreateContainerVersionResponse
	err := c.do(ctx, "workspaces.create_version", func() (e error) {
		v, e = c.API.Accounts.Containers.Workspaces.CreateVersion(wsPath, &tm.CreateContainerVersionRequestVersionOptions{Name: name, Notes: notes}).Context(ctx).Do()
		return e
	})
	if err != nil {
		return nil, err
	}
	if v.CompilerError {
		return v, fmt.Errorf("the workspace did not compile; no version was created (see compilerErrors)")
	}
	return v, nil
}

// PublishVersion はバージョンを公開する（サイトで動くものが置き換わる）。
func (c *Client) PublishVersion(ctx context.Context, path string) (*tm.PublishContainerVersionResponse, error) {
	var v *tm.PublishContainerVersionResponse
	err := c.do(ctx, "versions.publish", func() (e error) {
		v, e = c.API.Accounts.Containers.Versions.Publish(path).Context(ctx).Do()
		return e
	})
	if err != nil {
		return nil, err
	}
	if v.CompilerError {
		return v, fmt.Errorf("the version did not compile; nothing was published (see compilerErrors)")
	}
	return v, nil
}

// VersionPath は live/latest/数字 のバージョン指定をパスに解決する。
func (c *Client) VersionPath(ctx context.Context, ref string) (string, error) {
	cp, err := c.ContainerPath(ctx)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(ref) {
	case "", "live":
		v, err := c.Snapshot(ctx, "live")
		if err != nil {
			return "", err
		}
		return v.Path, nil
	case "latest":
		var h *tm.ContainerVersionHeader
		err := c.do(ctx, "version_headers.latest", func() (e error) {
			h, e = c.API.Accounts.Containers.VersionHeaders.Latest(cp).Context(ctx).Do()
			return e
		})
		if err != nil {
			return "", err
		}
		return h.Path, nil
	}
	return cp + "/versions/" + config.NormalizeID(ref), nil
}

// UpdateWorkspace はワークスペースの名前と説明を変える（全置換なので取得してから当てる）。
func (c *Client) UpdateWorkspace(ctx context.Context, path string, name, description *string) (*tm.Workspace, error) {
	cur, err := c.GetWorkspace(ctx, path)
	if err != nil {
		return nil, err
	}
	if name != nil {
		cur.Name = *name
	}
	if description != nil {
		cur.Description = *description
		cur.ForceSendFields = append(cur.ForceSendFields, "Description")
	}
	var v *tm.Workspace
	err = c.do(ctx, "workspaces.update", func() (e error) {
		v, e = c.API.Accounts.Containers.Workspaces.Update(path, cur).Fingerprint(cur.Fingerprint).Context(ctx).Do()
		return e
	})
	return v, err
}

// QuickPreview はバージョンを作らずにワークスペースをコンパイルして確かめる。
func (c *Client) QuickPreview(ctx context.Context, path string) (*tm.QuickPreviewResponse, error) {
	var v *tm.QuickPreviewResponse
	err := c.do(ctx, "workspaces.quick_preview", func() (e error) {
		v, e = c.API.Accounts.Containers.Workspaces.QuickPreview(path).Context(ctx).Do()
		return e
	})
	return v, err
}

// ResolveConflict はマージ競合1件を、渡したエンティティの内容で解消する。
// fingerprint は競合の entityInWorkspace 側のもの（空なら検査しない）。
func (c *Client) ResolveConflict(ctx context.Context, path string, e *tm.Entity, fingerprint string) error {
	return c.do(ctx, "workspaces.resolve_conflict", func() error {
		call := c.API.Accounts.Containers.Workspaces.ResolveConflict(path, e)
		if fingerprint != "" {
			call = call.Fingerprint(fingerprint)
		}
		return call.Context(ctx).Do()
	})
}

// BulkUpdate は複数の変更を1回で適用する。新規作成のエンティティは "new_1" のような
// 仮 ID を持ち、同じ一括変更の中でその仮 ID を参照できる。
func (c *Client) BulkUpdate(ctx context.Context, path string, changes *tm.ProposedChange) (*tm.BulkUpdateWorkspaceResponse, error) {
	var v *tm.BulkUpdateWorkspaceResponse
	err := c.do(ctx, "workspaces.bulk_update", func() (e error) {
		v, e = c.API.Accounts.Containers.Workspaces.BulkUpdate(path, changes).Context(ctx).Do()
		return e
	})
	return v, err
}

// UpdateVersion はバージョンの名前と説明を変える。
func (c *Client) UpdateVersion(ctx context.Context, path string, name, notes *string) (*tm.ContainerVersion, error) {
	var cur *tm.ContainerVersion
	err := c.do(ctx, "versions.get", func() (e error) {
		cur, e = c.API.Accounts.Containers.Versions.Get(path).Context(ctx).Do()
		return e
	})
	if err != nil {
		return nil, err
	}
	if name != nil {
		cur.Name = *name
	}
	if notes != nil {
		cur.Description = *notes
		cur.ForceSendFields = append(cur.ForceSendFields, "Description")
	}
	var v *tm.ContainerVersion
	err = c.do(ctx, "versions.update", func() (e error) {
		v, e = c.API.Accounts.Containers.Versions.Update(path, cur).Fingerprint(cur.Fingerprint).Context(ctx).Do()
		return e
	})
	return v, err
}

// DeleteVersion はバージョンを削除する（undelete で戻せる）。
func (c *Client) DeleteVersion(ctx context.Context, path string) error {
	return c.do(ctx, "versions.delete", func() error {
		return c.API.Accounts.Containers.Versions.Delete(path).Context(ctx).Do()
	})
}

// UndeleteVersion は削除したバージョンを戻す。
func (c *Client) UndeleteVersion(ctx context.Context, path string) (*tm.ContainerVersion, error) {
	var v *tm.ContainerVersion
	err := c.do(ctx, "versions.undelete", func() (e error) {
		v, e = c.API.Accounts.Containers.Versions.Undelete(path).Context(ctx).Do()
		return e
	})
	return v, err
}

// SetLatestVersion はワークスペースの同期・競合判定の基準にするバージョンを変える。
func (c *Client) SetLatestVersion(ctx context.Context, path string) (*tm.ContainerVersion, error) {
	var v *tm.ContainerVersion
	err := c.do(ctx, "versions.set_latest", func() (e error) {
		v, e = c.API.Accounts.Containers.Versions.SetLatest(path).Context(ctx).Do()
		return e
	})
	return v, err
}

// ---- コンテナ ----

// Snippet はサイトに貼る設置コードを返す。
func (c *Client) Snippet(ctx context.Context, containerPath string) (*tm.GetContainerSnippetResponse, error) {
	var v *tm.GetContainerSnippetResponse
	err := c.do(ctx, "containers.snippet", func() (e error) {
		v, e = c.API.Accounts.Containers.Snippet(containerPath).Context(ctx).Do()
		return e
	})
	return v, err
}

// ---- 環境 ----

// ListEnvironments はコンテナの環境（Live・Latest・ユーザー定義）を返す。
func (c *Client) ListEnvironments(ctx context.Context, containerPath string) ([]*tm.Environment, error) {
	var out []*tm.Environment
	err := c.do(ctx, "environments.list", func() error {
		out = nil
		return c.API.Accounts.Containers.Environments.List(containerPath).Pages(ctx, func(r *tm.ListEnvironmentsResponse) error {
			out = append(out, r.Environment...)
			return nil
		})
	})
	return out, err
}

// GetEnvironment は環境を1つ返す。
func (c *Client) GetEnvironment(ctx context.Context, path string) (*tm.Environment, error) {
	var v *tm.Environment
	err := c.do(ctx, "environments.get", func() (e error) {
		v, e = c.API.Accounts.Containers.Environments.Get(path).Context(ctx).Do()
		return e
	})
	return v, err
}

// CreateEnvironment はユーザー定義の環境を作る。
func (c *Client) CreateEnvironment(ctx context.Context, containerPath string, env *tm.Environment) (*tm.Environment, error) {
	var v *tm.Environment
	err := c.do(ctx, "environments.create", func() (e error) {
		v, e = c.API.Accounts.Containers.Environments.Create(containerPath, env).Context(ctx).Do()
		return e
	})
	return v, err
}

// UpdateEnvironment は環境を取得し、patch を当てて保存する。
func (c *Client) UpdateEnvironment(ctx context.Context, path string, patch func(*tm.Environment)) (*tm.Environment, error) {
	cur, err := c.GetEnvironment(ctx, path)
	if err != nil {
		return nil, err
	}
	patch(cur)
	var v *tm.Environment
	err = c.do(ctx, "environments.update", func() (e error) {
		v, e = c.API.Accounts.Containers.Environments.Update(path, cur).Fingerprint(cur.Fingerprint).Context(ctx).Do()
		return e
	})
	return v, err
}

// DeleteEnvironment は環境を削除する。
func (c *Client) DeleteEnvironment(ctx context.Context, path string) error {
	return c.do(ctx, "environments.delete", func() error {
		return c.API.Accounts.Containers.Environments.Delete(path).Context(ctx).Do()
	})
}

// ReauthorizeEnvironment は環境の認証コードを作り直す（古いプレビューリンクと設置コードは無効になる）。
func (c *Client) ReauthorizeEnvironment(ctx context.Context, path string) (*tm.Environment, error) {
	cur, err := c.GetEnvironment(ctx, path)
	if err != nil {
		return nil, err
	}
	var v *tm.Environment
	err = c.do(ctx, "environments.reauthorize", func() (e error) {
		v, e = c.API.Accounts.Containers.Environments.Reauthorize(path, cur).Context(ctx).Do()
		return e
	})
	return v, err
}
