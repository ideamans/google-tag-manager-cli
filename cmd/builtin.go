package cmd

import (
	"github.com/spf13/cobra"
)

func newBuiltInVariableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "builtin-variable",
		Aliases: []string{"builtin"},
		Short:   "List, enable, disable and revert built-in variables (Page URL, Click Element, ...)",
		Long: "Built-in variables are identified by their API type, not their UI name:\n" +
			"pageUrl, pagePath, pageHostname, referrer, event, clickElement, clickClasses, clickId,\n" +
			"clickTarget, clickUrl, clickText, formElement, formId, ..., scrollDepthThreshold, videoTitle, etc.",
	}
	cmd.AddCommand(newBuiltInListCmd(), newBuiltInSetCmd(true), newBuiltInSetCmd(false), newBuiltInRevertCmd())
	return cmd
}

func newBuiltInListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List built-in variables enabled in the workspace",
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
			vs, err := client.ListBuiltInVariables(ctx, ws)
			if err != nil {
				return err
			}
			if format == "table" {
				var rows [][]string
				for _, v := range vs {
					rows = append(rows, []string{v.Type, v.Name})
				}
				printTable([]string{"TYPE", "NAME"}, rows)
				return nil
			}
			return renderJSON(vs)
		},
	}
	addFormatFlag(cmd, &format)
	return cmd
}

func newBuiltInSetCmd(enable bool) *cobra.Command {
	use, short := "enable", "Enable built-in variables by API type"
	if !enable {
		use, short = "disable", "Disable built-in variables by API type"
	}
	return &cobra.Command{
		Use:   use + " <type>...",
		Short: short,
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
			v, err := client.SetBuiltInVariables(ctx, ws, args, enable)
			if err != nil {
				return err
			}
			return renderJSON(v)
		},
	}
}

func newBuiltInRevertCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revert <type>...",
		Short: "Discard the workspace's enable/disable of built-in variables (back to the latest version)",
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
			var out []map[string]any
			for _, t := range args {
				r, err := client.RevertBuiltInVariable(ctx, ws, t)
				if err != nil {
					return err
				}
				out = append(out, map[string]any{"type": t, "enabled": r.Enabled})
			}
			return renderJSON(out)
		},
	}
}
