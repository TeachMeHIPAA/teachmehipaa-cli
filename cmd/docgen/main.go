// Command docgen writes the Markdown command reference in
// docs/reference/commands from tmh's command tree. With -check it writes
// nothing and fails when the committed reference is stale.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/docgen"
)

func main() {
	out := flag.String("out", docgen.Dir, "the directory of the command reference")
	check := flag.Bool("check", false, "fail if the command reference is out of date instead of writing it")
	flag.Parse()
	if err := run(*out, *check); err != nil {
		fmt.Fprintln(os.Stderr, "docgen:", err)
		os.Exit(1)
	}
}

func run(out string, check bool) error {
	files, err := docgen.Commands()
	if err != nil {
		return err
	}
	if !check {
		return docgen.Write(out, files)
	}
	stale, err := docgen.Stale(out, files)
	if err != nil {
		return err
	}
	if len(stale) > 0 {
		return fmt.Errorf("out of date in %s: %s; run make docs", out, strings.Join(stale, ", "))
	}
	return nil
}
