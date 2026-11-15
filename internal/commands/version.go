package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/version"
)

func versionLine(build version.Info) string {
	return fmt.Sprintf("tmh version %s (commit %s, built %s)\n", build.Version, build.Commit, build.Date)
}

func newVersionCommand(build version.Info) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version, commit and build date of tmh",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), versionLine(build))
			return err
		},
	}
}
