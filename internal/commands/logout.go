package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
)

func newLogoutCommand(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove a profile's stored credentials",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			profile, err := f.Profile()
			if err != nil {
				return err
			}
			removed, err := f.Store().Delete(profile.Name)
			if err != nil {
				return err
			}
			if !removed {
				fmt.Fprintf(f.Stdout, "No credentials stored for the %s profile.\n", profile.Name)
				return nil
			}
			fmt.Fprintf(f.Stdout, "Logged out of the %s profile.\n", profile.Name)
			return nil
		},
	}
}
