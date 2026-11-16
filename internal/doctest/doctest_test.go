package doctest_test

import (
	"reflect"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/doctest"
)

func TestSlugMatchesGitHubHeadingAnchors(t *testing.T) {
	// Each anchor is one GitHub renders for the heading.
	tests := map[string]string{
		"Operations and `--wait`":                   "operations-and---wait",
		"Exit codes":                                "exit-codes",
		"Testing webhooks with `events tail`":       "testing-webhooks-with-events-tail",
		"1. Install `tmh`":                          "1-install-tmh",
		"See [the guide](guide.md) first":           "see-the-guide-first",
		"`tmh learners upsert-by-external-id`":      "tmh-learners-upsert-by-external-id",
		"Configuration, state files — and privacy?": "configuration-state-files--and-privacy",
	}
	for heading, want := range tests {
		if got := doctest.Slug(heading); got != want {
			t.Errorf("Slug(%q) = %q, want %q", heading, got, want)
		}
	}
}

func TestAnchorsNumberRepeatedHeadingsAndIgnoreCode(t *testing.T) {
	markdown := "# Title\n\n## Docker\n\n```sh\n# not a heading\n```\n\n### Docker\n\n<a id=\"custom\"></a>\n"

	got := doctest.Anchors(markdown)

	want := map[string]bool{"title": true, "docker": true, "docker-1": true, "custom": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Anchors = %v, want %v", got, want)
	}
}

func TestLinksSkipCode(t *testing.T) {
	markdown := "See [a](a.md#x) and `[b](b.md)`.\n\n```md\n[c](c.md)\n```\n![d](d.png \"title\")\n"

	got := doctest.Links(markdown)

	want := []doctest.Link{{Line: 1, Target: "a.md#x"}, {Line: 6, Target: "d.png"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Links = %v, want %v", got, want)
	}
}

func TestBlocksReadTheLanguageAndTheAnnotationBeforeTheFence(t *testing.T) {
	markdown := "Text\n<!-- doctest: login -->\n```console\n$ tmh whoami\n```\n\n````sh\n```\n````\n"

	got := doctest.Blocks(markdown)

	want := []doctest.Block{
		{Line: 3, Lang: "console", Annotation: "login", Body: "$ tmh whoami\n"},
		{Line: 7, Lang: "sh", Body: "```\n"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Blocks = %#v, want %#v", got, want)
	}
}

func TestSimpleCommandsSplitsTheShellTheDocsUse(t *testing.T) {
	tests := []struct {
		source string
		want   [][]string
	}{
		{`tmh learners list --all | jq length`, [][]string{{"tmh", "learners", "list", "--all"}, {"jq", "length"}}},
		{"tmh audit-events list --all --ndjson > audit.ndjson", [][]string{{"tmh", "audit-events", "list", "--all", "--ndjson"}}},
		{`tmh learners create --name "Ana Example" -d 'x=y z'`, [][]string{{"tmh", "learners", "create", "--name", "Ana Example", "-d", "x=y z"}}},
		{"tmh a \\\n  --b c # comment\nnext 2>&1", [][]string{{"tmh", "a", "--b", "c"}, {"next"}}},
		{`export S="$(tmh events tail --print-secret)"`, [][]string{{"tmh", "events", "tail", "--print-secret"}, {"export", "S=$(…)"}}},
		{"a && b || c; d &> log", [][]string{{"a"}, {"b"}, {"c"}, {"d"}}},
		{`printf %s "$SECRET" | tmh login`, [][]string{{"printf", "%s", "$SECRET"}, {"tmh", "login"}}},
		{"x < in.csv", [][]string{{"x"}}},
	}
	for _, tt := range tests {
		got, err := doctest.SimpleCommands(tt.source)
		if err != nil {
			t.Errorf("SimpleCommands(%q): %v", tt.source, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("SimpleCommands(%q) = %q, want %q", tt.source, got, tt.want)
		}
	}
}

func TestSimpleCommandsRejectsUnterminatedQuotes(t *testing.T) {
	for _, source := range []string{`echo "a`, `echo 'a`, `echo $(tmh`} {
		if _, err := doctest.SimpleCommands(source); err == nil {
			t.Errorf("SimpleCommands(%q) succeeded, want an error", source)
		}
	}
}

func TestInvocationsSkipAssignmentsAndKeywords(t *testing.T) {
	source := "while read -r id; do\n  TMH_X=1 tmh learners retrieve \"$id\"\ndone < ids.txt\nif ! tmh whoami; then exit 1; fi\n"

	got, err := doctest.Invocations(source)

	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"learners", "retrieve", "$id"}, {"whoami"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Invocations = %q, want %q", got, want)
	}
}
