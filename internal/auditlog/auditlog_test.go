package auditlog

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenWritesDailyFileAndPrunes(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)
	old := filepath.Join(dir, "audit-2026-07-01.jsonl")    // 125 日前: 消える
	recent := filepath.Join(dir, "audit-2026-10-01.jsonl") // 33 日前: 残る
	other := filepath.Join(dir, "notes.txt")               // 形式外: 触らない
	for _, p := range []string{old, recent, other} {
		if err := os.WriteFile(p, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	l, err := open(dir, 90, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Write(map[string]any{"event": "api", "path": "/x"}); err != nil {
		t.Fatal(err)
	}
	_ = l.Close()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("a file past the retention period was kept")
	}
	for _, p := range []string{recent, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was removed", p)
		}
	}
	if filepath.Base(l.Path()) != "audit-2026-11-03.jsonl" {
		t.Errorf("path = %s", l.Path())
	}
	info, _ := os.Stat(l.Path())
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", info.Mode().Perm())
	}
	recs, err := Read(dir, now.Add(-time.Hour))
	if err != nil || len(recs) != 1 || recs[0]["invocation"] == "" || recs[0]["time"] == nil {
		t.Fatalf("read back: %v %v", err, recs)
	}
}

func TestDirResolution(t *testing.T) {
	t.Setenv("GTM_LOG_DIR", "")
	t.Setenv("GTM_HOME", "")
	t.Setenv("XDG_STATE_HOME", "/state")
	if d, _ := Dir(); d != "/state/google-tag-manager/logs" {
		t.Errorf("XDG: %s", d)
	}
	t.Setenv("GTM_HOME", "/home-override")
	if d, _ := Dir(); d != "/home-override/logs" {
		t.Errorf("GTM_HOME: %s", d)
	}
	t.Setenv("GTM_LOG_DIR", "/explicit")
	if d, _ := Dir(); d != "/explicit" {
		t.Errorf("explicit: %s", d)
	}
	t.Setenv("GTM_LOG_RETENTION_DAYS", "7")
	if RetentionDays() != 7 {
		t.Error("retention override")
	}
	t.Setenv("GTM_LOG_RETENTION_DAYS", "zero")
	if RetentionDays() != DefaultRetentionDays {
		t.Error("invalid retention must fall back to the default")
	}
}

func TestAbbreviate(t *testing.T) {
	long := strings.Repeat("あ", 300)
	items := make([]string, 60)
	for i := range items {
		items[i] = `"x"`
	}
	body := `{"name":"T","parameter":[{"key":"html","value":"` + long + `"}],"ids":[` + strings.Join(items, ",") + `]}`
	v := Abbreviate([]byte(body)).(map[string]any)
	val := v["parameter"].([]any)[0].(map[string]any)["value"].(string)
	if !strings.HasSuffix(val, "…(300 chars)") || len([]rune(val)) > maxString+20 {
		t.Errorf("long string not abbreviated: %d runes", len([]rune(val)))
	}
	ids := v["ids"].([]any)
	if len(ids) != maxArray+1 || ids[maxArray] != "…(10 more)" {
		t.Errorf("long array not abbreviated: %d", len(ids))
	}
	if v["name"] != "T" {
		t.Error("short values must be kept")
	}
	raw := Abbreviate([]byte(strings.Repeat("z", 2000))).(string)
	if !strings.HasSuffix(raw, "…(2000 bytes)") {
		t.Error("non-JSON body not abbreviated")
	}
}

func TestScope(t *testing.T) {
	a, c, w := Scope("/tagmanager/v2/accounts/1234567890/containers/123/workspaces/7/tags/4")
	if a != "1234567890" || c != "123" || w != "7" {
		t.Errorf("%s %s %s", a, c, w)
	}
	if a, _, _ := Scope("/tagmanager/v2/accounts"); a != "" {
		t.Error("accounts.list has no account")
	}
}

func TestTransportRecordsRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if r.Method == http.MethodPost && !strings.Contains(string(b), "GA4") {
			t.Error("the body did not reach the server intact")
		}
		if strings.Contains(r.URL.Path, "lookup") {
			_, _ = io.WriteString(w, `{"path":"accounts/42/containers/9","publicId":"GTM-X"}`)
			return
		}
		if r.Method == http.MethodDelete {
			w.WriteHeader(403)
			_, _ = io.WriteString(w, `{"error":{"message":"forbidden"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"path":"accounts/42/containers/9/workspaces/3/tags/5"}`)
	}))
	defer srv.Close()

	dir := t.TempDir()
	l, err := open(dir, 90, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	hc := &http.Client{Transport: &Transport{Log: l, Base: http.DefaultTransport}}

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/tagmanager/v2/accounts/42/containers/9/workspaces/3/tags",
		strings.NewReader(`{"name":"GA4","type":"gaawe"}`))
	req.Header.Set("Authorization", "Bearer SECRET-TOKEN")
	mustDo(t, hc, req)
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/tagmanager/v2/accounts/containers:lookup?tagId=GTM-X&alt=json", nil)
	mustDo(t, hc, req)
	req, _ = http.NewRequest(http.MethodDelete, srv.URL+"/tagmanager/v2/accounts/42/containers/9/workspaces/3/tags/5", nil)
	mustDo(t, hc, req)
	_ = l.Close()

	b, _ := os.ReadFile(l.Path())
	if strings.Contains(string(b), "SECRET-TOKEN") {
		t.Fatal("the Authorization header was logged")
	}
	recs, _ := Read(dir, time.Now().Add(-time.Minute))
	if len(recs) != 3 {
		t.Fatalf("records = %d", len(recs))
	}
	create, lookup, del := recs[0], recs[1], recs[2]
	if create["write"] != true || create["account"] != "42" || create["workspace"] != "3" ||
		create["request"].(map[string]any)["name"] != "GA4" || create["status"].(float64) != 200 {
		t.Errorf("create record: %v", create)
	}
	// lookup のパスにはアカウントが無い。応答の path から対象を補う。
	if lookup["write"] != false || lookup["account"] != "42" || lookup["container"] != "9" || lookup["resultPath"] == nil {
		t.Errorf("lookup record: %v", lookup)
	}
	if q, _ := lookup["query"].(map[string]any); q["tagId"] == nil || q["alt"] != nil {
		t.Errorf("lookup query: %v", lookup["query"])
	}
	if del["status"].(float64) != 403 || del["error"] != "forbidden" {
		t.Errorf("delete record: %v", del)
	}
}

func mustDo(t *testing.T, hc *http.Client, req *http.Request) {
	t.Helper()
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
}
