package commands

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// documentedExitCodes reads a page's "Exit codes" table: a code in
// backticks in the first column, its meaning in the second.
func documentedExitCodes(t *testing.T, page string) map[int]string {
	t.Helper()
	markdown, err := os.ReadFile("../../" + page)
	if err != nil {
		t.Fatal(err)
	}
	_, section, found := strings.Cut(string(markdown), "\n## Exit codes\n")
	if !found {
		t.Fatalf(`%s has no "## Exit codes" section`, page)
	}
	section, _, _ = strings.Cut(section, "\n## ")
	row := regexp.MustCompile("(?m)^\\|\\s*`(\\d+)`\\s*\\|\\s*([^|]+?)\\s*\\|")
	codes := map[int]string{}
	for _, match := range row.FindAllStringSubmatch(section, -1) {
		code, _ := strconv.Atoi(match[1])
		if _, dup := codes[code]; dup {
			t.Errorf("%s lists exit code %d twice", page, code)
		}
		codes[code] = strings.ToLower(match[2])
	}
	return codes
}

func TestTheDocumentedExitCodeTablesMatchTheExitCodesTmhReturns(t *testing.T) {
	// Each exit code with a word its README meaning must contain, so a
	// renumbered code or swapped row fails as surely as a missing one.
	want := map[int]string{
		exitOK:          "success",
		exitUsage:       "usage",
		exitAuth:        "auth",
		exitNotFound:    "not found",
		exitConflict:    "conflict",
		exitInvalid:     "invalid",
		exitRateLimited: "rate limited",
		exitServer:      "server",
		exitInterrupted: "interrupted",
	}
	for _, page := range []string{"README.md", "docs/topics/exit-codes.md"} {
		documented := documentedExitCodes(t, page)
		for code, word := range want {
			meaning, ok := documented[code]
			switch {
			case !ok:
				t.Errorf("%s's exit-code table is missing %d (%s)", page, code, word)
			case !strings.Contains(meaning, word):
				t.Errorf("%s says exit code %d means %q; want it to mention %q", page, code, meaning, word)
			}
		}
		for code := range documented {
			if _, ok := want[code]; !ok {
				t.Errorf("%s documents exit code %d, which tmh never returns", page, code)
			}
		}
	}
}
