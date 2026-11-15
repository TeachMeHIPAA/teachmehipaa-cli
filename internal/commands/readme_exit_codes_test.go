package commands

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// readmeExitCodes reads the README's "Exit codes" table: a code in backticks
// in the first column, its meaning in the second.
func readmeExitCodes(t *testing.T) map[int]string {
	t.Helper()
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, found := strings.Cut(string(readme), "\n## Exit codes\n")
	if !found {
		t.Fatal(`README.md has no "## Exit codes" section`)
	}
	section, _, _ = strings.Cut(section, "\n## ")
	row := regexp.MustCompile("(?m)^\\|\\s*`(\\d+)`\\s*\\|\\s*([^|]+?)\\s*\\|")
	codes := map[int]string{}
	for _, match := range row.FindAllStringSubmatch(section, -1) {
		code, _ := strconv.Atoi(match[1])
		if _, dup := codes[code]; dup {
			t.Errorf("README lists exit code %d twice", code)
		}
		codes[code] = strings.ToLower(match[2])
	}
	return codes
}

func TestTheReadmeExitCodeTableMatchesTheExitCodesTmhReturns(t *testing.T) {
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
	documented := readmeExitCodes(t)
	for code, word := range want {
		meaning, ok := documented[code]
		switch {
		case !ok:
			t.Errorf("README's exit-code table is missing %d (%s)", code, word)
		case !strings.Contains(meaning, word):
			t.Errorf("README says exit code %d means %q; want it to mention %q", code, meaning, word)
		}
	}
	for code := range documented {
		if _, ok := want[code]; !ok {
			t.Errorf("README documents exit code %d, which tmh never returns", code)
		}
	}
}
