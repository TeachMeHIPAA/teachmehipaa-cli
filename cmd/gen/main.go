// Command gen writes tmh's resource commands from the vendored SDK IR. With
// -check it writes nothing and fails when the committed commands are stale.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/gen"
)

func main() {
	irPath := flag.String("ir", "openapi/sdk-ir.json", "the SDK IR to read")
	out := flag.String("out", "internal/commands/generated", "the directory of generated commands")
	check := flag.Bool("check", false, "fail if the generated commands are out of date instead of writing them")
	flag.Parse()
	if err := run(*irPath, *out, *check); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(irPath, out string, check bool) error {
	ir, err := os.ReadFile(irPath)
	if err != nil {
		return err
	}
	files, err := gen.Generate(ir)
	if err != nil {
		return err
	}
	if !check {
		return gen.Write(out, files)
	}
	stale, err := gen.Stale(out, files)
	if err != nil {
		return err
	}
	if len(stale) > 0 {
		return fmt.Errorf("out of date in %s: %s; run make generate", out, strings.Join(stale, ", "))
	}
	return nil
}
