package cmd

import (
	"github.com/spf13/cobra"
)

func newAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "account",
		Aliases: []string{"acc"},
		Short:   "List and inspect GTM accounts",
	}
	cmd.AddCommand(newAccountListCmd(), newAccountInfoCmd())
	return cmd
}

func newAccountListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List accessible GTM accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(format); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			accounts, err := client.ListAccounts(ctx)
			if err != nil {
				return err
			}
			if format == "table" {
				var rows [][]string
				for _, a := range accounts {
					rows = append(rows, []string{a.AccountId, a.Name})
				}
				printTable([]string{"ACCOUNT", "NAME"}, rows)
				return nil
			}
			return renderJSON(accounts)
		},
	}
	addFormatFlag(cmd, &format)
	return cmd
}

func newAccountInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info [account-id]",
		Short: "Fetch account details (default: the account of the current container)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			id := ""
			if len(args) == 1 {
				id = args[0]
			}
			path, err := client.AccountPath(ctx, id)
			if err != nil {
				return err
			}
			a, err := client.GetAccount(ctx, path)
			if err != nil {
				return err
			}
			return renderJSON(a)
		},
	}
}
