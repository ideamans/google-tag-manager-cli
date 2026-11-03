package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	tm "google.golang.org/api/tagmanager/v2"

	"github.com/ideamans/google-tag-manager-cli/internal/gtm"
)

func newAuditCmd() *cobra.Command {
	var from, file, format string
	cmd := &cobra.Command{
		Use:   "audit [--from live|latest|workspace|<version-id>] [--file export.json]",
		Short: "Inventory a container: tag types, paused/orphaned tags, unused triggers/variables, custom HTML",
		Long: "Reports facts that can be checked mechanically, as cleanup candidates. It never changes anything.\n" +
			"Finding codes: PAUSED_TAG, TAG_WITHOUT_FIRING_TRIGGER, UNUSED_TRIGGER, UNUSED_VARIABLE,\n" +
			"BROKEN_VARIABLE_REFERENCE,\n" +
			"UNIVERSAL_ANALYTICS, DUPLICATE_CUSTOM_HTML, DOCUMENT_WRITE, DUPLICATE_TAG_NAME.\n" +
			"--file audits an export JSON offline (no API call, no credentials needed).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(format); err != nil {
				return err
			}
			var v *tm.ContainerVersion
			source := from
			if file != "" {
				body, err := readBody(file)
				if err != nil {
					return err
				}
				var exp gtm.Export
				if err := json.Unmarshal(body, &exp); err != nil {
					return fmt.Errorf("%s is not a GTM export JSON: %w", file, err)
				}
				if exp.ContainerVersion == nil {
					return fmt.Errorf("%s has no containerVersion", file)
				}
				v, source = exp.ContainerVersion, "file:"+file
			} else {
				ctx := cmd.Context()
				client, err := newClient(ctx)
				if err != nil {
					return err
				}
				if v, err = client.Snapshot(ctx, from); err != nil {
					return err
				}
			}
			a := gtm.RunAudit(v, source)
			if format == "table" {
				printAudit(a)
				return nil
			}
			return renderJSON(a)
		},
	}
	cmd.Flags().StringVar(&from, "from", "live", fromHelp)
	cmd.Flags().StringVar(&file, "file", "", "Audit an export JSON file instead of calling the API (- for stdin)")
	addFormatFlag(cmd, &format)
	return cmd
}

func printAudit(a *gtm.Audit) {
	fmt.Printf("Container %s (%s)\n", a.Container, a.Source)
	fmt.Printf("  tags %d / triggers %d / variables %d / folders %d / built-in variables %d / templates %d\n\n",
		a.Counts["tags"], a.Counts["triggers"], a.Counts["variables"], a.Counts["folders"], a.Counts["builtInVariables"], a.Counts["customTemplates"])

	var rows [][]string
	for _, t := range a.TagTypes {
		rows = append(rows, []string{t.Type, t.Label, strconv.Itoa(t.Count), strconv.Itoa(t.Paused)})
	}
	printTable([]string{"TAG_TYPE", "LABEL", "COUNT", "PAUSED"}, rows)
	fmt.Println()

	rows = nil
	for _, h := range a.CustomHTML {
		flags := ""
		if h.Paused {
			flags += "paused "
		}
		if h.AllPages {
			flags += "all-pages "
		}
		if h.DocumentWrite {
			flags += "document.write"
		}
		rows = append(rows, []string{h.TagID, h.Name, strconv.Itoa(h.Bytes), flags})
	}
	if len(rows) > 0 {
		printTable([]string{"HTML_TAG", "NAME", "BYTES", "FLAGS"}, rows)
		fmt.Println()
	}

	rows = nil
	for _, f := range a.Findings {
		rows = append(rows, []string{f.Severity, f.Code, f.Kind, f.ID, f.Name})
	}
	printTable([]string{"SEVERITY", "CODE", "KIND", "ID", "NAME"}, rows)
	_ = os.Stdout.Sync()
}
