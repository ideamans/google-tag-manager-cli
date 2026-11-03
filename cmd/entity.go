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

// newEntityCmd はワークスペース配下のエンティティ（tag / trigger / variable / folder /
// template / client / transformation / zone / gtag-config）に共通のサブコマンド群を組み立てる。
func newEntityCmd(k *gtm.Kind) *cobra.Command {
	verbs := "List, get, create, update, delete and revert"
	if k.Revert == nil {
		verbs = "List, get, create, update and delete"
	}
	long := fmt.Sprintf("Operates on %s in the target workspace. Changes are not live until a version is created\n"+
		"and published (`version create`, `version publish`).\n"+
		"Create/update bodies are the Tag Manager API JSON — the same shape as one element of\n"+
		"containerVersion.%s in a GTM UI export (`gtmc export`).", k.Plural, k.ExportKey)
	if k.Note != "" {
		long += "\n" + k.Note
	}
	cmd := &cobra.Command{
		Use:   k.Name,
		Short: fmt.Sprintf("%s %s in the workspace", verbs, k.Plural),
		Long:  long,
	}
	cmd.AddCommand(
		newEntityListCmd(k),
		newEntityGetCmd(k),
		newEntityCreateCmd(k),
		newEntityUpdateCmd(k),
		newEntityDeleteCmd(k),
	)
	if k.Revert != nil {
		cmd.AddCommand(newEntityRevertCmd(k))
	}
	switch k {
	case gtm.TagKind:
		cmd.AddCommand(newTagPauseCmd(true), newTagPauseCmd(false))
	case gtm.FolderKind:
		cmd.AddCommand(newFolderMoveCmd(), newFolderEntitiesCmd())
	case gtm.TemplateKind:
		cmd.AddCommand(newTemplateImportCmd())
	}
	return cmd
}

func newEntityListCmd(k *gtm.Kind) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("List %s in the workspace", k.Plural),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(format); err != nil {
				return err
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
			items, err := k.List(ctx, client, ws)
			if err != nil {
				return err
			}
			if format == "table" {
				return printEntityTable(k, items)
			}
			if items == nil {
				items = []any{}
			}
			return renderJSON(items)
		},
	}
	addFormatFlag(cmd, &format)
	return cmd
}

func printEntityTable(k *gtm.Kind, items []any) error {
	header := []string{"ID", "NAME", "TYPE", "FOLDER"}
	if k == gtm.TagKind {
		header = append(header, "PAUSED", "FIRING", "BLOCKING")
	}
	var rows [][]string
	for _, it := range items {
		m, err := toMap(it)
		if err != nil {
			return err
		}
		row := []string{str(m, k.IDField), str(m, "name"), str(m, "type"), str(m, "parentFolderId")}
		if k == gtm.TemplateKind {
			// タグ・変数の type に書く値。
			row[2] = gtm.TemplateType(str(m, "containerId"), str(m, "templateId"), galleryID(m))
		}
		if k == gtm.ClientKind {
			row[3] = str(m, "parentFolderId") + " (priority " + str(m, "priority") + ")"
		}
		if k == gtm.TagKind {
			paused := ""
			if m["paused"] == true {
				paused = "paused"
			}
			row = append(row, paused, str(m, "firingTriggerId"), str(m, "blockingTriggerId"))
		}
		rows = append(rows, row)
	}
	printTable(header, rows)
	return nil
}

func newEntityGetCmd(k *gtm.Kind) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: fmt.Sprintf("Fetch one %s as API JSON", k.Name),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			ws, err := client.WorkspacePath(ctx)
			if err != nil {
				return err
			}
			v, err := k.Get(ctx, client, k.Path(ws, config.NormalizeID(args[0])))
			if err != nil {
				return err
			}
			return renderJSON(v)
		},
	}
}

func newEntityCreateCmd(k *gtm.Kind) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "create --file <json|->",
		Short: fmt.Sprintf("Create a %s from API JSON", k.Name),
		Long: fmt.Sprintf("Creates a %s in the workspace from a JSON file (- reads stdin).\n"+
			"IDs, paths and fingerprints in the body are ignored, so an element copied from another\n"+
			"container's export can be used — but its references (trigger IDs, folder IDs, {{variable}}\n"+
			"names, cvt_ template types) must exist in this container.", k.Name),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file is required (use - for stdin)")
			}
			body, err := readBody(file)
			if err != nil {
				return err
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
			v, err := k.Create(ctx, client, ws, body)
			if err != nil {
				return err
			}
			return renderJSON(v)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "JSON body (API shape); - for stdin")
	return cmd
}

// entityPatch は update の部分変更フラグ。
type entityPatch struct {
	file, name, notes, folder string
	setName, setNotes, setDir bool
	paused                    *bool
}

func (p entityPatch) empty() bool {
	return p.file == "" && !p.setName && !p.setNotes && !p.setDir && p.paused == nil
}

func newEntityUpdateCmd(k *gtm.Kind) *cobra.Command {
	var p entityPatch
	cmd := &cobra.Command{
		Use:   updateUse(k),
		Short: updateShort(k),
		Long: fmt.Sprintf("Update is a full replacement in the API. With --file, the JSON becomes the new %[1]s\n"+
			"(fetch it with `%[1]s get`, edit, and pass it back). Without --file, the current %[1]s is fetched\n"+
			"and only the given fields are changed. Field flags are applied on top of --file too.\n"+
			"The fingerprint in the file is used for optimistic locking when present (409 if someone\n"+
			"changed it meanwhile); otherwise the current fingerprint is used.", k.Name),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			p.setName, p.setNotes, p.setDir = f.Changed("name"), f.Changed("notes"), f.Changed("folder")
			if p.empty() {
				return fmt.Errorf("nothing to update: pass %s", updateFlagsText(k))
			}
			v, err := updateEntity(cmd, k, config.NormalizeID(args[0]), p)
			if err != nil {
				return err
			}
			return renderJSON(v)
		},
	}
	f := cmd.Flags()
	f.StringVar(&p.file, "file", "", "Replacement JSON body (API shape); - for stdin")
	if k.HasName {
		f.StringVar(&p.name, "name", "", "New name")
		f.StringVar(&p.notes, "notes", "", "New notes")
	}
	if k.HasFolder {
		f.StringVar(&p.folder, "folder", "", "Move into this folder ID (empty string: out of any folder)")
	}
	return cmd
}

func updateUse(k *gtm.Kind) string {
	u := "update <id> [--file <json|->]"
	if k.HasName {
		u += " [--name ...] [--notes ...]"
	}
	if k.HasFolder {
		u += " [--folder <id>]"
	}
	return u
}

func updateShort(k *gtm.Kind) string {
	switch {
	case k.HasFolder:
		return fmt.Sprintf("Update a %s: replace it from JSON, or change only name/notes/folder", k.Name)
	case k.HasName:
		return fmt.Sprintf("Update a %s: replace it from JSON, or change only name/notes", k.Name)
	}
	return fmt.Sprintf("Update a %s: replace it from JSON", k.Name)
}

func updateFlagsText(k *gtm.Kind) string {
	switch {
	case k.HasFolder:
		return "--file, --name, --notes or --folder"
	case k.HasName:
		return "--file, --name or --notes"
	}
	return "--file"
}

// updateEntity は現在の値を取得し、ファイルまたは部分変更を当てて全置換で保存する。
func updateEntity(cmd *cobra.Command, k *gtm.Kind, id string, p entityPatch) (any, error) {
	ctx := cmd.Context()
	client, err := newClient(ctx)
	if err != nil {
		return nil, err
	}
	ws, err := client.WorkspacePath(ctx)
	if err != nil {
		return nil, err
	}
	path := k.Path(ws, id)
	cur, err := k.Get(ctx, client, path)
	if err != nil {
		return nil, err
	}
	m, err := toMap(cur)
	if err != nil {
		return nil, err
	}
	fingerprint := str(m, "fingerprint")
	if p.file != "" {
		body, err := readBody(p.file)
		if err != nil {
			return nil, err
		}
		// ファイルで全置換するので、取得した値ではなくファイルの中身を土台にする。
		m = map[string]any{}
		if err := json.Unmarshal(body, &m); err != nil {
			return nil, fmt.Errorf("invalid JSON in %s: %w", p.file, err)
		}
		if fp := str(m, "fingerprint"); fp != "" {
			fingerprint = fp
		}
	}
	if p.setName {
		m["name"] = p.name
	}
	if p.setNotes {
		m["notes"] = p.notes
	}
	if p.setDir {
		if p.folder == "" {
			delete(m, "parentFolderId")
		} else {
			m["parentFolderId"] = config.NormalizeID(p.folder)
		}
	}
	if p.paused != nil {
		m["paused"] = *p.paused
	}
	body, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return k.Update(ctx, client, path, body, fingerprint)
}

func newEntityDeleteCmd(k *gtm.Kind) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>... --yes",
		Short: fmt.Sprintf("Delete %s from the workspace (requires --yes)", k.Plural),
		Long:  deleteLong(k),
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireYes(yes, fmt.Sprintf("deleting %d %s", len(args), k.Name+"(s)")); err != nil {
				return err
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
			var deleted []string
			for _, id := range args {
				path := k.Path(ws, config.NormalizeID(id))
				if err := k.Delete(ctx, client, path); err != nil {
					_ = renderJSON(map[string]any{"deleted": deleted})
					return err
				}
				deleted = append(deleted, path)
			}
			return renderJSON(map[string]any{"deleted": deleted})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm the deletion")
	return cmd
}

func deleteLong(k *gtm.Kind) string {
	undo := fmt.Sprintf("Until a version is created this can be undone with `%s revert <id>`", k.Name)
	if k.Revert == nil {
		undo = "The API has no revert for this kind: to undo, recreate it from `export` output or delete\n" +
			"the workspace"
	}
	s := fmt.Sprintf("Deletes %s from the workspace. %s; after a version is published it is gone\n"+
		"from the site.", k.Plural, undo)
	if k == gtm.TagKind {
		s += "\nConsider `tag pause` first to observe the effect before deleting."
	}
	return s
}

func newEntityRevertCmd(k *gtm.Kind) *cobra.Command {
	return &cobra.Command{
		Use:   "revert <id>...",
		Short: fmt.Sprintf("Discard the workspace's changes to %s (back to the latest version)", k.Plural),
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			ws, err := client.WorkspacePath(ctx)
			if err != nil {
				return err
			}
			var out []any
			for _, id := range args {
				v, err := k.Revert(ctx, client, k.Path(ws, config.NormalizeID(id)))
				if err != nil {
					return err
				}
				out = append(out, v)
			}
			return renderJSON(out)
		},
	}
}

func newTagPauseCmd(pause bool) *cobra.Command {
	use, short := "pause", "Pause tags (they stay in the container but never fire)"
	if !pause {
		use, short = "unpause", "Resume paused tags"
	}
	return &cobra.Command{
		Use:   use + " <tag-id>...",
		Short: short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var out []any
			for _, id := range args {
				v, err := updateEntity(cmd, gtm.TagKind, config.NormalizeID(id), entityPatch{paused: &pause})
				if err != nil {
					return err
				}
				out = append(out, v)
			}
			return renderJSON(out)
		},
	}
}

func newFolderMoveCmd() *cobra.Command {
	var tags, triggers, variables []string
	cmd := &cobra.Command{
		Use:   "move <folder-id> [--tags 1,2] [--triggers 3] [--variables 4]",
		Short: "Move tags, triggers and variables into a folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(tags)+len(triggers)+len(variables) == 0 {
				return fmt.Errorf("nothing to move: pass --tags, --triggers or --variables")
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
			folder := gtm.FolderKind.Path(ws, config.NormalizeID(args[0]))
			if err := client.MoveToFolder(ctx, folder, tags, triggers, variables); err != nil {
				return err
			}
			return renderJSON(map[string]any{"folder": folder, "tags": nonNil(tags), "triggers": nonNil(triggers), "variables": nonNil(variables)})
		},
	}
	f := cmd.Flags()
	f.StringSliceVar(&tags, "tags", nil, "Tag IDs (comma separated)")
	f.StringSliceVar(&triggers, "triggers", nil, "Trigger IDs (comma separated)")
	f.StringSliceVar(&variables, "variables", nil, "Variable IDs (comma separated)")
	return cmd
}

func newFolderEntitiesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "entities <folder-id>",
		Short: "List the tags, triggers and variables in a folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			ws, err := client.WorkspacePath(ctx)
			if err != nil {
				return err
			}
			v, err := client.FolderEntities(ctx, gtm.FolderKind.Path(ws, config.NormalizeID(args[0])))
			if err != nil {
				return err
			}
			return renderJSON(v)
		},
	}
}

func newTemplateImportCmd() *cobra.Command {
	var owner, repo, sha string
	var ack bool
	cmd := &cobra.Command{
		Use:   "import --owner <github-owner> --repository <repo> --acknowledge-permissions [--sha <commit>]",
		Short: "Import a template from the Community Template Gallery",
		Long: "Imports a Community Template Gallery template into the workspace. The gallery is backed by\n" +
			"GitHub: --owner and --repository are the template's GitHub owner and repository (shown on its\n" +
			"gallery page), --sha pins a commit (default: the latest).\n" +
			"A template declares permissions (URLs it may load, cookies, globals it may touch). The API\n" +
			"refuses the import unless they are acknowledged, so --acknowledge-permissions is required —\n" +
			"review the permissions on the gallery page first. After importing, `template get <id>` shows\n" +
			"them in templateData; the tag/variable type to use is cvt_<containerId>_<templateId>.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if owner == "" || repo == "" {
				return fmt.Errorf("--owner and --repository are required")
			}
			if !ack {
				return fmt.Errorf("--acknowledge-permissions is required: the template's permissions (network, cookies, globals) are granted on import; review them on its gallery page first")
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
			v, err := client.ImportGalleryTemplate(ctx, ws, owner, repo, sha)
			if err != nil {
				return err
			}
			return renderJSON(v)
		},
	}
	f := cmd.Flags()
	f.StringVar(&owner, "owner", "", "GitHub owner of the gallery template")
	f.StringVar(&repo, "repository", "", "GitHub repository of the gallery template")
	f.StringVar(&sha, "sha", "", "Commit SHA to import (default: latest)")
	f.BoolVar(&ack, "acknowledge-permissions", false, "Accept the permissions the template declares (required by the API)")
	return cmd
}

// describeEntity はワークスペースの変更1件から種類・ID・名前を取り出す。
func describeEntity(e *tm.Entity) (kind, id, name string) {
	switch {
	case e.Tag != nil:
		return "tag", e.Tag.TagId, e.Tag.Name
	case e.Trigger != nil:
		return "trigger", e.Trigger.TriggerId, e.Trigger.Name
	case e.Variable != nil:
		return "variable", e.Variable.VariableId, e.Variable.Name
	case e.Folder != nil:
		return "folder", e.Folder.FolderId, e.Folder.Name
	case e.CustomTemplate != nil:
		return "template", e.CustomTemplate.TemplateId, e.CustomTemplate.Name
	case e.BuiltInVariable != nil:
		return "builtInVariable", e.BuiltInVariable.Type, e.BuiltInVariable.Name
	case e.Client != nil:
		return "client", e.Client.ClientId, e.Client.Name
	case e.Zone != nil:
		return "zone", e.Zone.ZoneId, e.Zone.Name
	case e.Transformation != nil:
		return "transformation", e.Transformation.TransformationId, e.Transformation.Name
	case e.GtagConfig != nil:
		return "gtagConfig", e.GtagConfig.GtagConfigId, e.GtagConfig.Type
	}
	return "other", "", strings.TrimSpace(fmt.Sprint(e.ChangeStatus))
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func galleryID(m map[string]any) string {
	if g, ok := m["galleryReference"].(map[string]any); ok {
		return str(g, "galleryTemplateId")
	}
	return ""
}
