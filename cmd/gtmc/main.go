package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/ideamans/google-tag-manager-cli/cmd"
)

// GoReleaser が ldflags（-X main.version など）で差し込むビルド情報。
var (
	version = cmd.PluginVersion
	commit  = "none"
	date    = "unknown"
)

func main() {
	commit, date = resolveVCSInfo(commit, date)
	cmd.SetVersionInfo(version, commit, date)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "Error: "+err.Error())
		os.Exit(1)
	}
}

// resolveVCSInfo は ldflags で commit/date が入らなかった場合（go install / go run など）に、
// Go が自動で埋め込む VCS 情報（vcs.revision / vcs.time）で補う。
func resolveVCSInfo(commit, date string) (string, string) {
	if commit != "none" && date != "unknown" {
		return commit, date
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return commit, date
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "none" && s.Value != "" {
				commit = s.Value
			}
		case "vcs.time":
			if date == "unknown" && s.Value != "" {
				date = s.Value
			}
		}
	}
	return commit, date
}
