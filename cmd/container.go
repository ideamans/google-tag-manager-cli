package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	tm "google.golang.org/api/tagmanager/v2"

	"github.com/ideamans/google-tag-manager-cli/internal/config"
)

func newContainerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "container",
		Aliases: []string{"ct"},
		Short:   "List, look up, inspect and choose the default container; print its install snippet",
	}
	cmd.AddCommand(newContainerListCmd(), newContainerInfoCmd(), newContainerUseCmd(), newContainerSnippetCmd())
	return cmd
}

func newContainerListCmd() *cobra.Command {
	var format string
	var all bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List containers in the account (--all: in every accessible account)",
		Long: "Lists containers under --account (or the account of the default container).\n" +
			"With --all, walks every accessible account — use this to find which account holds a GTM-XXXX.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(format); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			var cs []*tm.Container
			if all {
				cs, err = client.ListAllContainers(ctx)
			} else {
				var ap string
				if ap, err = client.AccountPath(ctx, overrides.Account); err != nil {
					return err
				}
				cs, err = client.ListContainers(ctx, ap)
			}
			if err != nil {
				return err
			}
			if format == "table" {
				var rows [][]string
				for _, c := range cs {
					rows = append(rows, []string{c.PublicId, c.AccountId, c.ContainerId, c.Name, strings.Join(c.UsageContext, ","), strings.Join(c.DomainName, ",")})
				}
				printTable([]string{"PUBLIC_ID", "ACCOUNT", "CONTAINER", "NAME", "USAGE", "DOMAINS"}, rows)
				return nil
			}
			return renderJSON(cs)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "List containers in every accessible account")
	addFormatFlag(cmd, &format)
	return cmd
}

func newContainerInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info [container]",
		Short: "Fetch container details; the argument may be GTM-XXXX, a path, or a numeric ID",
		Long: "Resolves the container (GTM-XXXX is looked up with containers.lookup, so no account ID is needed)\n" +
			"and prints it. Without an argument, uses --container or the saved default.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				acc, ct := config.ParseContainerRef(args[0])
				client.SetTarget(firstNonEmpty(acc, config.NormalizeID(overrides.Account)), ct, "")
			}
			c, err := client.Container(ctx)
			if err != nil {
				return err
			}
			return renderJSON(c)
		},
	}
}

func newContainerUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <container>",
		Short: "Save the default container (GTM-XXXX, accounts/<a>/containers/<c>, or numeric ID with --account)",
		Long: "Resolves the container and saves its account ID, container ID and public ID to credentials.json.\n" +
			"The saved workspace is cleared because it belonged to the previous container;\n" +
			"pass --workspace to save one at the same time.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			acc, ct := config.ParseContainerRef(args[0])
			client.SetTarget(firstNonEmpty(acc, config.NormalizeID(overrides.Account)), ct, config.NormalizeID(overrides.Workspace))
			c, err := client.Container(ctx)
			if err != nil {
				return err
			}
			creds, err := config.LoadCredentials()
			if err != nil {
				return err
			}
			creds.AccountID, creds.ContainerID, creds.PublicID = c.AccountId, c.ContainerId, c.PublicId
			creds.WorkspaceID = config.NormalizeID(overrides.Workspace)
			if err := config.SaveCredentials(creds); err != nil {
				return err
			}
			fmt.Printf("✓ Default container: %s %q (accounts/%s/containers/%s)\n", c.PublicId, c.Name, c.AccountId, c.ContainerId)
			if creds.WorkspaceID != "" {
				fmt.Printf("  Default workspace: %s\n", creds.WorkspaceID)
			}
			return nil
		},
	}
}

func newContainerSnippetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "snippet [container]",
		Short: "Print the tagging snippet to install the container on a site",
		Long: "Prints the container's install code as the API returns it: snippet (the HTML to put in the\n" +
			"page) and, for some container types, containerConfig. To install a staging environment\n" +
			"instead, see `environment list` (its authorizationCode and environmentId go into the snippet).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				acc, ct := config.ParseContainerRef(args[0])
				client.SetTarget(firstNonEmpty(acc, config.NormalizeID(overrides.Account)), ct, "")
			}
			cp, err := client.ContainerPath(ctx)
			if err != nil {
				return err
			}
			v, err := client.Snippet(ctx, cp)
			if err != nil {
				return err
			}
			return renderJSON(v)
		},
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
