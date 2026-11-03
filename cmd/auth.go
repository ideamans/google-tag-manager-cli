package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ideamans/google-tag-manager-cli/internal/config"
	"github.com/ideamans/google-tag-manager-cli/internal/gtm"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication credentials",
	}
	cmd.AddCommand(newAuthLoginCmd(), newAuthStatusCmd(), newAuthListAccountsCmd())
	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	var noBrowser, readOnly bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in with OAuth2 and save the refresh token",
		Long: "Opens Google's consent screen in the browser and saves the resulting refresh token to credentials.json.\n" +
			"By default it requests read, workspace edit, version create, publish and workspace delete scopes.\n" +
			"With --readonly it requests only tagmanager.readonly, so every write command fails with 403.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			if r.ClientID == "" || r.ClientSecret == "" {
				path, _ := config.OAuthClientPath()
				return fmt.Errorf("OAuth client is not configured. Provide %s or specify --client-id/--client-secret", path)
			}
			scopes := gtm.Scopes
			if readOnly {
				scopes = gtm.ReadOnlyScopes
			}
			result, err := gtm.InteractiveLogin(cmd.Context(), r.ClientID, r.ClientSecret, scopes, !noBrowser)
			if err != nil {
				return err
			}
			creds, err := config.LoadCredentials()
			if err != nil {
				return err
			}
			creds.RefreshToken = result.RefreshToken
			creds.Scopes = scopes
			if err := config.SaveCredentials(creds); err != nil {
				return err
			}
			path, _ := config.CredentialsPath()
			fmt.Printf("✓ Authentication succeeded. Saved the refresh token: %s\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Do not open the browser automatically; only print the URL")
	cmd.Flags().BoolVar(&readOnly, "readonly", false, "Request only the read-only scope (for inspecting containers you must not change)")
	return cmd
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the resolved credentials and default account/container/workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			creds, _ := config.LoadCredentials()
			dir, _ := config.Dir()
			fmt.Printf("Config directory: %s\n", dir)
			fmt.Printf("  OAuth client ID : %s\n", present(r.ClientID, mask(r.ClientID)))
			fmt.Printf("  Client secret   : %s\n", present(r.ClientSecret, "set"))
			fmt.Printf("  refresh token   : %s\n", present(r.RefreshToken, "set"))
			fmt.Printf("  scopes          : %s\n", present(strings.Join(shortScopes(r.Scopes), " "), strings.Join(shortScopes(r.Scopes), " ")))
			fmt.Printf("  account         : %s\n", present(r.AccountID, r.AccountID))
			container := r.Container
			if creds != nil && creds.PublicID != "" && r.Container == creds.ContainerID {
				container += " (" + creds.PublicID + ")"
			}
			fmt.Printf("  container       : %s\n", present(r.Container, container))
			fmt.Printf("  workspace       : %s\n", present(r.WorkspaceID, r.WorkspaceID))
			fmt.Println()
			if err := r.RequireForAPI(); err != nil {
				fmt.Println("Status: " + err.Error())
			} else {
				fmt.Println("Status: ✓ All credentials required for API calls are present")
				if r.Container == "" {
					fmt.Println("Hint: pick a container with --container GTM-XXXX or `container use GTM-XXXX`")
				}
			}
			return nil
		},
	}
}

func newAuthListAccountsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-accounts",
		Short: "List accessible GTM accounts (authentication connectivity check)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := newClient(ctx)
			if err != nil {
				return err
			}
			accounts, err := client.ListAccounts(ctx)
			if err != nil {
				return err
			}
			if len(accounts) == 0 {
				fmt.Println("No accessible GTM accounts were found.")
				return nil
			}
			fmt.Printf("✓ API connectivity succeeded. %d accessible account(s):\n", len(accounts))
			for _, a := range accounts {
				fmt.Printf("  - %s  %s\n", a.AccountId, a.Name)
			}
			return nil
		},
	}
}

func shortScopes(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	for _, s := range scopes {
		out = append(out, strings.TrimPrefix(s, "https://www.googleapis.com/auth/"))
	}
	return out
}

func present(value, shown string) string {
	if value == "" {
		return "(not set)"
	}
	return shown
}

func mask(s string) string {
	if len(s) <= 8 {
		return "********"
	}
	return s[:4] + "…" + s[len(s)-4:]
}
