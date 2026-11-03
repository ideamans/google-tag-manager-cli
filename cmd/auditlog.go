package cmd

import (
	"os"
	"strings"
	"time"

	"github.com/ideamans/google-tag-manager-cli/internal/auditlog"
	"github.com/ideamans/google-tag-manager-cli/internal/config"
)

// 証跡ログ（internal/auditlog）は API クライアントを作るときに初めて開く。
// API を呼ばないコマンド（llm・help・オフラインの audit --file）は記録しない。
var session struct {
	log   *auditlog.Logger
	start time.Time
}

// openAuditLog は証跡ログを開き、この実行の開始（引数と解決済みの対象）を記録する。
func openAuditLog(r *config.Resolved) (*auditlog.Logger, error) {
	if session.log != nil {
		return session.log, nil
	}
	dir, err := auditlog.Dir()
	if err != nil {
		return nil, err
	}
	l, err := auditlog.Open(dir)
	if err != nil {
		return nil, err
	}
	session.log, session.start = l, time.Now()
	rec := map[string]any{
		"event":   "command",
		"phase":   "start",
		"args":    redactArgs(os.Args[1:]),
		"version": buildVersion,
	}
	target := map[string]any{}
	if r.AccountID != "" {
		target["account"] = r.AccountID
	}
	if r.Container != "" {
		target["container"] = r.Container
	}
	if r.WorkspaceID != "" {
		target["workspace"] = r.WorkspaceID
	}
	if len(target) > 0 {
		rec["target"] = target
	}
	if err := l.Write(rec); err != nil {
		return nil, err
	}
	return l, nil
}

// closeAuditLog はこの実行の終わり（成否）を記録して閉じる。
func closeAuditLog(runErr error) {
	if session.log == nil {
		return
	}
	rec := map[string]any{
		"event":      "command",
		"phase":      "end",
		"ok":         runErr == nil,
		"durationMs": time.Since(session.start).Milliseconds(),
	}
	if runErr != nil {
		msg := runErr.Error()
		if len(msg) > 512 {
			msg = msg[:512] + "…"
		}
		rec["error"] = msg
	}
	_ = session.log.Write(rec)
	_ = session.log.Close()
	session.log = nil
}

// secretFlags は値を記録しないフラグ。
var secretFlags = map[string]bool{"--client-secret": true}

// redactArgs は秘密の値を伏せた引数を返す。
func redactArgs(args []string) []string {
	out := make([]string, len(args))
	copy(out, args)
	for i := 0; i < len(out); i++ {
		a := out[i]
		if name, _, ok := strings.Cut(a, "="); ok && secretFlags[name] {
			out[i] = name + "=***"
			continue
		}
		if secretFlags[a] && i+1 < len(out) {
			out[i+1] = "***"
			i++
		}
	}
	return out
}
