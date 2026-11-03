package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ideamans/google-tag-manager-cli/internal/config"
	"github.com/ideamans/google-tag-manager-cli/internal/gtm"
)

func TestRedactArgs(t *testing.T) {
	got := redactArgs([]string{"tag", "list", "--client-secret", "s3cr3t", "--client-secret=abc", "--client-id", "id"})
	want := []string{"tag", "list", "--client-secret", "***", "--client-secret=***", "--client-id", "id"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got %v", got)
	}
}

// 証跡ログが無ければ API クライアントを作らないこと。
func TestClientRequiresAuditLog(t *testing.T) {
	r := &config.Resolved{ClientID: "id", ClientSecret: "s", RefreshToken: "t"}
	if _, err := gtm.NewClient(context.Background(), r, nil); err == nil || !strings.Contains(err.Error(), "audit log") {
		t.Errorf("expected a refusal, got %v", err)
	}
}

// API を呼ぶコマンドは、開始と終了をログに残すこと（API 呼び出し前に失敗しても）。
func TestCommandIsLogged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GTM_HOME", home)
	t.Setenv("GTM_LOG_DIR", "")
	t.Setenv("GTM_CLIENT_ID", "id")
	t.Setenv("GTM_CLIENT_SECRET", "secret")
	if err := os.WriteFile(filepath.Join(home, "credentials.json"), []byte(`{"refresh_token":"rt"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := resolve()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openAuditLog(r); err != nil {
		t.Fatal(err)
	}
	closeAuditLog(nil)

	out := runCmd(t, "log", "show", "--since", "1h")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"phase":"start"`) || !strings.Contains(lines[1], `"phase":"end"`) {
		t.Fatalf("log: %s", out)
	}
}

func TestLogSummaryAllowAccount(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GTM_LOG_DIR", dir)
	writeLog(t, dir,
		`{"event":"api","method":"GET","path":"/tagmanager/v2/accounts","write":false}`,
		`{"event":"api","method":"GET","path":"/a","account":"1234567890","container":"1","write":false}`,
		`{"event":"api","method":"PUT","path":"/b","account":"1234567890","container":"1","write":true}`,
	)
	root := newRootCmd()
	root.SetArgs([]string{"log", "summary", "--allow-account", "1234567890"})
	if err := root.Execute(); err != nil {
		t.Fatalf("allowed account only: %v", err)
	}
	writeLog(t, dir, `{"event":"api","method":"GET","path":"/c","account":"999","write":false}`)
	root = newRootCmd()
	root.SetArgs([]string{"log", "summary", "--allow-account", "1234567890"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "outside --allow-account") {
		t.Fatalf("expected a violation, got %v", err)
	}
}

func writeLog(t *testing.T, dir string, lines ...string) {
	t.Helper()
	l, err := openTestLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for _, s := range lines {
		if _, err := l.WriteString(strings.Replace(s, "{", `{"time":"`+nowRFC3339()+`",`, 1) + "\n"); err != nil {
			t.Fatal(err)
		}
	}
}

func runCmd(t *testing.T, args ...string) string {
	t.Helper()
	old := os.Stdout
	rd, w, _ := os.Pipe()
	os.Stdout = w
	root := newRootCmd()
	root.SetArgs(args)
	err := root.Execute()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(rd)
	if err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	return buf.String()
}

func openTestLog(dir string) (*os.File, error) {
	return os.OpenFile(filepath.Join(dir, "audit-"+time.Now().Format("2006-01-02")+".jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

func nowRFC3339() string { return time.Now().Format(time.RFC3339Nano) }

func TestLogDirPrintsBarePath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GTM_LOG_DIR", dir)
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"log", "dir"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != dir+"\n" {
		t.Errorf("got %q, want %q", buf.String(), dir+"\n")
	}
}

func TestLogShowFailedAndInvocation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GTM_LOG_DIR", dir)
	writeLog(t, dir,
		`{"event":"command","phase":"start","invocation":"A","args":["tag","create"]}`,
		`{"event":"api","method":"POST","path":"/p","account":"1","container":"2","write":true,"status":400,"invocation":"A"}`,
		`{"event":"command","phase":"end","ok":false,"invocation":"A"}`,
		`{"event":"api","method":"GET","path":"/q","account":"1","container":"2","write":false,"status":200,"invocation":"B"}`,
		`{"event":"api","method":"GET","path":"/r","account":"1","write":false,"error":"timeout","invocation":"B"}`,
	)
	out := runCmd(t, "log", "show", "--failed")
	if n := strings.Count(out, "\n"); n != 3 || strings.Contains(out, `"/q"`) {
		t.Errorf("--failed: %s", out)
	}
	out = runCmd(t, "log", "show", "--invocation", "A")
	if n := strings.Count(out, "\n"); n != 3 || strings.Contains(out, `"B"`) {
		t.Errorf("--invocation: %s", out)
	}
}

func TestLogSummaryAllowContainer(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GTM_LOG_DIR", dir)
	writeLog(t, dir,
		`{"event":"api","method":"GET","path":"/a","account":"1","write":false}`,
		`{"event":"api","method":"PUT","path":"/b","account":"1","container":"10","write":true}`,
	)
	root := newRootCmd()
	root.SetArgs([]string{"log", "summary", "--allow-container", "10"})
	if err := root.Execute(); err != nil {
		t.Fatalf("account-level request and allowed container: %v", err)
	}
	writeLog(t, dir, `{"event":"api","method":"GET","path":"/c","account":"1","container":"11","write":false}`)
	root = newRootCmd()
	root.SetArgs([]string{"log", "summary", "--allow-container", "10"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "--allow-container") {
		t.Fatalf("expected a violation, got %v", err)
	}
}
