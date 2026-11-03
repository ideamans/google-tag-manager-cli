// Package cmd は CLI のコマンド定義（cobra）をまとめる。
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/ideamans/go-llm-cli-kit/llmcmd"
	"github.com/spf13/cobra"

	"github.com/ideamans/google-tag-manager-cli/internal/config"
	"github.com/ideamans/google-tag-manager-cli/internal/gtm"
)

// PluginVersion はこの CLI のリリースバージョン。
// plugins/google-tag-manager-cli/.claude-plugin/plugin.json の version と一致していることを
// テストが、git タグと一致していることをリリースワークフローが検査する。
const PluginVersion = "0.1.0"

// グローバルフラグの上書き値。各コマンドが config.Resolve に渡す。
var overrides config.Overrides

var buildVersion = PluginVersion

// SetVersionInfo は `-X main.version` 等で得たビルド情報を CLI の --version に反映する。
func SetVersionInfo(version, commit, date string) {
	buildVersion = fmt.Sprintf("%s (commit %s, built %s)", version, commit, date)
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "gtmc",
		Short: "CLI for the Google Tag Manager (GTM) API",
		Long: "CLI for Google Tag Manager (Tag Manager API v2). Inspects accounts, containers and workspaces; " +
			"edits tags, triggers, variables, folders, templates, server-side clients and transformations, zones and " +
			"Google tag configs (also in bulk); compiles a workspace without versioning; creates, publishes and " +
			"manages versions and staging environments; " +
			"exports a container in the GTM UI import format; and audits a container for cleanup candidates.",
		Version:       buildVersion,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	pf := root.PersistentFlags()
	pf.StringVar(&overrides.Account, "account", "", "GTM account ID (default: $GTM_ACCOUNT_ID / credentials.json)")
	pf.StringVar(&overrides.Container, "container", "", "Container: GTM-XXXX public ID, numeric ID (needs --account), or accounts/<a>/containers/<c> (default: $GTM_CONTAINER_ID / credentials.json)")
	pf.StringVar(&overrides.Workspace, "workspace", "", "Workspace ID (default: $GTM_WORKSPACE_ID / credentials.json / the only workspace)")
	pf.StringVar(&overrides.ClientID, "client-id", "", "OAuth client ID (default: $GTM_CLIENT_ID / oauth2-client.json)")
	pf.StringVar(&overrides.ClientSecret, "client-secret", "", "OAuth client secret (default: $GTM_CLIENT_SECRET / oauth2-client.json)")

	root.AddCommand(
		newAuthCmd(),
		newAccountCmd(),
		newContainerCmd(),
		newWorkspaceCmd(),
	)
	for _, k := range gtm.Kinds {
		root.AddCommand(newEntityCmd(k))
	}
	root.AddCommand(
		newBuiltInVariableCmd(),
		newVersionCmd(),
		newEnvironmentCmd(),
		newExportCmd(),
		newAuditCmd(),
		newLogCmd(),
	)
	llmcmd.AddTo(root, LLMConfig())
	return root
}

// Root は組み立て済みのコマンドツリーを実行せずに返す（カタログ生成用）。
func Root() *cobra.Command { return newRootCmd() }

// ExecuteContext はルートコマンドを指定の context で実行する。
func ExecuteContext(ctx context.Context) error {
	if handled, err := llmcmd.HandleLegacy(os.Args[1:], LLMConfig(), os.Stdout); handled {
		return err
	}
	err := newRootCmd().ExecuteContext(ctx)
	closeAuditLog(err)
	return err
}

func resolve() (*config.Resolved, error) {
	return config.Resolve(overrides)
}

// newClient は認証済み API クライアントを生成する。各業務コマンドの入口で使う。
func newClient(ctx context.Context) (*gtm.Client, error) {
	r, err := resolve()
	if err != nil {
		return nil, err
	}
	log, err := openAuditLog(r)
	if err != nil {
		return nil, err
	}
	return gtm.NewClient(ctx, r, log)
}
