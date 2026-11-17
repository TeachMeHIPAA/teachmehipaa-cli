package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
)

const (
	// developerDocsURL serves both the guides and the API reference; the
	// developer OpenAPI document declares no separate externalDocs.
	developerDocsURL = "https://docs.teachmehipaa.com/api?lang=cli"
	consoleURL       = "https://console.teachmehipaa.com"
)

var openPages = map[string]string{
	"docs":      developerDocsURL,
	"reference": developerDocsURL,
	"console":   consoleURL,
	"dashboard": consoleURL,
}

func newOpenCommand(f *cmdutil.Factory) *cobra.Command {
	pages := make([]string, 0, len(openPages))
	for page := range openPages {
		pages = append(pages, page)
	}
	slices.Sort(pages)
	return &cobra.Command{
		Use:   "open <" + strings.Join(pages, "|") + ">",
		Short: "Open the developer docs or the TeachMeHIPAA console in your browser",
		Long: "Opens a TeachMeHIPAA page in your browser: docs or reference for the Developer API docs, " +
			"console or dashboard for the console where admins manage API clients. " +
			"Without a browser, it prints the URL instead.",
		ValidArgs: pages,
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := openPages[args[0]]
			if f.OpenBrowser == nil {
				_, err := fmt.Fprintln(f.Stdout, url)
				return err
			}
			if err := f.OpenBrowser(url); err != nil {
				fmt.Fprintf(f.Stderr, "Could not open a browser (%v); open this URL instead:\n", err)
				_, err := fmt.Fprintln(f.Stdout, url)
				return err
			}
			return nil
		},
	}
}
