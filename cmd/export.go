package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ideamans/google-tag-manager-cli/internal/gtm"
)

const fromHelp = "What to read: live (published), latest (newest version), workspace (current edits), or a version ID"

func newExportCmd() *cobra.Command {
	var from, output string
	cmd := &cobra.Command{
		Use:   "export [--from live|latest|workspace|<version-id>] [-o file]",
		Short: "Export the whole container as JSON in the GTM UI import format",
		Long: "Writes {exportFormatVersion, exportTime, containerVersion} — the same JSON as GTM's\n" +
			"Admin > Export Container, so it can be imported in the GTM UI (merge or overwrite).\n" +
			"--from workspace assembles it from the workspace's lists (several API calls).",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			v, err := client.Snapshot(ctx, from)
			if err != nil {
				return err
			}
			exp := gtm.NewExport(v)
			if output == "" || output == "-" {
				return renderJSON(exp)
			}
			f, err := os.Create(output)
			if err != nil {
				return err
			}
			defer f.Close()
			if err := writeJSON(f, exp); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "✓ Wrote %s (%d tags, %d triggers, %d variables)\n", output, len(v.Tag), len(v.Trigger), len(v.Variable))
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "live", fromHelp)
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write to this file instead of stdout")
	return cmd
}
