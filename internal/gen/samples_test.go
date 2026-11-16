package gen_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/gen"
)

func TestTheCommittedCodeSamplesAreCurrent(t *testing.T) {
	ir, err := os.ReadFile("../../openapi/sdk-ir.json")
	if err != nil {
		t.Fatal(err)
	}
	want, err := gen.Samples(ir)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile("../../" + gen.SamplesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(committed, want) {
		t.Errorf("%s is stale; run make generate", gen.SamplesPath)
	}
}
