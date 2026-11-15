package commands_test

import (
	"strings"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

func TestCompletionScriptsGenerateForEveryShell(t *testing.T) {
	tests := []struct {
		shell string
		// marker is a line each shell's script format starts or registers with.
		marker string
	}{
		{"bash", "# bash completion V2 for tmh"},
		{"zsh", "#compdef tmh"},
		{"fish", "complete -c tmh "},
		{"powershell", "Register-ArgumentCompleter -CommandName 'tmh'"},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			env := testkit.NewEnv(t)

			result := env.Run("completion", tt.shell)

			if result.ExitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
			}
			if !strings.Contains(result.Stdout, tt.marker) {
				t.Errorf("stdout does not contain %q:\n%.300s", tt.marker, result.Stdout)
			}
		})
	}
}

func TestHelpExplainsHowToInstallCompletion(t *testing.T) {
	env := testkit.NewEnv(t)

	result := env.Run("--help")

	for _, want := range []string{"tmh completion bash", "tmh completion zsh", "tmh completion fish", "tmh completion powershell"} {
		if !strings.Contains(result.Stdout, want) {
			t.Errorf("tmh --help does not mention %q:\n%s", want, result.Stdout)
		}
	}
}

func TestCompletionOffersOpenPagesAndRawFlags(t *testing.T) {
	env := testkit.NewEnv(t)

	pages := env.Run("__complete", "open", "")
	flags := env.Run("__complete", "post", "/learners", "--")

	for _, want := range []string{"console", "dashboard", "docs", "reference"} {
		if !strings.Contains(pages.Stdout, want+"\n") {
			t.Errorf("open completions = %q, want %s", pages.Stdout, want)
		}
	}
	for _, want := range []string{"--data", "--query", "--content-type", "--if-match", "--idempotency-key"} {
		if !strings.Contains(flags.Stdout, want) {
			t.Errorf("post flag completions = %q, want %s", flags.Stdout, want)
		}
	}
}
