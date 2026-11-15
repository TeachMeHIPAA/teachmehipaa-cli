package commands_test

import (
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/version"
)

func TestVersionPrintsVersionCommitAndBuildDate(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}} {
		t.Run(args[0], func(t *testing.T) {
			env := testkit.NewEnv(t)
			env.Build = version.Info{Version: "1.2.0", Commit: "3f9c2ab", Date: "2026-11-14T09:30:00Z"}

			result := env.Run(args...)

			if result.ExitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
			}
			if want := "tmh version 1.2.0 (commit 3f9c2ab, built 2026-11-14T09:30:00Z)\n"; result.Stdout != want {
				t.Errorf("stdout = %q, want %q", result.Stdout, want)
			}
		})
	}
}
