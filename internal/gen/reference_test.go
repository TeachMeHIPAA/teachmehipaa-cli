package gen_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/gen"
)

func reference(t *testing.T) []byte {
	t.Helper()
	ir, err := os.ReadFile("../../openapi/sdk-ir.json")
	if err != nil {
		t.Fatal(err)
	}
	page, err := gen.Reference(ir)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func TestTheCommittedResourceReferenceIsCurrent(t *testing.T) {
	committed, err := os.ReadFile("../../" + gen.ReferencePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(committed, reference(t)) {
		t.Errorf("%s is stale; run make generate", gen.ReferencePath)
	}
}

func TestTheResourceReferenceDescribesEachCommand(t *testing.T) {
	page := string(reference(t))
	// Expected rows come from the developer OpenAPI document's operations.
	sections := map[string][]string{
		"### `tmh learners update`": {
			"`PATCH /organizations/{orgId}/learners/{learnerId}`",
			"| Scopes | `learners:write` |",
			"| Retries | Never retried |",
			"--if-match \"$etag\"",
			"| Conditional | `--if-match <etag>` is required |",
			"[`tmh learners update`](commands/tmh_learners_update.md)",
		},
		"### `tmh certificate-verifications retrieve`": {
			"`GET /certificate-verifications/{tinyId}`",
			"| Scopes | None (public): runs before `tmh login` |",
		},
		"### `tmh enrollments import`": {
			"`POST /organizations/{orgId}/enrollments:import`",
			"| `--file <path>` | The `text/csv` file to upload; `-` reads stdin |",
			"[`tmh enrollments preview-import`](#tmh-enrollments-preview-import)",
			"`tmh` generates an Idempotency-Key and reuses it on retries",
		},
		"### `tmh certificates download-pdf`": {
			"The `application/pdf` file; `-o <file>` saves it",
		},
	}
	for heading, rows := range sections {
		_, section, found := strings.Cut(page, heading+"\n")
		if !found {
			t.Errorf("no section %s", heading)
			continue
		}
		section, _, _ = strings.Cut(section, "\n### ")
		for _, row := range rows {
			if !strings.Contains(section, row) {
				t.Errorf("%s lacks %q", heading, row)
			}
		}
	}
	if n := strings.Count(page, "\n### `tmh "); n != 45 {
		t.Errorf("%d command sections, want one per IR method (45)", n)
	}
}
