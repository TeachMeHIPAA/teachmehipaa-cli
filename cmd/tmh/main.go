// Command tmh is the TeachMeHIPAA CLI.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"

	"golang.org/x/term"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/clock"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/commands"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/config"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/keyring"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/version"
)

func main() {
	configDir, err := config.DefaultDir(os.LookupEnv, os.UserHomeDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
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
		StderrIsTTY: term.IsTerminal(int(os.Stderr.Fd())),
		ReadSecret:  readHiddenSecret,
		ConfigDir:   configDir,
		OpenBrowser: browserOpener(runtime.GOOS, exec.LookPath),
		Build:       version.Current(),
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

// browserOpener uses the OS's URL handler, or returns nil when there is
// none, such as on a server without xdg-open.
func browserOpener(goos string, lookPath func(string) (string, error)) func(string) error {
	name, args := "xdg-open", []string{}
	switch goos {
	case "darwin":
		name = "open"
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler"}
	}
	path, err := lookPath(name)
	if err != nil {
		return nil
	}
	return func(url string) error {
		return exec.Command(path, append(args, url)...).Run()
	}
}
