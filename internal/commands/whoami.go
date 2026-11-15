package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/config"
)

func newWhoamiCommand(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the Organization, Scopes and Environment of your credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, profile, err := f.APIClient()
			if err != nil {
				return err
			}
			introspection, err := client.Introspect(cmd.Context())
			if err != nil {
				return err
			}
			organization := "none"
			if introspection.OrgID != nil {
				organization = *introspection.OrgID
			}
			fmt.Fprintf(f.Stdout, "Organization  %s\nScopes        %s\nEnvironment   %s\n",
				organization, strings.Join(introspection.Scopes, ", "), environmentOf(introspection.Mode, profile))
			return nil
		},
	}
}

// environmentOf trusts the token over the profile: TMH_CLIENT_ID credentials
// may belong to either Environment.
func environmentOf(mode *string, profile config.Profile) string {
	if mode == nil {
		return profile.Environment
	}
	switch *mode {
	case "test":
		return config.Sandbox.Environment
	case "live":
		return config.Live.Environment
	}
	return *mode
}
