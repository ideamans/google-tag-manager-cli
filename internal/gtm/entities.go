package gtm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	tm "google.golang.org/api/tagmanager/v2"
)

// Kind はワークスペース配下のエンティティ1種類（タグ・トリガー・変数・フォルダ・
// テンプレート・クライアント・変換・ゾーン・Google タグ設定）に対する操作をまとめたもの。
// CLI 側はこれを使って同じ形のサブコマンド群（list/get/create/update/delete/revert）を組み立てる。
//
// 作成・更新の本文は API と同じ JSON（GTM の UI がエクスポートする containerVersion の
// 要素と同じ形）で受ける。
type Kind struct {
	Name      string // CLI のコマンド名（tag, gtag-config）
	Plural    string // 複数形（tags）
	Segment   string // パスの区切り（tags, gtag_config）
	IDField   string // JSON 上の ID フィールド（tagId）
	ExportKey string // containerVersion 上のキー（tag, customTemplate, gtagConfig）
	Note      string // ヘルプに添える一言（どの種類のコンテナにあるか等）

	HasName   bool // name / notes を持つ
	HasFolder bool // parentFolderId を持つ

	List   func(ctx context.Context, c *Client, ws string) ([]any, error)
	Get    func(ctx context.Context, c *Client, path string) (any, error)
	Create func(ctx context.Context, c *Client, ws string, body []byte) (any, error)
	Update func(ctx context.Context, c *Client, path string, body []byte, fingerprint string) (any, error)
	Delete func(ctx context.Context, c *Client, path string) error
	Revert func(ctx context.Context, c *Client, path string) (any, error) // nil: API に revert が無い
}

// Path はワークスペースのパスとIDからエンティティのパスを作る。
func (k *Kind) Path(ws, id string) string { return ws + "/" + k.Segment + "/" + id }

// Kinds は CLI が扱うエンティティの一覧。
var Kinds = []*Kind{
	TagKind, TriggerKind, VariableKind, FolderKind, TemplateKind,
	ClientKind, TransformationKind, ZoneKind, GtagConfigKind,
}

// decode は JSON を API の型に読み込む。未知のフィールドはエラーにする
// （綴り違いのフィールドが黙って捨てられ、意図しない設定で保存されるのを防ぐ）。
func decode(body []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

// stripIdentity は作成用の本文から、別コンテナ由来の ID・パス・fingerprint を取り除く。
func stripIdentity(body []byte, idField string) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("invalid JSON body: %w", err)
	}
	for _, k := range []string{"path", "accountId", "containerId", "workspaceId", "fingerprint", "tagManagerUrl", idField} {
		delete(m, k)
	}
	return json.Marshal(m)
}

func toAny[T any](items []T) []any {
	out := make([]any, len(items))
	for i, v := range items {
		out[i] = v
	}
	return out
}

// wsAPI はワークスペース配下のサービス。
type wsAPI = tm.AccountsContainersWorkspacesService

// kindOps は1種類分の API 呼び出し。makeKind が c.do（再試行・エラー整形）で包む。
type kindOps[T any] struct {
	list         func(ctx context.Context, c *Client, ws string) ([]*T, error)
	get          func(ctx context.Context, s *wsAPI, path string) (*T, error)
	create       func(ctx context.Context, s *wsAPI, ws string, v *T) (*T, error)
	update       func(ctx context.Context, s *wsAPI, path string, v *T, fp string) (*T, error)
	del          func(ctx context.Context, s *wsAPI, path string) error
	revert       func(ctx context.Context, s *wsAPI, path string) (any, error)
	beforeUpdate func(*T)
}

func makeKind[T any](k *Kind, o kindOps[T]) *Kind {
	op := func(name string) string { return k.Segment + "." + name }
	k.List = func(ctx context.Context, c *Client, ws string) ([]any, error) {
		v, err := o.list(ctx, c, ws)
		return toAny(v), err
	}
	k.Get = func(ctx context.Context, c *Client, path string) (any, error) {
		var v *T
		err := c.do(ctx, op("get"), func() (e error) {
			v, e = o.get(ctx, c.API.Accounts.Containers.Workspaces, path)
			return e
		})
		return v, err
	}
	k.Create = func(ctx context.Context, c *Client, ws string, body []byte) (any, error) {
		body, err := stripIdentity(body, k.IDField)
		if err != nil {
			return nil, err
		}
		var in T
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		var v *T
		err = c.do(ctx, op("create"), func() (e error) {
			v, e = o.create(ctx, c.API.Accounts.Containers.Workspaces, ws, &in)
			return e
		})
		return v, err
	}
	k.Update = func(ctx context.Context, c *Client, path string, body []byte, fp string) (any, error) {
		var in T
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		if o.beforeUpdate != nil {
			o.beforeUpdate(&in)
		}
		var v *T
		err := c.do(ctx, op("update"), func() (e error) {
			v, e = o.update(ctx, c.API.Accounts.Containers.Workspaces, path, &in, fp)
			return e
		})
		return v, err
	}
	k.Delete = func(ctx context.Context, c *Client, path string) error {
		return c.do(ctx, op("delete"), func() error {
			return o.del(ctx, c.API.Accounts.Containers.Workspaces, path)
		})
	}
	if o.revert != nil {
		k.Revert = func(ctx context.Context, c *Client, path string) (any, error) {
			var v any
			err := c.do(ctx, op("revert"), func() (e error) {
				v, e = o.revert(ctx, c.API.Accounts.Containers.Workspaces, path)
				return e
			})
			return v, err
		}
	}
	return k
}

// TagKind はタグ。
var TagKind = makeKind(&Kind{
	Name: "tag", Plural: "tags", Segment: "tags", IDField: "tagId", ExportKey: "tag",
	HasName: true, HasFolder: true,
}, kindOps[tm.Tag]{
	list: func(ctx context.Context, c *Client, ws string) ([]*tm.Tag, error) { return c.ListTags(ctx, ws) },
	get: func(ctx context.Context, s *wsAPI, p string) (*tm.Tag, error) {
		return s.Tags.Get(p).Context(ctx).Do()
	},
	create: func(ctx context.Context, s *wsAPI, ws string, v *tm.Tag) (*tm.Tag, error) {
		return s.Tags.Create(ws, v).Context(ctx).Do()
	},
	update: func(ctx context.Context, s *wsAPI, p string, v *tm.Tag, fp string) (*tm.Tag, error) {
		return s.Tags.Update(p, v).Fingerprint(fp).Context(ctx).Do()
	},
	del: func(ctx context.Context, s *wsAPI, p string) error { return s.Tags.Delete(p).Context(ctx).Do() },
	revert: func(ctx context.Context, s *wsAPI, p string) (any, error) {
		return s.Tags.Revert(p).Context(ctx).Do()
	},
	// update は全置換。false の paused を省略すると送られないので明示する。
	beforeUpdate: func(t *tm.Tag) { t.ForceSendFields = append(t.ForceSendFields, "Paused") },
})

// TriggerKind はトリガー。
var TriggerKind = makeKind(&Kind{
	Name: "trigger", Plural: "triggers", Segment: "triggers", IDField: "triggerId", ExportKey: "trigger",
	HasName: true, HasFolder: true,
}, kindOps[tm.Trigger]{
	list: func(ctx context.Context, c *Client, ws string) ([]*tm.Trigger, error) { return c.ListTriggers(ctx, ws) },
	get: func(ctx context.Context, s *wsAPI, p string) (*tm.Trigger, error) {
		return s.Triggers.Get(p).Context(ctx).Do()
	},
	create: func(ctx context.Context, s *wsAPI, ws string, v *tm.Trigger) (*tm.Trigger, error) {
		return s.Triggers.Create(ws, v).Context(ctx).Do()
	},
	update: func(ctx context.Context, s *wsAPI, p string, v *tm.Trigger, fp string) (*tm.Trigger, error) {
		return s.Triggers.Update(p, v).Fingerprint(fp).Context(ctx).Do()
	},
	del: func(ctx context.Context, s *wsAPI, p string) error { return s.Triggers.Delete(p).Context(ctx).Do() },
	revert: func(ctx context.Context, s *wsAPI, p string) (any, error) {
		return s.Triggers.Revert(p).Context(ctx).Do()
	},
})

// VariableKind はユーザー定義変数。
var VariableKind = makeKind(&Kind{
	Name: "variable", Plural: "variables", Segment: "variables", IDField: "variableId", ExportKey: "variable",
	HasName: true, HasFolder: true,
}, kindOps[tm.Variable]{
	list: func(ctx context.Context, c *Client, ws string) ([]*tm.Variable, error) {
		return c.ListVariables(ctx, ws)
	},
	get: func(ctx context.Context, s *wsAPI, p string) (*tm.Variable, error) {
		return s.Variables.Get(p).Context(ctx).Do()
	},
	create: func(ctx context.Context, s *wsAPI, ws string, v *tm.Variable) (*tm.Variable, error) {
		return s.Variables.Create(ws, v).Context(ctx).Do()
	},
	update: func(ctx context.Context, s *wsAPI, p string, v *tm.Variable, fp string) (*tm.Variable, error) {
		return s.Variables.Update(p, v).Fingerprint(fp).Context(ctx).Do()
	},
	del: func(ctx context.Context, s *wsAPI, p string) error { return s.Variables.Delete(p).Context(ctx).Do() },
	revert: func(ctx context.Context, s *wsAPI, p string) (any, error) {
		return s.Variables.Revert(p).Context(ctx).Do()
	},
})

// FolderKind はフォルダ。
var FolderKind = makeKind(&Kind{
	Name: "folder", Plural: "folders", Segment: "folders", IDField: "folderId", ExportKey: "folder",
	HasName: true,
}, kindOps[tm.Folder]{
	list: func(ctx context.Context, c *Client, ws string) ([]*tm.Folder, error) { return c.ListFolders(ctx, ws) },
	get: func(ctx context.Context, s *wsAPI, p string) (*tm.Folder, error) {
		return s.Folders.Get(p).Context(ctx).Do()
	},
	create: func(ctx context.Context, s *wsAPI, ws string, v *tm.Folder) (*tm.Folder, error) {
		return s.Folders.Create(ws, v).Context(ctx).Do()
	},
	update: func(ctx context.Context, s *wsAPI, p string, v *tm.Folder, fp string) (*tm.Folder, error) {
		return s.Folders.Update(p, v).Fingerprint(fp).Context(ctx).Do()
	},
	del: func(ctx context.Context, s *wsAPI, p string) error { return s.Folders.Delete(p).Context(ctx).Do() },
	revert: func(ctx context.Context, s *wsAPI, p string) (any, error) {
		return s.Folders.Revert(p).Context(ctx).Do()
	},
})

// TemplateKind はカスタムテンプレート（タグ・変数の type が cvt_ で始まるものの実体）。
var TemplateKind = makeKind(&Kind{
	Name: "template", Plural: "templates", Segment: "templates", IDField: "templateId", ExportKey: "customTemplate",
	HasName: true,
}, kindOps[tm.CustomTemplate]{
	list: func(ctx context.Context, c *Client, ws string) ([]*tm.CustomTemplate, error) {
		return c.ListTemplates(ctx, ws)
	},
	get: func(ctx context.Context, s *wsAPI, p string) (*tm.CustomTemplate, error) {
		return s.Templates.Get(p).Context(ctx).Do()
	},
	create: func(ctx context.Context, s *wsAPI, ws string, v *tm.CustomTemplate) (*tm.CustomTemplate, error) {
		return s.Templates.Create(ws, v).Context(ctx).Do()
	},
	update: func(ctx context.Context, s *wsAPI, p string, v *tm.CustomTemplate, fp string) (*tm.CustomTemplate, error) {
		return s.Templates.Update(p, v).Fingerprint(fp).Context(ctx).Do()
	},
	del: func(ctx context.Context, s *wsAPI, p string) error { return s.Templates.Delete(p).Context(ctx).Do() },
	revert: func(ctx context.Context, s *wsAPI, p string) (any, error) {
		return s.Templates.Revert(p).Context(ctx).Do()
	},
})

// ClientKind はクライアント（サーバーコンテナでリクエストを受け取る部品）。
var ClientKind = makeKind(&Kind{
	Name: "client", Plural: "clients", Segment: "clients", IDField: "clientId", ExportKey: "client",
	Note: "Server containers only.", HasName: true, HasFolder: true,
}, kindOps[tm.Client]{
	list: func(ctx context.Context, c *Client, ws string) ([]*tm.Client, error) { return c.ListClients(ctx, ws) },
	get: func(ctx context.Context, s *wsAPI, p string) (*tm.Client, error) {
		return s.Clients.Get(p).Context(ctx).Do()
	},
	create: func(ctx context.Context, s *wsAPI, ws string, v *tm.Client) (*tm.Client, error) {
		return s.Clients.Create(ws, v).Context(ctx).Do()
	},
	update: func(ctx context.Context, s *wsAPI, p string, v *tm.Client, fp string) (*tm.Client, error) {
		return s.Clients.Update(p, v).Fingerprint(fp).Context(ctx).Do()
	},
	del: func(ctx context.Context, s *wsAPI, p string) error { return s.Clients.Delete(p).Context(ctx).Do() },
	revert: func(ctx context.Context, s *wsAPI, p string) (any, error) {
		return s.Clients.Revert(p).Context(ctx).Do()
	},
})

// TransformationKind は変換（サーバーコンテナでイベントデータを書き換える部品）。
var TransformationKind = makeKind(&Kind{
	Name: "transformation", Plural: "transformations", Segment: "transformations", IDField: "transformationId",
	ExportKey: "transformation", Note: "Server containers only.", HasName: true, HasFolder: true,
}, kindOps[tm.Transformation]{
	list: func(ctx context.Context, c *Client, ws string) ([]*tm.Transformation, error) {
		return c.ListTransformations(ctx, ws)
	},
	get: func(ctx context.Context, s *wsAPI, p string) (*tm.Transformation, error) {
		return s.Transformations.Get(p).Context(ctx).Do()
	},
	create: func(ctx context.Context, s *wsAPI, ws string, v *tm.Transformation) (*tm.Transformation, error) {
		return s.Transformations.Create(ws, v).Context(ctx).Do()
	},
	update: func(ctx context.Context, s *wsAPI, p string, v *tm.Transformation, fp string) (*tm.Transformation, error) {
		return s.Transformations.Update(p, v).Fingerprint(fp).Context(ctx).Do()
	},
	del: func(ctx context.Context, s *wsAPI, p string) error {
		return s.Transformations.Delete(p).Context(ctx).Do()
	},
	revert: func(ctx context.Context, s *wsAPI, p string) (any, error) {
		return s.Transformations.Revert(p).Context(ctx).Do()
	},
})

// ZoneKind はゾーン（GTM 360 のウェブコンテナで、子コンテナの範囲と許可する種類を決める）。
var ZoneKind = makeKind(&Kind{
	Name: "zone", Plural: "zones", Segment: "zones", IDField: "zoneId", ExportKey: "zone",
	Note: "Web containers on Tag Manager 360 only; free accounts get 403.", HasName: true,
}, kindOps[tm.Zone]{
	list: func(ctx context.Context, c *Client, ws string) ([]*tm.Zone, error) { return c.ListZones(ctx, ws) },
	get: func(ctx context.Context, s *wsAPI, p string) (*tm.Zone, error) {
		return s.Zones.Get(p).Context(ctx).Do()
	},
	create: func(ctx context.Context, s *wsAPI, ws string, v *tm.Zone) (*tm.Zone, error) {
		return s.Zones.Create(ws, v).Context(ctx).Do()
	},
	update: func(ctx context.Context, s *wsAPI, p string, v *tm.Zone, fp string) (*tm.Zone, error) {
		return s.Zones.Update(p, v).Fingerprint(fp).Context(ctx).Do()
	},
	del: func(ctx context.Context, s *wsAPI, p string) error { return s.Zones.Delete(p).Context(ctx).Do() },
	revert: func(ctx context.Context, s *wsAPI, p string) (any, error) {
		return s.Zones.Revert(p).Context(ctx).Do()
	},
})

// GtagConfigKind は Google タグの設定（ウェブコンテナの Google タグに付く共通設定）。
// API に revert は無い。
var GtagConfigKind = makeKind(&Kind{
	Name: "gtag-config", Plural: "Google tag configs", Segment: "gtag_config", IDField: "gtagConfigId",
	ExportKey: "gtagConfig", Note: "Web containers. Has no name/notes and no revert in the API.",
}, kindOps[tm.GtagConfig]{
	list: func(ctx context.Context, c *Client, ws string) ([]*tm.GtagConfig, error) {
		return c.ListGtagConfigs(ctx, ws)
	},
	get: func(ctx context.Context, s *wsAPI, p string) (*tm.GtagConfig, error) {
		return s.GtagConfig.Get(p).Context(ctx).Do()
	},
	create: func(ctx context.Context, s *wsAPI, ws string, v *tm.GtagConfig) (*tm.GtagConfig, error) {
		return s.GtagConfig.Create(ws, v).Context(ctx).Do()
	},
	update: func(ctx context.Context, s *wsAPI, p string, v *tm.GtagConfig, fp string) (*tm.GtagConfig, error) {
		return s.GtagConfig.Update(p, v).Fingerprint(fp).Context(ctx).Do()
	},
	del: func(ctx context.Context, s *wsAPI, p string) error { return s.GtagConfig.Delete(p).Context(ctx).Do() },
})

// ---- 一覧（export と audit からも使う型付きの版） ----

// ListTags はワークスペースのタグをすべて返す。
func (c *Client) ListTags(ctx context.Context, ws string) ([]*tm.Tag, error) {
	var out []*tm.Tag
	err := c.do(ctx, "tags.list", func() error {
		out = nil
		return c.API.Accounts.Containers.Workspaces.Tags.List(ws).Pages(ctx, func(r *tm.ListTagsResponse) error {
			out = append(out, r.Tag...)
			return nil
		})
	})
	return out, err
}

// ListTriggers はワークスペースのトリガーをすべて返す。
func (c *Client) ListTriggers(ctx context.Context, ws string) ([]*tm.Trigger, error) {
	var out []*tm.Trigger
	err := c.do(ctx, "triggers.list", func() error {
		out = nil
		return c.API.Accounts.Containers.Workspaces.Triggers.List(ws).Pages(ctx, func(r *tm.ListTriggersResponse) error {
			out = append(out, r.Trigger...)
			return nil
		})
	})
	return out, err
}

// ListVariables はワークスペースのユーザー定義変数をすべて返す。
func (c *Client) ListVariables(ctx context.Context, ws string) ([]*tm.Variable, error) {
	var out []*tm.Variable
	err := c.do(ctx, "variables.list", func() error {
		out = nil
		return c.API.Accounts.Containers.Workspaces.Variables.List(ws).Pages(ctx, func(r *tm.ListVariablesResponse) error {
			out = append(out, r.Variable...)
			return nil
		})
	})
	return out, err
}

// ListFolders はワークスペースのフォルダをすべて返す。
func (c *Client) ListFolders(ctx context.Context, ws string) ([]*tm.Folder, error) {
	var out []*tm.Folder
	err := c.do(ctx, "folders.list", func() error {
		out = nil
		return c.API.Accounts.Containers.Workspaces.Folders.List(ws).Pages(ctx, func(r *tm.ListFoldersResponse) error {
			out = append(out, r.Folder...)
			return nil
		})
	})
	return out, err
}

// ListTemplates はワークスペースのカスタムテンプレートをすべて返す。
func (c *Client) ListTemplates(ctx context.Context, ws string) ([]*tm.CustomTemplate, error) {
	var out []*tm.CustomTemplate
	err := c.do(ctx, "templates.list", func() error {
		out = nil
		return c.API.Accounts.Containers.Workspaces.Templates.List(ws).Pages(ctx, func(r *tm.ListTemplatesResponse) error {
			out = append(out, r.Template...)
			return nil
		})
	})
	return out, err
}

// ListBuiltInVariables はワークスペースで有効な組み込み変数を返す。
func (c *Client) ListBuiltInVariables(ctx context.Context, ws string) ([]*tm.BuiltInVariable, error) {
	var out []*tm.BuiltInVariable
	err := c.do(ctx, "built_in_variables.list", func() error {
		out = nil
		return c.API.Accounts.Containers.Workspaces.BuiltInVariables.List(ws).Pages(ctx, func(r *tm.ListEnabledBuiltInVariablesResponse) error {
			out = append(out, r.BuiltInVariable...)
			return nil
		})
	})
	return out, err
}

// ListClients はワークスペースのクライアント（サーバーコンテナ）を返す。
func (c *Client) ListClients(ctx context.Context, ws string) ([]*tm.Client, error) {
	var out []*tm.Client
	err := c.do(ctx, "clients.list", func() error {
		out = nil
		return c.API.Accounts.Containers.Workspaces.Clients.List(ws).Pages(ctx, func(r *tm.ListClientsResponse) error {
			out = append(out, r.Client...)
			return nil
		})
	})
	return out, err
}

// ListTransformations はワークスペースの変換（サーバーコンテナ）を返す。
func (c *Client) ListTransformations(ctx context.Context, ws string) ([]*tm.Transformation, error) {
	var out []*tm.Transformation
	err := c.do(ctx, "transformations.list", func() error {
		out = nil
		return c.API.Accounts.Containers.Workspaces.Transformations.List(ws).Pages(ctx, func(r *tm.ListTransformationsResponse) error {
			out = append(out, r.Transformation...)
			return nil
		})
	})
	return out, err
}

// ListZones はワークスペースのゾーンを返す。
func (c *Client) ListZones(ctx context.Context, ws string) ([]*tm.Zone, error) {
	var out []*tm.Zone
	err := c.do(ctx, "zones.list", func() error {
		out = nil
		return c.API.Accounts.Containers.Workspaces.Zones.List(ws).Pages(ctx, func(r *tm.ListZonesResponse) error {
			out = append(out, r.Zone...)
			return nil
		})
	})
	return out, err
}

// ListGtagConfigs はワークスペースの Google タグ設定を返す。
func (c *Client) ListGtagConfigs(ctx context.Context, ws string) ([]*tm.GtagConfig, error) {
	var out []*tm.GtagConfig
	err := c.do(ctx, "gtag_config.list", func() error {
		out = nil
		return c.API.Accounts.Containers.Workspaces.GtagConfig.List(ws).Pages(ctx, func(r *tm.ListGtagConfigResponse) error {
			out = append(out, r.GtagConfig...)
			return nil
		})
	})
	return out, err
}

// SetBuiltInVariables は組み込み変数を有効化（enable=true）または無効化する。
func (c *Client) SetBuiltInVariables(ctx context.Context, ws string, types []string, enable bool) (any, error) {
	if enable {
		var v *tm.CreateBuiltInVariableResponse
		err := c.do(ctx, "built_in_variables.create", func() (e error) {
			v, e = c.API.Accounts.Containers.Workspaces.BuiltInVariables.Create(ws).Type(types...).Context(ctx).Do()
			return e
		})
		return v, err
	}
	err := c.do(ctx, "built_in_variables.delete", func() error {
		return c.API.Accounts.Containers.Workspaces.BuiltInVariables.Delete(ws + "/built_in_variables").Type(types...).Context(ctx).Do()
	})
	return map[string]any{"disabled": types}, err
}

// RevertBuiltInVariable は組み込み変数1つのワークスペースでの変更を取り消す。
// API の URL は {path}/built_in_variables:revert なので、path はワークスペースそのもの。
func (c *Client) RevertBuiltInVariable(ctx context.Context, ws, typ string) (*tm.RevertBuiltInVariableResponse, error) {
	var v *tm.RevertBuiltInVariableResponse
	err := c.do(ctx, "built_in_variables.revert", func() (e error) {
		v, e = c.API.Accounts.Containers.Workspaces.BuiltInVariables.Revert(ws).Type(typ).Context(ctx).Do()
		return e
	})
	return v, err
}

// MoveToFolder はタグ・トリガー・変数をフォルダへ移す。
func (c *Client) MoveToFolder(ctx context.Context, folderPath string, tags, triggers, variables []string) error {
	return c.do(ctx, "folders.move_entities_to_folder", func() error {
		call := c.API.Accounts.Containers.Workspaces.Folders.MoveEntitiesToFolder(folderPath, &tm.Folder{})
		if len(tags) > 0 {
			call = call.TagId(tags...)
		}
		if len(triggers) > 0 {
			call = call.TriggerId(triggers...)
		}
		if len(variables) > 0 {
			call = call.VariableId(variables...)
		}
		return call.Context(ctx).Do()
	})
}

// FolderEntities はフォルダの中のタグ・トリガー・変数を返す。
func (c *Client) FolderEntities(ctx context.Context, folderPath string) (*tm.FolderEntities, error) {
	out := &tm.FolderEntities{}
	err := c.do(ctx, "folders.entities", func() error {
		out = &tm.FolderEntities{}
		return c.API.Accounts.Containers.Workspaces.Folders.Entities(folderPath).Pages(ctx, func(r *tm.FolderEntities) error {
			out.Tag = append(out.Tag, r.Tag...)
			out.Trigger = append(out.Trigger, r.Trigger...)
			out.Variable = append(out.Variable, r.Variable...)
			return nil
		})
	})
	out.NextPageToken = ""
	return out, err
}

// ImportGalleryTemplate はコミュニティテンプレートギャラリーのテンプレートを取り込む。
// sha が空なら最新版。
func (c *Client) ImportGalleryTemplate(ctx context.Context, ws, owner, repo, sha string) (*tm.CustomTemplate, error) {
	var v *tm.CustomTemplate
	err := c.do(ctx, "templates.import_from_gallery", func() (e error) {
		call := c.API.Accounts.Containers.Workspaces.Templates.ImportFromGallery(ws).
			GalleryOwner(owner).GalleryRepository(repo).AcknowledgePermissions(true)
		if sha != "" {
			call = call.GallerySha(sha)
		}
		v, e = call.Context(ctx).Do()
		return e
	})
	return v, err
}
