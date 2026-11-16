package commands_test

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/doctest"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/gen"
)

// The API reference shows docs/reference/code-samples.json as the `cli`
// sample of every operation. These tests hold each sample to the command
// tree: it calls the command for its operationId, with arguments and flags
// tmh accepts, and sets every body field the operation requires.

type codeSamples struct {
	Language string            `json:"language"`
	Release  string            `json:"release"`
	Samples  map[string]string `json:"samples"`
}

func readCodeSamples(t *testing.T) codeSamples {
	t.Helper()
	var file codeSamples
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(repoRoot, gen.SamplesPath))), &file); err != nil {
		t.Fatalf("%s: %v", gen.SamplesPath, err)
	}
	return file
}

func irOperations(t *testing.T) []gen.Operation {
	t.Helper()
	ops, err := gen.Operations([]byte(readFile(t, filepath.Join(repoRoot, "openapi/sdk-ir.json"))))
	if err != nil {
		t.Fatal(err)
	}
	return ops
}

func TestTheCodeSamplesCoverEveryOperationOfThisRelease(t *testing.T) {
	file := readCodeSamples(t)
	if file.Language != "cli" {
		t.Errorf("language = %q, want cli", file.Language)
	}
	if want := strings.TrimSpace(readFile(t, filepath.Join(repoRoot, "OPENAPI_VERSION"))); file.Release != want {
		t.Errorf("release = %q, want OPENAPI_VERSION's %q", file.Release, want)
	}
	ops := irOperations(t)
	for _, op := range ops {
		if strings.TrimSpace(file.Samples[op.OperationID]) == "" {
			t.Errorf("no sample for %s", op.OperationID)
		}
	}
	for id := range file.Samples {
		if !slices.ContainsFunc(ops, func(op gen.Operation) bool { return op.OperationID == id }) {
			t.Errorf("a sample for %s, which the IR doesn't have", id)
		}
	}
}

func TestEveryCodeSampleCallsItsOperationWithRealFlags(t *testing.T) {
	file := readCodeSamples(t)
	for _, op := range irOperations(t) {
		sample, ok := file.Samples[op.OperationID]
		if !ok {
			continue
		}
		if lines := strings.Count(sample, "\n") + 1; lines > 15 {
			t.Errorf("%s: %d lines; keep a sample under 15", op.OperationID, lines)
		}
		invocations, err := doctest.Invocations(sample)
		if err != nil {
			t.Errorf("%s: %v", op.OperationID, err)
			continue
		}
		calls := 0
		for _, args := range invocations {
			if err := checkInvocation(args); err != nil {
				t.Errorf("%s: tmh %s: %v", op.OperationID, strings.Join(args, " "), err)
				continue
			}
			cmd, err := parseInvocation(args)
			if err != nil || cmd.Annotations["method"] != op.Method {
				continue
			}
			calls++
			for _, input := range op.Required {
				if !setsBodyInput(cmd, input) {
					t.Errorf("%s: tmh %s doesn't set the required body field %s", op.OperationID, strings.Join(args, " "), input.Key)
				}
			}
		}
		if calls != 1 {
			t.Errorf("%s: the sample runs %s %d times, want once", op.OperationID, op.Method, calls)
		}
	}
}

// setsBodyInput reports whether a parsed command line supplies a body
// field: by its flag, or for a JSON body by -d or a --data document.
func setsBodyInput(cmd *cobra.Command, input gen.BodyInput) bool {
	if input.Flag != "" && cmd.Flags().Changed(input.Flag) {
		return true
	}
	if input.File {
		return false
	}
	data, err := cmd.Flags().GetStringArray("data")
	if err != nil {
		return false
	}
	for _, d := range data {
		if strings.HasPrefix(d, "@") || strings.HasPrefix(strings.TrimSpace(d), "{") {
			return true
		}
		key, _, _ := strings.Cut(d, "=")
		key = strings.TrimSuffix(strings.TrimSuffix(key, ":"), "[]")
		if key == input.Key || strings.HasPrefix(key, input.Key+".") {
			return true
		}
	}
	return false
}
