package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ideamans/google-tag-manager-cli/internal/auditlog"
	"github.com/ideamans/google-tag-manager-cli/internal/config"
)

func newLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Read the local audit log of every API request this CLI made",
		Long: "Every command that calls the API appends to a local audit log (JSON Lines, one file per day):\n" +
			"a command start/end record with the arguments and resolved target, and one record per HTTP\n" +
			"request (method, path, account/container/workspace, status, abbreviated request body).\n" +
			"Long strings and arrays in bodies are abbreviated; the OAuth token and --client-secret are never\n" +
			"written. The CLI refuses to call the API if the log cannot be opened.\n" +
			"Location: $GTM_LOG_DIR, else $XDG_STATE_HOME/google-tag-manager/logs, else\n" +
			"~/.local/state/google-tag-manager/logs. Files older than $GTM_LOG_RETENTION_DAYS days\n" +
			"(default 90) are deleted automatically.",
	}
	cmd.AddCommand(newLogDirCmd(), newLogPathCmd(), newLogShowCmd(), newLogSummaryCmd())
	return cmd
}

func newLogDirCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dir",
		Short: "Print the audit log directory as a bare path (for cd, open, ls)",
		Long: "Prints only the directory the audit log is written to, followed by a newline, so it can be used\n" +
			"as `open \"$(gtmc log dir)\"` or `ls \"$(gtmc log dir)\"`. The directory may not exist yet\n" +
			"if no command has called the API. `log path` prints the same directory with the files and the\n" +
			"retention as JSON.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := auditlog.Dir()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), dir)
			return err
		},
	}
}

func newLogPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the audit log directory and its files",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := auditlog.Dir()
			if err != nil {
				return err
			}
			files, err := auditlog.Files(dir)
			if err != nil {
				return err
			}
			if files == nil {
				files = []string{}
			}
			return renderJSON(map[string]any{"dir": dir, "retentionDays": auditlog.RetentionDays(), "files": files})
		},
	}
}

// parseSince は "24h" のような期間か、"2026-11-03" のような日付を受ける。
func parseSince(s string) (time.Time, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("--since must be a duration (24h, 30m) or a date (2026-11-03)")
}

type logFilter struct {
	since              string
	account, container string
	invocation         string
	writes, failed     bool
}

func (f *logFilter) add(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.since, "since", "24h", "Start: a duration back from now (24h, 30m) or a date (2026-11-03)")
	fl.StringVar(&f.account, "account", "", "Only records for this account ID")
	fl.StringVar(&f.container, "container-id", "", "Only records for this numeric container ID")
	fl.BoolVar(&f.writes, "writes", false, "Only API requests that change something (non-GET)")
	fl.StringVar(&f.invocation, "invocation", "", "Only records of this CLI run (the \"invocation\" field)")
	fl.BoolVar(&f.failed, "failed", false, "Only failures: API requests with an error or status >= 400, and commands that ended with ok=false")
}

// failedRecord は失敗した API リクエストか、失敗で終わったコマンドの行か。
func failedRecord(r map[string]any) bool {
	if str(r, "error") != "" {
		return true
	}
	if st, ok := r["status"].(float64); ok && st >= 400 {
		return true
	}
	return str(r, "event") == "command" && str(r, "phase") == "end" && r["ok"] == false
}

func (f *logFilter) read() ([]map[string]any, error) {
	since, err := parseSince(f.since)
	if err != nil {
		return nil, err
	}
	dir, err := auditlog.Dir()
	if err != nil {
		return nil, err
	}
	recs, err := auditlog.Read(dir, since)
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, r := range recs {
		if f.account != "" && str(r, "account") != config.NormalizeID(f.account) {
			continue
		}
		if f.container != "" && str(r, "container") != config.NormalizeID(f.container) {
			continue
		}
		if f.writes && r["write"] != true {
			continue
		}
		if f.invocation != "" && str(r, "invocation") != f.invocation {
			continue
		}
		if f.failed && !failedRecord(r) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func newLogShowCmd() *cobra.Command {
	var f logFilter
	var limit int
	cmd := &cobra.Command{
		Use:   "show [--since 24h] [--account ID] [--container-id ID] [--writes] [--failed] [--invocation ID] [--limit N]",
		Short: "Print audit log records as JSON Lines (oldest first)",
		Long: "Prints matching records, one JSON object per line, oldest first. --limit keeps the newest N.\n" +
			"API records have event \"api\"; command records have event \"command\" and phase start/end.\n" +
			"With --account/--container-id/--writes only API records can match. --invocation keeps one CLI run\n" +
			"(its start record carries the arguments), so `--writes` then `--invocation <id>` answers\n" +
			"\"which command made this change\".",
		RunE: func(cmd *cobra.Command, args []string) error {
			recs, err := f.read()
			if err != nil {
				return err
			}
			if limit > 0 && len(recs) > limit {
				recs = recs[len(recs)-limit:]
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetEscapeHTML(false)
			for _, r := range recs {
				if err := enc.Encode(r); err != nil {
					return err
				}
			}
			return nil
		},
	}
	f.add(cmd)
	cmd.Flags().IntVar(&limit, "limit", 0, "Keep only the newest N records (0: all)")
	return cmd
}

type scopeCount struct {
	Reads  int `json:"reads"`
	Writes int `json:"writes"`
}

func newLogSummaryCmd() *cobra.Command {
	var since string
	var allow, allowCt []string
	cmd := &cobra.Command{
		Use:   "summary [--since 24h] [--allow-account ID,...] [--allow-container ID,...]",
		Short: "Count API requests per account and container; --allow-account/--allow-container fail on anything else",
		Long: "Summarizes which accounts and containers were touched, with read (GET) and write counts.\n" +
			"Requests without an account in the path (accounts.list, containers.lookup before the result\n" +
			"is known) are counted under \"unscoped\".\n" +
			"--allow-account lists the accounts that may appear: the command exits 1 and lists the\n" +
			"violations if any API request touched another account. Use it to prove a session stayed\n" +
			"inside the accounts it was allowed to use. --allow-container does the same for numeric container\n" +
			"IDs (not GTM-XXXX): requests inside an account but outside any container (accounts.get,\n" +
			"containers.list) are not violations of it, requests in another container are.\n" +
			"The output also has failedRequests (error or status >= 400) and the write count per method.",
		RunE: func(cmd *cobra.Command, args []string) error {
			f := logFilter{since: since}
			recs, err := f.read()
			if err != nil {
				return err
			}
			accounts := map[string]map[string]*scopeCount{} // account -> container("" = account level)
			unscoped := &scopeCount{}
			invocations := map[string]bool{}
			calls := 0
			allowed := map[string]bool{}
			for _, a := range allow {
				allowed[config.NormalizeID(a)] = true
			}
			allowedCt := map[string]bool{}
			for _, c := range allowCt {
				allowedCt[config.NormalizeID(c)] = true
			}
			violations := []map[string]any{}
			failed := 0
			writesByMethod := map[string]int{}
			for _, r := range recs {
				invocations[str(r, "invocation")] = true
				if str(r, "event") != "api" {
					continue
				}
				calls++
				if failedRecord(r) {
					failed++
				}
				if r["write"] == true {
					writesByMethod[str(r, "method")]++
				}
				c := unscoped
				if acc := str(r, "account"); acc != "" {
					if accounts[acc] == nil {
						accounts[acc] = map[string]*scopeCount{}
					}
					ct := str(r, "container")
					if accounts[acc][ct] == nil {
						accounts[acc][ct] = &scopeCount{}
					}
					c = accounts[acc][ct]
					reason := ""
					if len(allowed) > 0 && !allowed[acc] {
						reason = "account"
					} else if len(allowedCt) > 0 && ct != "" && !allowedCt[ct] {
						reason = "container"
					}
					if reason != "" {
						violations = append(violations, map[string]any{
							"time": r["time"], "invocation": r["invocation"], "method": r["method"],
							"path": r["path"], "account": acc, "container": ct, "outside": reason,
						})
					}
				}
				if r["write"] == true {
					c.Writes++
				} else {
					c.Reads++
				}
			}
			accList := []map[string]any{}
			for _, acc := range sortedKeys(accounts) {
				cts := []map[string]any{}
				total := scopeCount{}
				for _, ct := range sortedKeys(accounts[acc]) {
					sc := accounts[acc][ct]
					total.Reads += sc.Reads
					total.Writes += sc.Writes
					if ct == "" {
						continue
					}
					cts = append(cts, map[string]any{"container": ct, "reads": sc.Reads, "writes": sc.Writes})
				}
				accList = append(accList, map[string]any{"account": acc, "reads": total.Reads, "writes": total.Writes, "containers": cts})
			}
			out := map[string]any{
				"since":          since,
				"invocations":    len(invocations),
				"apiRequests":    calls,
				"accounts":       accList,
				"unscoped":       unscoped,
				"failedRequests": failed,
				"writesByMethod": writesByMethod,
			}
			if len(allowed) > 0 {
				out["allowedAccounts"] = allow
			}
			if len(allowedCt) > 0 {
				out["allowedContainers"] = allowCt
			}
			if len(allowed) > 0 || len(allowedCt) > 0 {
				out["violations"] = violations
			}
			if err := renderJSON(out); err != nil {
				return err
			}
			if len(violations) > 0 {
				return fmt.Errorf("%d API request(s) touched accounts or containers outside --allow-account/--allow-container (%s)",
					len(violations), strings.Join(append(append([]string{}, allow...), allowCt...), ","))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "Start: a duration back from now (24h, 30m) or a date (2026-11-03)")
	cmd.Flags().StringSliceVar(&allow, "allow-account", nil, "Accounts that may appear; any other fails the command")
	cmd.Flags().StringSliceVar(&allowCt, "allow-container", nil, "Numeric container IDs that may appear; any other fails the command")
	return cmd
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
