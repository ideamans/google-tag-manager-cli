package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	tm "google.golang.org/api/tagmanager/v2"

	"github.com/ideamans/google-tag-manager-cli/internal/config"
	"github.com/ideamans/google-tag-manager-cli/internal/gtm"
)

func newWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace",
		Aliases: []string{"ws"},
		Short:   "Create, inspect, preview, sync, bulk-edit and delete workspaces; resolve conflicts; choose the default",
		Long: "Every edit in GTM happens in a workspace and stays invisible to the site until a version is\n" +
			"created from it and published. Create a dedicated workspace for each piece of work.",
	}
	cmd.AddCommand(
		newWorkspaceListCmd(),
		newWorkspaceInfoCmd(),
		newWorkspaceCreateCmd(),
		newWorkspaceUseCmd(),
		newWorkspaceUpdateCmd(),
		newWorkspaceStatusCmd(),
		newWorkspacePreviewCmd(),
		newWorkspaceSyncCmd(),
		newWorkspaceResolveCmd(),
		newWorkspaceBulkUpdateCmd(),
		newWorkspaceDeleteCmd(),
	)
	return cmd
}

func newWorkspaceListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspaces in the container",
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
			wss, err := client.ListWorkspaces(ctx, cp)
			if err != nil {
				return err
			}
			if format == "table" {
				var rows [][]string
				for _, w := range wss {
					rows = append(rows, []string{w.WorkspaceId, w.Name, w.Description})
				}
				printTable([]string{"WORKSPACE", "NAME", "DESCRIPTION"}, rows)
				return nil
			}
			return renderJSON(wss)
		},
	}
	addFormatFlag(cmd, &format)
	return cmd
}

// workspaceTarget は [workspace-id] 引数があればそれを、無ければ既定のワークスペースを返す。
func workspaceTarget(cmd *cobra.Command, args []string) (*gtm.Client, string, error) {
	ctx := cmd.Context()
	client, err := newClient(ctx)
	if err != nil {
		return nil, "", err
	}
	if len(args) == 1 {
		cp, err := client.ContainerPath(ctx)
		if err != nil {
			return nil, "", err
		}
		return client, cp + "/workspaces/" + config.NormalizeID(args[0]), nil
	}
	ws, err := client.WorkspacePath(ctx)
	return client, ws, err
}

func newWorkspaceInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info [workspace-id]",
		Short: "Fetch workspace details",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ws, err := workspaceTarget(cmd, args)
			if err != nil {
				return err
			}
			w, err := client.GetWorkspace(cmd.Context(), ws)
			if err != nil {
				return err
			}
			return renderJSON(w)
		},
	}
}

func newWorkspaceCreateCmd() *cobra.Command {
	var name, description string
	var use bool
	cmd := &cobra.Command{
		Use:   "create --name <name>",
		Short: "Create a workspace (starts from the latest version)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			c, err := client.Container(ctx)
			if err != nil {
				return err
			}
			w, err := client.CreateWorkspace(ctx, c.Path, name, description)
			if err != nil {
				return err
			}
			if use {
				if err := saveTarget(c.AccountId, c.ContainerId, c.PublicId, w.WorkspaceId); err != nil {
					return err
				}
			}
			return renderJSON(w)
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "Workspace name (required)")
	f.StringVar(&description, "description", "", "Workspace description")
	f.BoolVar(&use, "use", false, "Also save it as the default workspace")
	return cmd
}

func newWorkspaceUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <workspace-id>",
		Short: "Save the default workspace (together with its container)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			c, err := client.Container(ctx)
			if err != nil {
				return err
			}
			w, err := client.GetWorkspace(ctx, c.Path+"/workspaces/"+config.NormalizeID(args[0]))
			if err != nil {
				return err
			}
			if err := saveTarget(c.AccountId, c.ContainerId, c.PublicId, w.WorkspaceId); err != nil {
				return err
			}
			fmt.Printf("✓ Default workspace: %s %q in %s\n", w.WorkspaceId, w.Name, c.PublicId)
			return nil
		},
	}
}

func saveTarget(account, container, publicID, workspace string) error {
	creds, err := config.LoadCredentials()
	if err != nil {
		return err
	}
	creds.AccountID, creds.ContainerID, creds.PublicID, creds.WorkspaceID = account, container, publicID, workspace
	return config.SaveCredentials(creds)
}

func newWorkspaceStatusCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "status [workspace-id]",
		Short: "Show the workspace's unsaved changes and merge conflicts",
		Long: "Lists the entities changed in the workspace compared with the latest version (changeStatus:\n" +
			"added / deleted / updated) and any merge conflicts. Check this before `version create`.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(format); err != nil {
				return err
			}
			client, ws, err := workspaceTarget(cmd, args)
			if err != nil {
				return err
			}
			st, err := client.WorkspaceStatus(cmd.Context(), ws)
			if err != nil {
				return err
			}
			if format == "table" {
				var rows [][]string
				for _, e := range st.WorkspaceChange {
					kind, id, name := describeEntity(e)
					rows = append(rows, []string{e.ChangeStatus, kind, id, name})
				}
				printTable([]string{"CHANGE", "KIND", "ID", "NAME"}, rows)
				if n := len(st.MergeConflict); n > 0 {
					fmt.Printf("%d merge conflict(s) — resolve each with `workspace resolve-conflict` before `version create`\n", n)
				}
				return nil
			}
			return renderJSON(st)
		},
	}
	addFormatFlag(cmd, &format)
	return cmd
}

func newWorkspaceSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync [workspace-id]",
		Short: "Bring the latest version's changes into the workspace",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ws, err := workspaceTarget(cmd, args)
			if err != nil {
				return err
			}
			r, err := client.SyncWorkspace(cmd.Context(), ws)
			if err != nil {
				return err
			}
			return renderJSON(r)
		},
	}
}

func newWorkspaceDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <workspace-id> --yes",
		Short: "Delete a workspace and every unsaved change in it (requires --yes)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireYes(yes, "deleting a workspace discards all of its unsaved changes"); err != nil {
				return err
			}
			client, ws, err := workspaceTarget(cmd, args)
			if err != nil {
				return err
			}
			if err := client.DeleteWorkspace(cmd.Context(), ws); err != nil {
				return err
			}
			creds, _ := config.LoadCredentials()
			if creds != nil && creds.WorkspaceID == config.NormalizeID(args[0]) {
				creds.WorkspaceID = ""
				_ = config.SaveCredentials(creds)
			}
			return renderJSON(map[string]any{"deleted": ws})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm the deletion")
	return cmd
}

func newWorkspaceUpdateCmd() *cobra.Command {
	var name, description string
	cmd := &cobra.Command{
		Use:   "update [workspace-id] [--name ...] [--description ...]",
		Short: "Rename a workspace or change its description",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			var np, dp *string
			if f.Changed("name") {
				np = &name
			}
			if f.Changed("description") {
				dp = &description
			}
			if np == nil && dp == nil {
				return fmt.Errorf("nothing to update: pass --name or --description")
			}
			client, ws, err := workspaceTarget(cmd, args)
			if err != nil {
				return err
			}
			w, err := client.UpdateWorkspace(cmd.Context(), ws, np, dp)
			if err != nil {
				return err
			}
			return renderJSON(w)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New name")
	cmd.Flags().StringVar(&description, "description", "", "New description")
	return cmd
}

func newWorkspacePreviewCmd() *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "quick-preview [workspace-id]",
		Short: "Compile the workspace without creating a version (exit 1 on compiler errors)",
		Long: "Compiles the workspace as if creating a version, without creating one or consuming the\n" +
			"workspace. Run it after editing and before `version create` to catch compiler errors.\n" +
			"Prints compilerError, compilerErrors, syncStatus and diagnostics; --full adds the compiled\n" +
			"containerVersion. Exits 1 when the workspace does not compile.\n" +
			"The API often reports compilerError without any compilerErrors detail, so on failure the CLI\n" +
			"adds diagnostics: references to {{variables}} that do not exist (the usual cause — the API\n" +
			"accepts them on write but cannot compile them; unknown trigger IDs are rejected on write).\n" +
			"This does not open GTM's Preview mode (Tag Assistant); that is only available in the UI.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ws, err := workspaceTarget(cmd, args)
			if err != nil {
				return err
			}
			r, err := client.QuickPreview(cmd.Context(), ws)
			if err != nil {
				return err
			}
			// API は compilerErrors を空で返すことが多い。原因の候補をこちらで突き合わせて添える。
			diagnostics := []gtm.Finding{}
			if r.CompilerError && r.ContainerVersion != nil {
				diagnostics = append(diagnostics, gtm.BrokenVariableReferences(r.ContainerVersion)...)
			}
			if !full {
				r.ContainerVersion = nil
			}
			if r.CompilerErrors == nil {
				r.CompilerErrors = []*tm.CompilerErrorLite{}
			}
			r.ForceSendFields = append(r.ForceSendFields, "CompilerError", "CompilerErrors")
			out, err := toMap(r)
			if err != nil {
				return err
			}
			out["diagnostics"] = diagnostics
			if err := renderJSON(out); err != nil {
				return err
			}
			if r.CompilerError {
				return fmt.Errorf("the workspace does not compile (see compilerErrors and diagnostics)")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Include the compiled containerVersion")
	return cmd
}

func newWorkspaceResolveCmd() *cobra.Command {
	var kind, id, take, file string
	cmd := &cobra.Command{
		Use:   "resolve-conflict --kind <kind> --id <id> (--take workspace|base | --file <entity-json|->)",
		Short: "Resolve a merge conflict after `workspace sync`",
		Long: "A sync can leave merge conflicts: the same entity changed both in this workspace and in a\n" +
			"version created meanwhile (see `workspace status`). Resolve each one before `version create`.\n" +
			"--take workspace keeps this workspace's side, --take base takes the newer version's side.\n" +
			"--file posts your own merged entity instead, in the Entity shape: {\"tag\": {...}} (or trigger,\n" +
			"variable, folder, client, transformation, zone, customTemplate, builtInVariable, gtagConfig);\n" +
			"to resolve as a deletion add \"changeStatus\": \"deleted\".\n" +
			"--kind is the kind shown by `workspace status --format table` (tag, trigger, variable, ...).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if kind == "" || id == "" {
				return fmt.Errorf("--kind and --id are required (see `workspace status --format table`)")
			}
			if (take == "") == (file == "") {
				return fmt.Errorf("pass exactly one of --take workspace|base or --file")
			}
			if take != "" && take != "workspace" && take != "base" {
				return fmt.Errorf("--take must be workspace or base")
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
			st, err := client.WorkspaceStatus(ctx, ws)
			if err != nil {
				return err
			}
			var conflict *tm.MergeConflict
			for _, mc := range st.MergeConflict {
				for _, e := range []*tm.Entity{mc.EntityInWorkspace, mc.EntityInBaseVersion} {
					if e == nil {
						continue
					}
					if k, eid, _ := describeEntity(e); k == kind && eid == config.NormalizeID(id) {
						conflict = mc
					}
				}
			}
			if conflict == nil {
				return fmt.Errorf("no merge conflict for %s %s in this workspace (see `workspace status`)", kind, id)
			}
			var resolved *tm.Entity
			switch {
			case take == "workspace":
				resolved = conflict.EntityInWorkspace
			case take == "base":
				resolved = conflict.EntityInBaseVersion
			default:
				body, err := readBody(file)
				if err != nil {
					return err
				}
				resolved = &tm.Entity{}
				if err := decodeStrict(body, resolved); err != nil {
					return err
				}
			}
			if resolved == nil {
				return fmt.Errorf("that side of the conflict has no entity; pass --file with \"changeStatus\": \"deleted\" to resolve as a deletion")
			}
			if err := client.ResolveConflict(ctx, ws, resolved, entityFingerprint(conflict.EntityInWorkspace)); err != nil {
				return err
			}
			return renderJSON(map[string]any{"resolved": kind + " " + config.NormalizeID(id), "took": firstNonEmpty(take, "file")})
		},
	}
	f := cmd.Flags()
	f.StringVar(&kind, "kind", "", "Entity kind: tag, trigger, variable, folder, template, client, transformation, zone, builtInVariable, gtagConfig")
	f.StringVar(&id, "id", "", "Entity ID (for builtInVariable: its type)")
	f.StringVar(&take, "take", "", "Keep one side: workspace or base")
	f.StringVar(&file, "file", "", "Merged entity JSON ({\"tag\": {...}}); - for stdin")
	return cmd
}

// entityFingerprint は Entity の中身（tag / trigger / ...）の fingerprint を返す。
func entityFingerprint(e *tm.Entity) string {
	if e == nil {
		return ""
	}
	m, err := toMap(e)
	if err != nil {
		return ""
	}
	for _, v := range m {
		if inner, ok := v.(map[string]any); ok {
			if fp := str(inner, "fingerprint"); fp != "" {
				return fp
			}
		}
	}
	return ""
}

func newWorkspaceBulkUpdateCmd() *cobra.Command {
	var file string
	var yes bool
	cmd := &cobra.Command{
		Use:   "bulk-update --file <json|-> [--yes]",
		Short: "Apply many entity changes in one call; new entities can reference each other by new_N IDs",
		Long: "Applies a list of changes to the workspace in one API call. The body is\n" +
			"{\"changes\": [Entity, ...]} (a bare array is accepted too). Each Entity holds one entity\n" +
			"(tag, trigger, variable, folder, client, transformation, zone, customTemplate, gtagConfig)\n" +
			"and a changeStatus: added, updated or deleted.\n" +
			"New entities need temporary IDs of the form new_1, new_2, ... and may refer to each other\n" +
			"with them — e.g. a tag whose firingTriggerId is [\"new_2\"] and parentFolderId is \"new_1\".\n" +
			"This is how to copy a group of entities from another container's export with their\n" +
			"references intact: give each copied entity a new_N ID and rewrite the references to match.\n" +
			"Changes containing changeStatus \"deleted\" require --yes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file is required (use - for stdin)")
			}
			body, err := readBody(file)
			if err != nil {
				return err
			}
			pc, err := parseProposedChange(body)
			if err != nil {
				return err
			}
			deletions := 0
			for _, e := range pc.Changes {
				if e.ChangeStatus == "" {
					return fmt.Errorf("every change needs a changeStatus (added, updated or deleted)")
				}
				if e.ChangeStatus == "deleted" {
					deletions++
				}
			}
			if deletions > 0 {
				if err := requireYes(yes, fmt.Sprintf("the bulk update deletes %d entit(ies)", deletions)); err != nil {
					return err
				}
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
			r, err := client.BulkUpdate(ctx, ws, pc)
			if err != nil {
				return err
			}
			return renderJSON(r)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Changes JSON ({\"changes\": [...]}); - for stdin")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm when the changes include deletions")
	return cmd
}

// parseProposedChange は {"changes": [...]} または配列そのものを読む。
func parseProposedChange(body []byte) (*tm.ProposedChange, error) {
	pc := &tm.ProposedChange{}
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "[") {
		if err := decodeStrict([]byte(trimmed), &pc.Changes); err != nil {
			return nil, err
		}
	} else if err := decodeStrict(body, pc); err != nil {
		return nil, err
	}
	if len(pc.Changes) == 0 {
		return nil, fmt.Errorf("no changes in the body")
	}
	return pc, nil
}

// decodeStrict は未知のフィールドをエラーにして JSON を読む。
func decodeStrict(body []byte, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}
