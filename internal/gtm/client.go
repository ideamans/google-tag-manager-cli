package gtm

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	tm "google.golang.org/api/tagmanager/v2"

	"github.com/ideamans/google-tag-manager-cli/internal/auditlog"
	"github.com/ideamans/google-tag-manager-cli/internal/config"
)

// Client は認証済みの Tag Manager API サービスと、操作対象（アカウント/コンテナ/
// ワークスペース）の解決状態を保持する。
type Client struct {
	API *tm.Service

	accountID   string
	container   string // 数字のID、または未解決の GTM-XXXX
	workspaceID string

	// 解決結果のキャッシュ。
	resolvedContainer *tm.Container
}

// NewClient は実効設定から認証済みクライアントを生成する。
//
// API への全リクエストは log に記録される（再試行の1回ごと）。証跡の無い操作を作らないため、
// log が無ければクライアントを作らない。
func NewClient(ctx context.Context, r *config.Resolved, log *auditlog.Logger) (*Client, error) {
	if log == nil {
		return nil, fmt.Errorf("the audit log is not open; refusing to call the API without it")
	}
	if err := r.RequireForAPI(); err != nil {
		return nil, err
	}
	ts := TokenSource(ctx, r.ClientID, r.ClientSecret, r.RefreshToken)
	hc := &http.Client{Transport: &retryTransport{
		base: &auditlog.Transport{
			Log:  log,
			Base: &oauth2.Transport{Source: ts, Base: http.DefaultTransport},
		},
	}}
	svc, err := tm.NewService(ctx, option.WithHTTPClient(hc))
	if err != nil {
		return nil, fmt.Errorf("failed to create the Tag Manager API client: %w", err)
	}
	return &Client{
		API:         svc,
		accountID:   r.AccountID,
		container:   r.Container,
		workspaceID: r.WorkspaceID,
	}, nil
}

// AccountPath は "accounts/<id>" を返す。引数が空なら既定のアカウントを使う。
func (c *Client) AccountPath(ctx context.Context, id string) (string, error) {
	id = config.NormalizeID(id)
	if id == "" {
		if c.container != "" {
			ct, err := c.Container(ctx)
			if err != nil {
				return "", err
			}
			return "accounts/" + ct.AccountId, nil
		}
		id = c.accountID
	}
	if id == "" {
		return "", fmt.Errorf("account ID is not set (specify --account, GTM_ACCOUNT_ID, or run `container use <id>`)")
	}
	return "accounts/" + id, nil
}

// LookupPublicID は GTM-XXXX からコンテナを引く（アカウントIDを知らなくてよい）。
func (c *Client) LookupPublicID(ctx context.Context, publicID string) (*tm.Container, error) {
	var ct *tm.Container
	err := c.do(ctx, "containers.lookup", func() (err error) {
		ct, err = c.API.Accounts.Containers.Lookup().TagId(strings.ToUpper(publicID)).Context(ctx).Do()
		return err
	})
	return ct, err
}

// Container は操作対象のコンテナを解決して返す。
// GTM-XXXX なら containers.lookup、数字なら containers.get（アカウントIDが必要）。
func (c *Client) Container(ctx context.Context) (*tm.Container, error) {
	if c.resolvedContainer != nil {
		return c.resolvedContainer, nil
	}
	if c.container == "" {
		return nil, fmt.Errorf("container is not set (specify --container with a GTM-XXXX public ID, a container ID, or accounts/<a>/containers/<c>; or run `container use`)")
	}
	var ct *tm.Container
	var err error
	if config.IsPublicID(c.container) {
		ct, err = c.LookupPublicID(ctx, c.container)
	} else {
		if c.accountID == "" {
			return nil, fmt.Errorf("container %s was given as a numeric ID, which also needs --account. Pass the GTM-XXXX public ID or accounts/<a>/containers/<c> instead", c.container)
		}
		path := fmt.Sprintf("accounts/%s/containers/%s", c.accountID, c.container)
		err = c.do(ctx, "containers.get", func() (e error) {
			ct, e = c.API.Accounts.Containers.Get(path).Context(ctx).Do()
			return e
		})
	}
	if err != nil {
		return nil, err
	}
	c.resolvedContainer = ct
	return ct, nil
}

// ContainerPath は "accounts/<a>/containers/<c>" を返す。
func (c *Client) ContainerPath(ctx context.Context) (string, error) {
	ct, err := c.Container(ctx)
	if err != nil {
		return "", err
	}
	return ct.Path, nil
}

// WorkspacePath は操作対象のワークスペースのパスを返す。
//
// 指定が無ければ、コンテナにワークスペースが1つしか無いときに限りそれを使う。
// 複数あるときは推測せずにエラーにする（別人の作業中のワークスペースを編集しないため）。
func (c *Client) WorkspacePath(ctx context.Context) (string, error) {
	cp, err := c.ContainerPath(ctx)
	if err != nil {
		return "", err
	}
	if c.workspaceID != "" {
		return cp + "/workspaces/" + c.workspaceID, nil
	}
	wss, err := c.ListWorkspaces(ctx, cp)
	if err != nil {
		return "", err
	}
	if len(wss) == 1 {
		c.workspaceID = wss[0].WorkspaceId
		return wss[0].Path, nil
	}
	var names []string
	for _, w := range wss {
		names = append(names, fmt.Sprintf("%s (%s)", w.WorkspaceId, w.Name))
	}
	return "", fmt.Errorf("the container has %d workspaces; choose one with --workspace or `workspace use <id>`: %s", len(wss), strings.Join(names, ", "))
}

// ListWorkspaces はコンテナ配下のワークスペースを返す。
func (c *Client) ListWorkspaces(ctx context.Context, containerPath string) ([]*tm.Workspace, error) {
	var out []*tm.Workspace
	err := c.do(ctx, "workspaces.list", func() error {
		out = nil
		return c.API.Accounts.Containers.Workspaces.List(containerPath).Pages(ctx, func(r *tm.ListWorkspacesResponse) error {
			out = append(out, r.Workspace...)
			return nil
		})
	})
	return out, err
}

// do は API 呼び出しを実行し、エラーを読みやすく整える。
// 429/5xx の再試行は HTTP の層（retryTransport）で行う。
func (c *Client) do(_ context.Context, label string, fn func() error) error {
	if err := fn(); err != nil {
		return apiError(label, err)
	}
	return nil
}

// retryTransport は 429（レート制限）と 5xx を指数バックオフで再試行する。
//
// Tag Manager API は利用者あたりのレート制限が低く、ワークスペースを丸ごと
// 読むだけで制限に触れることがある。Retry-After があればそれに従う。
type retryTransport struct {
	base http.RoundTripper
}

const maxRetries = 6

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 本文を作り直せないリクエストは再試行しない（空の本文を送ってしまうため）。
	replayable := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
	wait := 2 * time.Second
	for attempt := 0; ; attempt++ {
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			req.Body = body
		}
		resp, err := t.base.RoundTrip(req)
		if err != nil || !replayable || attempt >= maxRetries || !retryable(req.Method, resp.StatusCode) {
			return resp, err
		}
		d := wait
		if ra, convErr := strconv.Atoi(resp.Header.Get("Retry-After")); convErr == nil && ra > 0 {
			d = time.Duration(ra) * time.Second
		}
		resp.Body.Close()
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(d):
		}
		if wait < 32*time.Second {
			wait *= 2
		}
	}
}

// retryable は再送してよい応答かを返す。
//
// 429 はリクエストが処理されていないので常に再送する。5xx は「処理されたかどうか分からない」ので、
// 何度送っても結果が同じメソッド（GET・PUT・DELETE）に限る。POST は作成・バージョン作成・公開・
// 一括更新などで、再送すると二重に作られたり公開されたりしうるので再送しない。
func retryable(method string, code int) bool {
	if code == http.StatusTooManyRequests {
		return true
	}
	if code < 500 {
		return false
	}
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}
