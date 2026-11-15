// Package commands is tmh's command tree: the root, the hand-written credential
// commands, and the generated resource commands from package generated.
package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/api"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/commands/generated"
)

// Exit codes from the CLI spec.
const (
	exitOK          = 0
	exitUsage       = 1
	exitAuth        = 2
	exitNotFound    = 3
	exitConflict    = 4
	exitInvalid     = 5
	exitRateLimited = 6
	exitServer      = 7
	// exitInterrupted is the shell's code for a process stopped by SIGINT
	// (128 + 2), so an interrupted `--all` or `--wait` never reads as done.
	exitInterrupted = 130
)

// newRoot builds `tmh` with every subcommand wired to deps; the command that
// runs starts the release check in notice.
func newRoot(deps cmdutil.Deps, notice *updateNotice) *cobra.Command {
	f := cmdutil.NewFactory(deps)
	root := &cobra.Command{
		Use:   "tmh",
		Short: "Work with the TeachMeHIPAA Developer API from your terminal",
		Long: `Work with the TeachMeHIPAA Developer API from your terminal.

Shell completion: load it in your shell's startup file with one of
  source <(tmh completion bash)
  source <(tmh completion zsh)
  tmh completion fish | source
  tmh completion powershell | Out-String | Invoke-Expression
See "tmh completion <shell> --help" to install it permanently.`,
		Version:       deps.Build.Version,
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			notice.start(cmd.Context(), cmd)
		},
	}
	// Cobra renders this as a template; quoting braces keeps the build info
	// literal.
	root.SetVersionTemplate(strings.NewReplacer("{", "{{`{`}}", "}", "{{`}`}}").Replace(versionLine(deps.Build)))
	root.SetIn(deps.Stdin)
	root.SetOut(deps.Stdout)
	root.SetErr(deps.Stderr)
	f.BindProfileFlags(root.PersistentFlags())
	f.BindRequestFlags(root.PersistentFlags())

	root.AddCommand(newLoginCommand(f), newLogoutCommand(f), newWhoamiCommand(f))
	root.AddCommand(newRawCommands(f)...)
	root.AddCommand(newOpenCommand(f), newVersionCommand(deps.Build))
	generated.Register(root, f)
	addEventCommands(root, f)
	return root
}

// Execute runs `tmh <args>` and returns the process exit code.
func Execute(ctx context.Context, deps cmdutil.Deps, args []string) int {
	notice := &updateNotice{deps: deps}
	root := newRoot(deps, notice)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if err != nil {
		fmt.Fprintln(deps.Stderr, renderError(err))
	}
	notice.finish()
	return exitCode(err)
}

// renderError never includes a credential: API errors carry only what the
// response said, and request headers never reach an error.
func renderError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "Interrupted"
	}
	lines := []string{"Error: " + err.Error()}
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		for _, field := range apiErr.Errors {
			lines = append(lines, fmt.Sprintf("  %s: %s", field.Path, field.Message))
		}
		if apiErr.RequestID != "" {
			lines = append(lines, "Request ID: "+apiErr.RequestID)
		}
	}
	return strings.Join(lines, "\n")
}

func exitCode(err error) int {
	var apiErr *api.Error
	var connectionErr *api.ConnectionError
	var notLoggedIn *cmdutil.NotLoggedInError
	var notFound *cmdutil.NotFoundError
	var operationFailed *api.OperationFailedError
	var operationTimeout *api.OperationTimeoutError
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, context.Canceled):
		return exitInterrupted
	case errors.As(err, &notLoggedIn):
		return exitAuth
	case errors.As(err, &notFound):
		return exitNotFound
	case errors.As(err, &operationFailed):
		// A failed Operation's problem says what went wrong as an HTTP error
		// would; without a status there is nothing to tell it from a crash.
		if operationFailed.Problem.Status == 0 {
			return exitServer
		}
		return exitCodeForStatus(operationFailed.Problem)
	case errors.As(err, &connectionErr), errors.As(err, &operationTimeout):
		return exitServer
	case errors.As(err, &apiErr):
		return exitCodeForStatus(apiErr)
	}
	return exitUsage
}

func exitCodeForStatus(err *api.Error) int {
	switch {
	case err.Status == 429:
		return exitRateLimited
	case err.UnexpectedBody || err.Status >= 500 || (err.Status >= 200 && err.Status < 300):
		return exitServer
	case err.OAuth || err.Status == 401 || err.Status == 403:
		return exitAuth
	case err.Status == 404:
		return exitNotFound
	case err.Status == 409 || err.Status == 412 || err.Status == 428:
		return exitConflict
	}
	return exitInvalid
}
