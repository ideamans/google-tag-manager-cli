package auditlog

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"time"
)

// Transport は API への HTTP リクエストを1件ずつ記録する RoundTripper。
// 再試行の内側に置くので、再試行した分も1回ずつ残る。認証ヘッダーは記録しない。
type Transport struct {
	Base http.RoundTripper
	Log  *Logger
}

// pathRe は API のパスから対象のアカウント・コンテナ・ワークスペースを取り出す。
var pathRe = regexp.MustCompile(`accounts/(\d+)(?:/containers/(\d+))?(?:/workspaces/(\d+))?`)

// Scope はパス（accounts/1/containers/2/workspaces/3/...）から対象を取り出す。
func Scope(path string) (account, container, workspace string) {
	m := pathRe.FindStringSubmatch(path)
	if m == nil {
		return "", "", ""
	}
	return m[1], m[2], m[3]
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := map[string]any{
		"event":  "api",
		"method": req.Method,
		"host":   req.URL.Host,
		"path":   req.URL.Path,
		"write":  req.Method != http.MethodGet,
	}
	if q := req.URL.Query(); len(q) > 0 {
		q.Del("alt")
		q.Del("prettyPrint")
		if len(q) > 0 {
			rec["query"] = q
		}
	}
	setScope(rec, req.URL.Path)

	if body := requestBody(req); len(body) > 0 {
		rec["request"] = Abbreviate(body)
	}

	start := time.Now()
	resp, err := t.Base.RoundTrip(req)
	rec["durationMs"] = time.Since(start).Milliseconds()
	if err != nil {
		rec["error"] = err.Error()
		_ = t.Log.Write(rec)
		return resp, err
	}
	rec["status"] = resp.StatusCode

	// 応答の path（lookup や create の結果の実体）と、エラーの文言だけ拾う。
	if resp.Body != nil {
		b, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(b))
		if rerr == nil {
			var r struct {
				Path  string `json:"path"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(b, &r) == nil {
				if r.Path != "" {
					rec["resultPath"] = r.Path
					if rec["account"] == nil {
						setScope(rec, r.Path)
					}
				}
				if r.Error != nil {
					rec["error"] = abbreviate(r.Error.Message)
				}
			}
		}
	}
	_ = t.Log.Write(rec)
	return resp, nil
}

func setScope(rec map[string]any, path string) {
	a, c, w := Scope(path)
	if a != "" {
		rec["account"] = a
	}
	if c != "" {
		rec["container"] = c
	}
	if w != "" {
		rec["workspace"] = w
	}
}

// requestBody は本文を読む。読んだ分は元に戻す（送信と再試行に使うため）。
func requestBody(req *http.Request) []byte {
	if req.Body == nil || req.Body == http.NoBody {
		return nil
	}
	if req.GetBody != nil {
		rc, err := req.GetBody()
		if err != nil {
			return nil
		}
		defer rc.Close()
		b, _ := io.ReadAll(rc)
		return b
	}
	b, _ := io.ReadAll(req.Body)
	req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(b))
	return b
}
