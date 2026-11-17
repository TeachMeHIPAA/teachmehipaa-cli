package commands_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

func TestOpenOpensEachPageInTheBrowser(t *testing.T) {
	tests := []struct {
		page string
		want string
	}{
		{"docs", "https://docs.teachmehipaa.com/api?lang=cli"},
		{"reference", "https://docs.teachmehipaa.com/api?lang=cli"},
		{"console", "https://console.teachmehipaa.com"},
		{"dashboard", "https://console.teachmehipaa.com"},
	}
	for _, tt := range tests {
		t.Run(tt.page, func(t *testing.T) {
			env := testkit.NewEnv(t)

			result := env.Run("open", tt.page)

			if result.ExitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
			}
			if !slices.Equal(env.Opened, []string{tt.want}) {
				t.Errorf("opened %v, want %s", env.Opened, tt.want)
			}
			if routes := env.Transport.Routes(); len(routes) != 0 {
				t.Errorf("sent %v, want no requests", routes)
			}
		})
	}
}

func TestOpenPrintsTheURLWithoutABrowser(t *testing.T) {
	env := testkit.NewEnv(t)
	env.NoBrowser = true

	result := env.Run("open", "console")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if result.Stdout != "https://console.teachmehipaa.com\n" {
		t.Errorf("stdout = %q, want the URL", result.Stdout)
	}
}

func TestOpenPrintsTheURLWhenTheBrowserFails(t *testing.T) {
	env := testkit.NewEnv(t)
	env.BrowserErr = errors.New("xdg-open: no method available")

	result := env.Run("open", "docs")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if result.Stdout != "https://docs.teachmehipaa.com/api?lang=cli\n" {
		t.Errorf("stdout = %q, want the URL", result.Stdout)
	}
	if !strings.Contains(result.Stderr, "xdg-open: no method available") {
		t.Errorf("stderr = %q, want the browser's error", result.Stderr)
	}
}

func TestOpenAnUnknownPageIsAUsageError(t *testing.T) {
	env := testkit.NewEnv(t)

	result := env.Run("open", "billing")

	if result.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1", result.ExitCode)
	}
	if len(env.Opened) != 0 {
		t.Errorf("opened %v, want nothing", env.Opened)
	}
}
