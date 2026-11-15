// Command tmh is the TeachMeHIPAA CLI.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"

	"golang.org/x/term"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/clock"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/commands"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/keyring"
)

func main() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: no user config directory:", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := commands.Execute(ctx, cmdutil.Deps{
		Transport:   http.DefaultTransport,
		Keyring:     keyring.System(),
		Clock:       clock.System(),
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		LookupEnv:   os.LookupEnv,
		StdinIsTTY:  term.IsTerminal(int(os.Stdin.Fd())),
		StdoutIsTTY: term.IsTerminal(int(os.Stdout.Fd())),
		ReadSecret:  readHiddenSecret,
		ConfigDir:   filepath.Join(configDir, "tmh"),
	}, os.Args[1:])
	stop()
	os.Exit(code)
}

func readHiddenSecret() (string, error) {
	secret, err := term.ReadPassword(int(os.Stdin.Fd()))
	// The newline the person typed was never echoed.
	fmt.Fprintln(os.Stderr)
	return string(secret), err
}
