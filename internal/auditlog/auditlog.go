// Package auditlog は「どの対象にどんな操作をしたか」の証跡を、ローカルに JSON Lines で
// 残し続ける。意図しないアカウントやコンテナを触っていないことを後から確かめるためのもの。
//
// 置き場所（優先順）:
//
//	$GTM_LOG_DIR
//	$GTM_HOME/logs                 （設定ディレクトリを差し替えているとき。テスト用）
//	$XDG_STATE_HOME/google-tag-manager/logs
//	~/.local/state/google-tag-manager/logs        （XDG Base Directory の state。ログの標準的な置き場）
//
// ファイルは1日1本（audit-YYYY-MM-DD.jsonl）。開くたびに保持期間（既定 90 日、
// $GTM_LOG_RETENTION_DAYS で変更）より古いファイルを消す。追記は O_APPEND の1回の
// write なので、複数のプロセスが同時に書いても行は混ざらない。
package auditlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	appDirName           = "google-tag-manager"
	envLogDir            = "GTM_LOG_DIR"
	envConfigDir         = "GTM_HOME"
	envRetentionDays     = "GTM_LOG_RETENTION_DAYS"
	DefaultRetentionDays = 90
	filePrefix           = "audit-"
	fileSuffix           = ".jsonl"
)

var fileNameRe = regexp.MustCompile(`^audit-(\d{4}-\d{2}-\d{2})\.jsonl$`)

// Dir はログディレクトリを返す。
func Dir() (string, error) {
	if d := os.Getenv(envLogDir); d != "" {
		return d, nil
	}
	if d := os.Getenv(envConfigDir); d != "" {
		return filepath.Join(d, "logs"), nil
	}
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, appDirName, "logs"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine the home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", appDirName, "logs"), nil
}

// RetentionDays は保持日数を返す（環境変数が不正なら既定値）。
func RetentionDays() int {
	if v := os.Getenv(envRetentionDays); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return DefaultRetentionDays
}

// Logger は1回の CLI 実行（invocation）分の書き手。
type Logger struct {
	mu         sync.Mutex
	f          *os.File
	path       string
	invocation string
	now        func() time.Time
	warned     bool
}

// Open は今日のログファイルを開き、保持期間を過ぎたファイルを消す。
func Open(dir string) (*Logger, error) {
	return open(dir, RetentionDays(), time.Now)
}

func open(dir string, retention int, now func() time.Time) (*Logger, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("cannot create the audit log directory %s: %w", dir, err)
	}
	t := now()
	path := filepath.Join(dir, filePrefix+t.Format("2006-01-02")+fileSuffix)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("cannot open the audit log %s: %w", path, err)
	}
	prune(dir, retention, t)
	return &Logger{f: f, path: path, invocation: newInvocationID(t), now: now}, nil
}

// prune は保持期間を過ぎたログファイルを消す（失敗しても記録は続ける）。
func prune(dir string, retention int, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := now.AddDate(0, 0, -retention).Format("2006-01-02")
	for _, e := range entries {
		m := fileNameRe.FindStringSubmatch(e.Name())
		if m != nil && m[1] < cutoff {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func newInvocationID(t time.Time) string {
	return fmt.Sprintf("%s-%d", t.Format("20060102T150405.000"), os.Getpid())
}

// Path は書き込み中のファイルのパス。
func (l *Logger) Path() string { return l.path }

// Invocation はこの実行の ID（同じ実行の行をまとめる鍵）。
func (l *Logger) Invocation() string { return l.invocation }

// Write は1行書く。time と invocation は自動で付く。
func (l *Logger) Write(rec map[string]any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]any, len(rec)+2)
	for k, v := range rec {
		out[k] = v
	}
	out["time"] = l.now().Format(time.RFC3339Nano)
	out["invocation"] = l.invocation
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, err = l.f.Write(append(b, '\n'))
	if err != nil && !l.warned {
		l.warned = true
		fmt.Fprintf(os.Stderr, "Warning: cannot write the audit log %s: %v\n", l.path, err)
	}
	return err
}

// Close はファイルを閉じる。
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}

// ---- 長い値の省略 ----

const (
	maxString  = 256  // 文字列はこの文字数で切る
	maxArray   = 50   // 配列はこの要素数で切る
	maxRawBody = 1024 // JSON でない本文はこのバイト数で切る
)

// Abbreviate は JSON 本文を、長い文字列・長い配列を省略した値にして返す。
func Abbreviate(body []byte) any {
	if len(body) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		s := string(body)
		if len(s) > maxRawBody {
			return s[:maxRawBody] + fmt.Sprintf("…(%d bytes)", len(body))
		}
		return s
	}
	return abbreviate(v)
}

func abbreviate(v any) any {
	switch x := v.(type) {
	case string:
		r := []rune(x)
		if len(r) > maxString {
			return string(r[:maxString]) + fmt.Sprintf("…(%d chars)", len(r))
		}
		return x
	case []any:
		n := len(x)
		if n > maxArray {
			x = x[:maxArray]
		}
		out := make([]any, 0, len(x)+1)
		for _, e := range x {
			out = append(out, abbreviate(e))
		}
		if n > maxArray {
			out = append(out, fmt.Sprintf("…(%d more)", n-maxArray))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = abbreviate(e)
		}
		return out
	}
	return v
}

// ---- 読み出し ----

// Files はログファイルを古い順に返す。
func Files(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if fileNameRe.MatchString(e.Name()) {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// Read は since 以降の行を古い順に返す（壊れた行は読み飛ばす）。
func Read(dir string, since time.Time) ([]map[string]any, error) {
	files, err := Files(dir)
	if err != nil {
		return nil, err
	}
	day := since.Format("2006-01-02")
	var out []map[string]any
	for _, p := range files {
		if m := fileNameRe.FindStringSubmatch(filepath.Base(p)); m != nil && m[1] < day {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(b), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var rec map[string]any
			if json.Unmarshal([]byte(line), &rec) != nil {
				continue
			}
			if t, err := time.Parse(time.RFC3339Nano, fmt.Sprint(rec["time"])); err == nil && t.Before(since) {
				continue
			}
			out = append(out, rec)
		}
	}
	return out, nil
}
