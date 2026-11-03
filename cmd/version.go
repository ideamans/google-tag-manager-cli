package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ideamans/google-tag-manager-cli/internal/config"
	"github.com/ideamans/google-tag-manager-cli/internal/gtm"
)

func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"ver"},
		Short:   "List, inspect, create, publish, rename, delete and undelete container versions",
	}
	cmd.AddCommand(newVersionListCmd(), newVersionGetCmd(), newVersionCreateCmd(), newVersionPublishCmd(),
		newVersionUpdateCmd(), newVersionDeleteCmd(), newVersionUndeleteCmd(), newVersionSetLatestCmd())
	return cmd
}

func newVersionListCmd() *cobra.Command {
	var format string
	var includeDeleted bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List version headers (ID, name, entity counts) without their contents",
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
			hs, err := client.ListVersionHeaders(ctx, cp, includeDeleted)
			if err != nil {
				return err
			}
			if format == "table" {
				var rows [][]string
				for _, h := range hs {
					deleted := ""
					if h.Deleted {
						deleted = "deleted"
					}
					rows = append(rows, []string{h.ContainerVersionId, h.Name, h.NumTags, h.NumTriggers, h.NumVariables, deleted})
				}
				printTable([]string{"VERSION", "NAME", "TAGS", "TRIGGERS", "VARIABLES", ""}, rows)
				return nil
			}
			return renderJSON(hs)
		},
	}
	cmd.Flags().BoolVar(&includeDeleted, "include-deleted", false, "Include deleted versions")
	addFormatFlag(cmd, &format)
	return cmd
}

func newVersionGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [live|latest|<version-id>]",
		Short: "Fetch a whole version (tags, triggers, variables, ...); default: live",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			ref := "live"
			if len(args) == 1 {
				ref = args[0]
			}
			v, err := client.Snapshot(ctx, ref)
			if err != nil {
				return err
			}
			return renderJSON(v)
		},
	}
}

func newVersionCreateCmd() *cobra.Command {
	var name, notes string
	cmd := &cobra.Command{
		Use:   "create --name <name> [--notes ...]",
		Short: "Create a version from the workspace (the workspace is consumed; nothing is published)",
		Long: "Saves the workspace as a new container version. GTM consumes the workspace in the process:\n" +
			"GTM creates a fresh workspace (newWorkspacePath) only when it was the container's last one.\n" +
			"If the consumed workspace was the saved default, the default moves to the new one, or is\n" +
			"cleared when no workspace was created.\n" +
			"Nothing changes on the site until `version publish`. Fails with compilerErrors if the\n" +
			"workspace has errors, and merge conflicts must be resolved first (`workspace status`).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required (it is what people see in the version history)")
			}
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			ws, err := client.WorkspacePath(ctx)
			if err != nil {
				return err
			}
			consumed := client.WorkspaceID()
			r, err := client.CreateVersion(ctx, ws, name, notes)
			if r != nil {
				_ = renderJSON(r)
			}
			if err != nil {
				return err
			}
			// 消費されたワークスペースが保存済みの既定なら、既定を差し替える。GTM が新しい
			// ワークスペースを作るのは、それが最後の1つだったときだけ（newWorkspacePath）。
			// 他にワークスペースが残っていれば何も作られないので、既定を外す。
			creds, cerr := config.LoadCredentials()
			if cerr == nil && creds.WorkspaceID == consumed && creds.ContainerID == r.ContainerVersion.ContainerId {
				creds.WorkspaceID = config.NormalizeID(r.NewWorkspacePath)
				if config.SaveCredentials(creds) == nil {
					if creds.WorkspaceID != "" {
						fmt.Fprintf(os.Stderr, "Default workspace moved to %s (workspace %s was consumed)\n", creds.WorkspaceID, consumed)
					} else {
						fmt.Fprintf(os.Stderr, "Workspace %s was consumed and the default workspace was cleared; create one with `workspace create --use`\n", consumed)
					}
				}
			}
			fmt.Fprintf(os.Stderr, "✓ Created version %s. Publish it with: gtmc version publish %s --yes\n",
				r.ContainerVersion.ContainerVersionId, r.ContainerVersion.ContainerVersionId)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Version name (required)")
	cmd.Flags().StringVar(&notes, "notes", "", "Version description: what changed and why")
	return cmd
}

func newVersionPublishCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "publish <version-id|latest> --yes",
		Short: "Publish a version to the live site (requires --yes)",
		Long: "Makes the version live: every site using the container starts running it within minutes.\n" +
			"To roll back, publish the previous version ID again (see `version list`).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireYes(yes, "publishing changes what runs on the live site"); err != nil {
				return err
			}
			if args[0] == "live" {
				return fmt.Errorf("give a version ID or latest; the live version is already published")
			}
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			path, err := client.VersionPath(ctx, args[0])
			if err != nil {
				return err
			}
			r, err := client.PublishVersion(ctx, path)
			if r != nil {
				_ = renderJSON(r)
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm publishing to the live site")
	return cmd
}

func newVersionUpdateCmd() *cobra.Command {
	var name, notes string
	cmd := &cobra.Command{
		Use:   "update <version-id|live|latest> [--name ...] [--notes ...]",
		Short: "Rename a version or rewrite its notes (the contents cannot be changed)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			var np, dp *string
			if f.Changed("name") {
				np = &name
			}
			if f.Changed("notes") {
				dp = &notes
			}
			if np == nil && dp == nil {
				return fmt.Errorf("nothing to update: pass --name or --notes")
			}
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			path, err := client.VersionPath(ctx, args[0])
			if err != nil {
				return err
			}
			v, err := client.UpdateVersion(ctx, path, np, dp)
			if err != nil {
				return err
			}
			return renderJSON(map[string]any{"path": v.Path, "containerVersionId": v.ContainerVersionId, "name": v.Name, "description": v.Description})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New version name")
	cmd.Flags().StringVar(&notes, "notes", "", "New version description")
	return cmd
}

func newVersionDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <version-id> --yes",
		Short: "Delete a version from the history (requires --yes; `version undelete` restores it)",
		Long: "Hides a version from the version history. It does not change what is live, but a deleted\n" +
			"version can no longer be chosen for rollback until it is restored with `version undelete`\n" +
			"(`version list --include-deleted` shows it). The live version is refused.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireYes(yes, "deleting a version removes it from the rollback history"); err != nil {
				return err
			}
			if a := strings.ToLower(args[0]); a == "live" || a == "latest" {
				return fmt.Errorf("give an explicit version ID, not %s", args[0])
			}
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			path, err := client.VersionPath(ctx, args[0])
			if err != nil {
				return err
			}
			// 公開中のバージョンは消さない（切り戻し先の把握を壊す）。未公開のコンテナは 404 なので通す。
			live, lerr := client.Snapshot(ctx, "live")
			if lerr == nil && live.Path == path {
				return fmt.Errorf("version %s is the live version; publish another version first if it really has to go", live.ContainerVersionId)
			}
			if lerr != nil && gtm.StatusCode(lerr) != 404 {
				return lerr
			}
			if err := client.DeleteVersion(ctx, path); err != nil {
				return err
			}
			return renderJSON(map[string]any{"deleted": path})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm the deletion")
	return cmd
}

func newVersionUndeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undelete <version-id>",
		Short: "Restore a deleted version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			path, err := client.VersionPath(ctx, args[0])
			if err != nil {
				return err
			}
			v, err := client.UndeleteVersion(ctx, path)
			if err != nil {
				return err
			}
			return renderJSON(map[string]any{"path": v.Path, "containerVersionId": v.ContainerVersionId, "name": v.Name, "deleted": v.Deleted})
		},
	}
}

func newVersionSetLatestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-latest <version-id>",
		Short: "Make a version the base that workspaces sync with and are compared against",
		Long: "Sets which version counts as \"latest\": workspaces sync from it and their changes and\n" +
			"conflicts are computed against it. It does not publish anything.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			path, err := client.VersionPath(ctx, args[0])
			if err != nil {
				return err
			}
			v, err := client.SetLatestVersion(ctx, path)
			if err != nil {
				return err
			}
			return renderJSON(map[string]any{"path": v.Path, "containerVersionId": v.ContainerVersionId, "name": v.Name})
		},
	}
}
