package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/config"
)

func newLoginCommand(f *cmdutil.Factory) *cobra.Command {
	var clientID string
	command := &cobra.Command{
		Use:   "login",
		Short: "Store an API client's credentials for a profile",
		Long: "Asks for an API client's id and secret, checks them with TeachMeHIPAA and stores\n" +
			"them for the profile: the secret in the OS keyring, or in a private config file\n" +
			"where no keyring exists.\n\n" +
			"Without a terminal, pass --client-id and pipe the secret on stdin:\n\n" +
			"  printf %s \"$SECRET\" | tmh login --client-id cli_…",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			profile, err := f.Profile()
			if err != nil {
				return err
			}
			credentials, err := readCredentials(f, clientID)
			if err != nil {
				return err
			}
			organizationID, err := f.NewAPIClient(profile, credentials).OrganizationID(cmd.Context())
			if err != nil {
				return err
			}
			credentials.OrganizationID = organizationID
			store := f.Store()
			inFile, err := store.Save(profile.Name, credentials)
			if err != nil {
				return err
			}
			if inFile {
				fmt.Fprintf(f.Stderr, "OS keyring unavailable; stored the client secret in %s, readable only by you.\n", store.Path())
			}
			fmt.Fprintf(f.Stdout, "Logged in to %s as Organization %s.\n", profile.Environment, organizationID)
			return nil
		},
	}
	command.Flags().StringVar(&clientID, "client-id", "", "the API client's id (cli_…); the secret is then read from stdin")
	return command
}

func readCredentials(f *cmdutil.Factory, clientID string) (config.Credentials, error) {
	if clientID == "" {
		if !f.StdinIsTTY {
			return config.Credentials{}, &cmdutil.UsageError{
				Message: "no terminal to prompt on: pass --client-id and pipe the client secret on stdin",
			}
		}
		fmt.Fprint(f.Stderr, "Client ID: ")
		line, err := f.ReadLine()
		if err != nil {
			return config.Credentials{}, err
		}
		clientID = line
	}
	var secret string
	var err error
	if f.StdinIsTTY {
		fmt.Fprint(f.Stderr, "Client secret: ")
		secret, err = f.ReadHiddenSecret()
	} else {
		secret, err = f.ReadAll()
	}
	if err != nil {
		return config.Credentials{}, err
	}
	credentials := config.Credentials{
		ClientID:     strings.TrimSpace(clientID),
		ClientSecret: strings.TrimSpace(secret),
	}
	if credentials.ClientID == "" || credentials.ClientSecret == "" {
		return config.Credentials{}, &cmdutil.UsageError{Message: "both a client id and a client secret are required"}
	}
	return credentials, nil
}
