package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/api"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/config"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/oauth"
)

// partnerLogin is `tmh login --partner-app`: the PKCE consent flow with a
// loopback redirect (RFC 8252).
type partnerLogin struct {
	enabled      bool
	scopes       []string
	redirectPort int
	authorizeURL string
}

var partnerAppPrompts = prompts{
	id:         "Partner app ID: ",
	secret:     "Partner app secret: ",
	noTerminal: "no terminal to prompt on: pass --client-id and pipe the Partner app secret on stdin",
	missing:    "both a Partner app id and a Partner app secret are required",
}

var partnerOnlyFlags = []string{"scopes", "redirect-port", "authorize-url"}

func (p *partnerLogin) bindFlags(command *cobra.Command) {
	flags := command.Flags()
	flags.BoolVar(&p.enabled, "partner-app", false, "connect a Partner app to an Organization through the console's consent page")
	flags.StringSliceVar(&p.scopes, "scopes", nil, "with --partner-app, the Scopes to ask the admin for, comma-separated")
	flags.IntVar(&p.redirectPort, "redirect-port", 0, "with --partner-app, the 127.0.0.1 port for the consent redirect (default: any free port)")
	flags.StringVar(&p.authorizeURL, "authorize-url", "", "with --partner-app, the consent page to open instead of the profile's (required for Sandbox)")
}

func (p *partnerLogin) rejectFlagsWithout(cmd *cobra.Command) error {
	for _, name := range partnerOnlyFlags {
		if cmd.Flags().Changed(name) {
			return &cmdutil.UsageError{Message: fmt.Sprintf("--%s needs --partner-app", name)}
		}
	}
	return nil
}

func (p *partnerLogin) run(cmd *cobra.Command, f *cmdutil.Factory, profile config.Profile, clientID string) error {
	authorizeURL := p.authorizeURL
	if authorizeURL == "" {
		authorizeURL = profile.AuthorizeURL
	}
	if authorizeURL == "" {
		return &cmdutil.UsageError{Message: fmt.Sprintf(
			"the %s profile has no published consent page: pass --authorize-url", profile.Name)}
	}
	if p.redirectPort < 0 || p.redirectPort > 65535 {
		return &cmdutil.UsageError{Message: "--redirect-port must be between 1 and 65535"}
	}
	credentials, err := readCredentials(f, clientID, partnerAppPrompts)
	if err != nil {
		return err
	}
	credentials.PartnerApp = true

	pkce, err := oauth.NewPKCE()
	if err != nil {
		return err
	}
	state, err := oauth.NewState()
	if err != nil {
		return err
	}
	loopback, err := oauth.Listen(p.redirectPort, state)
	if err != nil {
		return err
	}
	defer loopback.Close()
	consentURL, err := oauth.AuthorizeURL(authorizeURL, oauth.Consent{
		ClientID:    credentials.ClientID,
		RedirectURI: loopback.RedirectURI(),
		Scopes:      p.scopes,
		State:       state,
		Challenge:   pkce.Challenge,
	})
	if err != nil {
		return &cmdutil.UsageError{Message: err.Error()}
	}
	p.openConsent(f, consentURL)
	code, err := loopback.Wait(cmd.Context())
	if err != nil {
		return err
	}

	store := f.Store()
	client := f.NewAPIClient(profile, credentials)
	var inFile bool
	err = client.ExchangeCode(cmd.Context(), api.CodeExchange{
		Code:         code,
		RedirectURI:  loopback.RedirectURI(),
		CodeVerifier: pkce.Verifier,
	}, func(refreshToken string) error {
		var saveErr error
		inFile, saveErr = store.SaveGrant(profile.Name, credentials, refreshToken)
		return saveErr
	})
	if err != nil {
		return err
	}
	if inFile {
		fmt.Fprintf(f.Stderr, "OS keyring unavailable; stored the Grant in %s, readable only by you.\n", store.Path())
	}
	organizationID, err := client.OrganizationID(cmd.Context())
	if err != nil {
		return err
	}
	if err := store.SetOrganizationID(profile.Name, organizationID); err != nil {
		return err
	}
	fmt.Fprintf(f.Stdout, "Logged in to %s as Organization %s through Partner app %s.\n",
		profile.Environment, organizationID, credentials.ClientID)
	return nil
}

// openConsent always prints the URL too, so the person can finish the login
// in another browser when the opened one is the wrong one.
func (p *partnerLogin) openConsent(f *cmdutil.Factory, consentURL string) {
	fmt.Fprintf(f.Stderr, "Open this URL to connect an Organization, then return here:\n  %s\n", consentURL)
	if f.OpenBrowser == nil {
		return
	}
	if err := f.OpenBrowser(consentURL); err != nil {
		fmt.Fprintf(f.Stderr, "Could not open a browser (%v); open the URL above instead.\n", err)
	}
}
