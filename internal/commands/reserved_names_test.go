package commands

import (
	"slices"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/commands/generated"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/gen"
)

// The generator checks generated names against lists of the root's own;
// these keep the lists from drifting from the root they describe.

func TestTheGeneratorReservesExactlyTheRootsGlobalFlags(t *testing.T) {
	root := newRoot(cmdutil.Deps{}, &updateNotice{})
	root.InitDefaultHelpFlag()
	got := []string{}
	root.PersistentFlags().VisitAll(func(flag *pflag.Flag) { got = append(got, flag.Name) })
	got = append(got, "help")

	assertSameNames(t, "global flags", got, gen.GlobalFlags())
}

func TestTheGeneratorReservesExactlyTheRootsHandWrittenCommands(t *testing.T) {
	root := newRoot(cmdutil.Deps{}, &updateNotice{})
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	groups := &cobra.Command{Use: "tmh"}
	generated.Register(groups, cmdutil.NewFactory(cmdutil.Deps{}))
	generatedNames := []string{}
	for _, group := range groups.Commands() {
		generatedNames = append(generatedNames, group.Name())
	}
	got := []string{}
	for _, command := range root.Commands() {
		if !slices.Contains(generatedNames, command.Name()) {
			got = append(got, command.Name())
		}
	}

	assertSameNames(t, "root commands", got, gen.RootCommands())
}

func assertSameNames(t *testing.T, what string, got, want []string) {
	t.Helper()
	got, want = slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))
	if !slices.Equal(got, want) {
		t.Errorf("root %s = %v, generator reserves %v", what, got, want)
	}
}
