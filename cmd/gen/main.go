// Command gen writes tmh's resource commands and the API reference's code
// samples from the vendored SDK IR. With -check it
// writes nothing and fails when any of them is stale.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/gen"
)

func main() {
	irPath := flag.String("ir", "openapi/sdk-ir.json", "the SDK IR to read")
	out := flag.String("out", "internal/commands/generated", "the directory of generated commands")
	samples := flag.String("samples", gen.SamplesPath, "the code samples file to write")
	check := flag.Bool("check", false, "fail if the generated files are out of date instead of writing them")
	flag.Parse()
	if err := run(*irPath, *out, *samples, *check); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(irPath, out, samples string, check bool) error {
	ir, err := os.ReadFile(irPath)
	if err != nil {
		return err
	}
	files, err := gen.Generate(ir)
	if err != nil {
		return err
	}
	sampleFile, err := gen.Samples(ir)
	if err != nil {
		return err
	}
	if !check {
		if err := gen.Write(out, files); err != nil {
			return err
		}
		return os.WriteFile(samples, sampleFile, 0o644)
	}
	stale, err := gen.Stale(out, files)
	if err != nil {
		return err
	}
	if current, err := os.ReadFile(samples); err != nil || !bytes.Equal(current, sampleFile) {
		stale = append(stale, samples)
	}
	if len(stale) > 0 {
		return fmt.Errorf("out of date in %s: %s; run make generate", out, strings.Join(stale, ", "))
	}
	return nil
}
