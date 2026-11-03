package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	tm "google.golang.org/api/tagmanager/v2"

	"github.com/ideamans/google-tag-manager-cli/internal/config"
	"github.com/ideamans/google-tag-manager-cli/internal/gtm"
)

func newEnvironmentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "environment",
		Aliases: []string{"env"},
		Short:   "List, create, update and delete environments (staging installs of the container)",
		Long: "An environment is a separate install of the container — e.g. a staging site — that runs a\n" +
			"chosen version instead of the live one. Every container has the built-in Live and Latest\n" +
			"environments; user environments are created here.\n" +
			"A page loads an environment by adding gtm_auth=<authorizationCode>&gtm_preview=env-<environmentId>\n" +
			"&gtm_cookies_win=x to the gtm.js URL in the snippet. `environment list` prints that query string.\n" +
			"Deploy a version to a user environment with `environment update <id> --version <version-id>`;\n" +
			"this changes only pages that load that environment, not the live site.",
	}
	cmd.AddCommand(newEnvListCmd(), newEnvGetCmd(), newEnvCreateCmd(), newEnvUpdateCmd(), newEnvDeleteCmd(), newEnvReauthorizeCmd())
	return cmd
}

// viewEnv は環境に、設置コードへ足すクエリ文字列（previewQuery）を添えた JSON 用の値を返す。
// tm.Environment は独自の MarshalJSON を持つので、埋め込みではなく map に足す。
func viewEnv(e *tm.Environment) map[string]any {
	m, err := toMap(e)
	if err != nil {
		return map[string]any{"environment": e}
	}
	if e.AuthorizationCode != "" {
		m["previewQuery"] = fmt.Sprintf("gtm_auth=%s&gtm_preview=env-%s&gtm_cookies_win=x", e.AuthorizationCode, e.EnvironmentId)
	}
	return m
}

func envPath(cmd *cobra.Command, id string) (*gtm.Client, string, error) {
	ctx := cmd.Context()
	client, err := newClient(ctx)
	if err != nil {
		return nil, "", err
	}
	cp, err := client.ContainerPath(ctx)
	if err != nil {
		return nil, "", err
	}
	return client, cp + "/environments/" + config.NormalizeID(id), nil
}

func newEnvListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List environments with their deployed version and preview query string",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(format); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			cp, err := client.ContainerPath(ctx)
			if err != nil {
				return err
			}
			es, err := client.ListEnvironments(ctx, cp)
			if err != nil {
				return err
			}
			if format == "table" {
				var rows [][]string
				for _, e := range es {
					typ := e.Type
					if typ == "" {
						typ = "user" // 利用者が作った環境は API が type を返さない
					}
					rows = append(rows, []string{e.EnvironmentId, typ, e.Name, e.ContainerVersionId, e.WorkspaceId, e.Url})
				}
				printTable([]string{"ENV", "TYPE", "NAME", "VERSION", "WORKSPACE", "URL"}, rows)
				return nil
			}
			out := make([]map[string]any, len(es))
			for i, e := range es {
				out[i] = viewEnv(e)
			}
			return renderJSON(out)
		},
	}
	addFormatFlag(cmd, &format)
	return cmd
}

func newEnvGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <environment-id>",
		Short: "Fetch one environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, path, err := envPath(cmd, args[0])
			if err != nil {
				return err
			}
			e, err := client.GetEnvironment(cmd.Context(), path)
			if err != nil {
				return err
			}
			return renderJSON(viewEnv(e))
		},
	}
}

// envFlags は create / update で共通のフラグ。
type envFlags struct {
	name, description, url, version string
	debug                           bool
}

func (f *envFlags) add(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.name, "name", "", "Environment name")
	fl.StringVar(&f.description, "description", "", "Environment description")
	fl.StringVar(&f.url, "url", "", "Default preview URL (the staging site)")
	fl.StringVar(&f.version, "version", "", "Container version ID to deploy to this environment")
	fl.BoolVar(&f.debug, "enable-debug", false, "Open the debug pane by default when previewing")
}

func (f *envFlags) apply(cmd *cobra.Command, e *tm.Environment) {
	fl := cmd.Flags()
	if fl.Changed("name") {
		e.Name = f.name
	}
	if fl.Changed("description") {
		e.Description = f.description
		e.ForceSendFields = append(e.ForceSendFields, "Description")
	}
	if fl.Changed("url") {
		e.Url = f.url
	}
	if fl.Changed("version") {
		e.ContainerVersionId = config.NormalizeID(f.version)
	}
	if fl.Changed("enable-debug") {
		e.EnableDebug = f.debug
		e.ForceSendFields = append(e.ForceSendFields, "EnableDebug")
	}
}

func newEnvCreateCmd() *cobra.Command {
	var f envFlags
	cmd := &cobra.Command{
		Use:   "create --name <name> [--url ...] [--description ...] [--version <id>] [--enable-debug]",
		Short: "Create a user environment (e.g. staging)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.name == "" {
				return fmt.Errorf("--name is required")
			}
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			cp, err := client.ContainerPath(ctx)
			if err != nil {
				return err
			}
			e := &tm.Environment{Type: "user"}
			f.apply(cmd, e)
			v, err := client.CreateEnvironment(ctx, cp, e)
			if err != nil {
				return err
			}
			return renderJSON(viewEnv(v))
		},
	}
	f.add(cmd)
	return cmd
}

func newEnvUpdateCmd() *cobra.Command {
	var f envFlags
	cmd := &cobra.Command{
		Use:   "update <environment-id> [--version <id>] [--name ...] [--url ...] [--description ...] [--enable-debug]",
		Short: "Change an environment; --version deploys a version to it (not to the live site)",
		Long: "Changes a user environment. --version <id> deploys that container version to the environment:\n" +
			"pages loading this environment start running it, the live site does not change.\n" +
			"Name and description can be changed only on user environments.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fl := cmd.Flags()
			if !fl.Changed("name") && !fl.Changed("description") && !fl.Changed("url") && !fl.Changed("version") && !fl.Changed("enable-debug") {
				return fmt.Errorf("nothing to update: pass --version, --name, --description, --url or --enable-debug")
			}
			client, path, err := envPath(cmd, args[0])
			if err != nil {
				return err
			}
			v, err := client.UpdateEnvironment(cmd.Context(), path, func(e *tm.Environment) { f.apply(cmd, e) })
			if err != nil {
				return err
			}
			return renderJSON(viewEnv(v))
		},
	}
	f.add(cmd)
	return cmd
}

func newEnvDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <environment-id> --yes",
		Short: "Delete a user environment (requires --yes; pages still loading it stop getting tags)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireYes(yes, "deleting an environment breaks every page that loads it"); err != nil {
				return err
			}
			client, path, err := envPath(cmd, args[0])
			if err != nil {
				return err
			}
			if err := client.DeleteEnvironment(cmd.Context(), path); err != nil {
				return err
			}
			return renderJSON(map[string]any{"deleted": path})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm the deletion")
	return cmd
}

func newEnvReauthorizeCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "reauthorize <environment-id> --yes",
		Short: "Regenerate an environment's authorization code (requires --yes; old links and snippets stop working)",
		Long: "Issues a new authorizationCode. Every snippet and shared preview link that carries the old\n" +
			"gtm_auth stops working until it is replaced — use it when a code has leaked.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireYes(yes, "reauthorizing invalidates every snippet and preview link using the old code"); err != nil {
				return err
			}
			client, path, err := envPath(cmd, args[0])
			if err != nil {
				return err
			}
			v, err := client.ReauthorizeEnvironment(cmd.Context(), path)
			if err != nil {
				return err
			}
			return renderJSON(viewEnv(v))
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm regenerating the code")
	return cmd
}
