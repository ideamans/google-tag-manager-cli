package gtm

import (
	"context"
	"fmt"
	"slices"
	"time"

	tm "google.golang.org/api/tagmanager/v2"

	"github.com/ideamans/google-tag-manager-cli/internal/config"
)

// Export は GTM の UI の「コンテナをエクスポート」と同じ形の JSON。
// そのまま UI の「コンテナをインポート」に渡せる。
type Export struct {
	ExportFormatVersion int                  `json:"exportFormatVersion"`
	ExportTime          string               `json:"exportTime"`
	ContainerVersion    *tm.ContainerVersion `json:"containerVersion"`
}

// NewExport は ContainerVersion をエクスポート形式に包む。
func NewExport(v *tm.ContainerVersion) *Export {
	return &Export{
		ExportFormatVersion: 2,
		ExportTime:          time.Now().Format("2006-01-02 15:04:05"),
		ContainerVersion:    v,
	}
}

// Snapshot はコンテナの中身を ContainerVersion の形で返す。
//
//	from = "live"      … 公開中のバージョン（サイトで実際に動いているもの）
//	from = "latest"    … 最新のバージョン（未公開を含む）
//	from = "workspace" … 操作対象ワークスペースの現状（未保存の変更を含む）
//	from = <数字>       … 指定のバージョン
func (c *Client) Snapshot(ctx context.Context, from string) (*tm.ContainerVersion, error) {
	cp, err := c.ContainerPath(ctx)
	if err != nil {
		return nil, err
	}
	switch from {
	case "", "live":
		var v *tm.ContainerVersion
		err := c.do(ctx, "versions.live", func() (e error) {
			v, e = c.API.Accounts.Containers.Versions.Live(cp).Context(ctx).Do()
			return e
		})
		if err != nil {
			return nil, fmt.Errorf("%w\n  (a container that has never been published has no live version; try --from workspace)", err)
		}
		return v, nil
	case "latest":
		var h *tm.ContainerVersionHeader
		err := c.do(ctx, "version_headers.latest", func() (e error) {
			h, e = c.API.Accounts.Containers.VersionHeaders.Latest(cp).Context(ctx).Do()
			return e
		})
		if err != nil {
			return nil, err
		}
		return c.GetVersion(ctx, h.Path)
	case "workspace":
		return c.workspaceSnapshot(ctx)
	default:
		return c.GetVersion(ctx, cp+"/versions/"+config.NormalizeID(from))
	}
}

// GetVersion はバージョンを1つ取得する。
func (c *Client) GetVersion(ctx context.Context, path string) (*tm.ContainerVersion, error) {
	var v *tm.ContainerVersion
	err := c.do(ctx, "versions.get", func() (e error) {
		v, e = c.API.Accounts.Containers.Versions.Get(path).Context(ctx).Do()
		return e
	})
	return v, err
}

// workspaceSnapshot はワークスペースの各エンティティを集めて ContainerVersion に組み立てる。
// API にワークスペース全体を一度に返す呼び出しは無いため、種類ごとに一覧を取る。
func (c *Client) workspaceSnapshot(ctx context.Context) (*tm.ContainerVersion, error) {
	ct, err := c.Container(ctx)
	if err != nil {
		return nil, err
	}
	ws, err := c.WorkspacePath(ctx)
	if err != nil {
		return nil, err
	}
	v := &tm.ContainerVersion{
		AccountId:   ct.AccountId,
		ContainerId: ct.ContainerId,
		Container:   ct,
		Name:        "workspace " + c.workspaceID,
		Path:        ws,
	}
	if v.Tag, err = c.ListTags(ctx, ws); err != nil {
		return nil, err
	}
	if v.Trigger, err = c.ListTriggers(ctx, ws); err != nil {
		return nil, err
	}
	if v.Variable, err = c.ListVariables(ctx, ws); err != nil {
		return nil, err
	}
	if v.Folder, err = c.ListFolders(ctx, ws); err != nil {
		return nil, err
	}
	if v.BuiltInVariable, err = c.ListBuiltInVariables(ctx, ws); err != nil {
		return nil, err
	}
	if v.CustomTemplate, err = c.ListTemplates(ctx, ws); err != nil {
		return nil, err
	}
	// クライアント・変換はサーバーコンテナ、ゾーン・Google タグ設定はウェブコンテナにしか無い。
	if slices.Contains(ct.UsageContext, "server") {
		if v.Client, err = c.ListClients(ctx, ws); err != nil {
			return nil, err
		}
		if v.Transformation, err = c.ListTransformations(ctx, ws); err != nil {
			return nil, err
		}
	}
	if slices.Contains(ct.UsageContext, "web") {
		// ゾーンは GTM 360 の機能。無料版では 403 になるので無いものとして扱う。
		if v.Zone, err = c.ListZones(ctx, ws); err != nil && StatusCode(err) != 403 {
			return nil, err
		}
		if v.GtagConfig, err = c.ListGtagConfigs(ctx, ws); err != nil {
			return nil, err
		}
	}
	return v, nil
}
