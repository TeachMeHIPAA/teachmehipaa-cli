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
	var partner partnerLogin
	command := &cobra.Command{
		Use:   "login",
		Short: "Store an API client's credentials, or a Partner app's Grant, for a profile",
		Long: "Asks for an API client's id and secret, checks them with TeachMeHIPAA and stores\n" +
			"them for the profile: the secret in the OS keyring, or in a private config file\n" +
			"where no keyring exists.\n\n" +
			"Without a terminal, pass --client-id and pipe the secret on stdin:\n\n" +
			"  printf %s \"$SECRET\" | tmh login --client-id cli_…\n\n" +
			"With --partner-app, asks for a Partner app's id and secret instead and opens the\n" +
			"console's consent page, where an Organization admin connects the app. tmh receives\n" +
			"the answer on http://127.0.0.1:<port>/callback and stores the Grant's Refresh token.\n" +
			"Sandbox has no published consent page, so it needs --authorize-url. The API accepts\n" +
			"only https redirect URIs today, so the consent page rejects the loopback one until\n" +
			"loopback redirects are allowed.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			profile, err := f.Profile()
			if err != nil {
				return err
			}
			if partner.enabled {
				return partner.run(cmd, f, profile, clientID)
			}
			if err := partner.rejectFlagsWithout(cmd); err != nil {
				return err
			}
			credentials, err := readCredentials(f, clientID, apiClientPrompts)
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
	command.Flags().StringVar(&clientID, "client-id", "",
		"the API client's id (cli_…), or with --partner-app the Partner app's (app_…); the secret is then read from stdin")
	partner.bindFlags(command)
	return command
}

// prompts name the credential being asked for.
type prompts struct {
	id, secret, noTerminal, missing string
}

var apiClientPrompts = prompts{
	id:         "Client ID: ",
	secret:     "Client secret: ",
	noTerminal: "no terminal to prompt on: pass --client-id and pipe the client secret on stdin",
	missing:    "both a client id and a client secret are required",
}

func readCredentials(f *cmdutil.Factory, clientID string, ask prompts) (config.Credentials, error) {
	if clientID == "" {
		if !f.StdinIsTTY {
			return config.Credentials{}, &cmdutil.UsageError{Message: ask.noTerminal}
		}
		fmt.Fprint(f.Stderr, ask.id)
		line, err := f.ReadLine()
		if err != nil {
			return config.Credentials{}, err
		}
		clientID = line
	}
	var secret string
	var err error
	if f.StdinIsTTY {
		fmt.Fprint(f.Stderr, ask.secret)
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
		return config.Credentials{}, &cmdutil.UsageError{Message: ask.missing}
	}
	return credentials, nil
}
