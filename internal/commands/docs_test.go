package commands_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/commands"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/doctest"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/gen"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

// The documentation tests: every shell example in the README, the guides
// and examples/ names real commands and flags, and every session annotated
// with a name runs against the fake transport and prints exactly what the
// docs show.

const repoRoot = "../.."

// shellLangs are the fenced-block languages that hold shell.
var shellLangs = []string{"console", "sh", "shell", "bash"}

// documentedPages are the Markdown pages whose examples are tested.
func documentedPages(t *testing.T) []string {
	t.Helper()
	topics, err := filepath.Glob(filepath.Join(repoRoot, "docs/topics/*.md"))
	if err != nil || len(topics) == 0 {
		t.Fatalf("no topic pages found: %v", err)
	}
	return append([]string{filepath.Join(repoRoot, "README.md"), filepath.Join(repoRoot, "examples/README.md")}, topics...)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func rel(path string) string {
	name, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return path
	}
	return name
}

func TestEveryShellExampleIsTestedOrMarkedIllustrative(t *testing.T) {
	for _, page := range documentedPages(t) {
		for _, block := range doctest.Blocks(readFile(t, page)) {
			if slices.Contains(shellLangs, block.Lang) && block.Annotation == "" {
				t.Errorf("%s: a %s block needs <!-- doctest: <session> --> or <!-- doctest: illustrative --> before it",
					block.Where(rel(page)), block.Lang)
			}
			switch {
			case block.Annotation == "" || block.Annotation == "illustrative":
			case block.Annotation == "synopsis":
				if block.Lang != "text" {
					t.Errorf("%s: a synopsis is a text block, not %s", block.Where(rel(page)), block.Lang)
				}
			case block.Lang != "console":
				t.Errorf("%s: only console blocks run as sessions; mark this %s block illustrative",
					block.Where(rel(page)), block.Lang)
			case sessions[block.Annotation] == nil:
				t.Errorf("%s: no session setup named %q in docs_sessions_test.go", block.Where(rel(page)), block.Annotation)
			}
		}
	}
}

// shellSources are the shell in the docs and examples that no session
// runs, by where it is.
func shellSources(t *testing.T) map[string]string {
	t.Helper()
	sources := map[string]string{}
	for _, page := range documentedPages(t) {
		for _, block := range doctest.Blocks(readFile(t, page)) {
			// A tested session runs its commands for real, mistakes included.
			if !slices.Contains(shellLangs, block.Lang) || (block.Lang == "console" && block.Annotation != "illustrative") {
				continue
			}
			source := block.Body
			if block.Lang == "console" {
				source = strings.Join(promptedCommands(block.Body), "\n")
			}
			sources[block.Where(rel(page))] = source
		}
	}
	scripts, err := filepath.Glob(filepath.Join(repoRoot, "examples/*.sh"))
	if err != nil || len(scripts) == 0 {
		t.Fatalf("no example scripts found: %v", err)
	}
	for _, script := range scripts {
		sources[rel(script)] = readFile(t, script)
	}
	return sources
}

func TestEveryTmhCommandInTheDocsAndExamplesExists(t *testing.T) {
	checked := 0
	for where, source := range shellSources(t) {
		invocations, err := doctest.Invocations(source)
		if err != nil {
			t.Errorf("%s: %v", where, err)
			continue
		}
		for _, args := range invocations {
			checked++
			if err := checkInvocation(args); err != nil {
				t.Errorf("%s: tmh %s: %v", where, strings.Join(args, " "), err)
			}
		}
	}
	if checked < 30 {
		t.Errorf("checked only %d tmh commands; is the shell parser finding them?", checked)
	}
}

// checkInvocation resolves args to a command and parses its flags and
// arguments the way running it would, without running it.
func checkInvocation(args []string) error {
	cmd, err := parseInvocation(args)
	if errors.Is(err, pflag.ErrHelp) {
		return nil
	} else if err != nil {
		return err
	}
	if version, _ := cmd.Flags().GetBool("version"); version && !cmd.HasParent() {
		return nil
	}
	if !cmd.Runnable() {
		return fmt.Errorf("%q is a group of commands, not a command", cmd.CommandPath())
	}
	if err := cmd.ValidateArgs(cmd.Flags().Args()); err != nil {
		return err
	}
	if err := cmd.ValidateRequiredFlags(); err != nil {
		return err
	}
	return cmd.ValidateFlagGroups()
}

var (
	synopsisWord  = regexp.MustCompile(`^[a-z][a-z-]*$`)
	synopsisFlags = regexp.MustCompile(`(?:^|[\s\[|])(--[a-z][a-z0-9.-]*|-[a-zA-Z])\b`)
)

// TestEverySynopsisNamesRealCommandsAndFlags checks the usage blocks that
// open the topic pages, `<!-- doctest: synopsis -->` text blocks with one
// `tmh …` command per line: the command exists and has every flag named.
func TestEverySynopsisNamesRealCommandsAndFlags(t *testing.T) {
	checked := 0
	for _, page := range documentedPages(t) {
		for _, block := range doctest.Blocks(readFile(t, page)) {
			if block.Annotation != "synopsis" {
				continue
			}
			for _, line := range strings.Split(strings.TrimSpace(block.Body), "\n") {
				checked++
				words := strings.Fields(line)
				if len(words) == 0 || words[0] != "tmh" {
					t.Errorf("%s: %q: a synopsis line starts with tmh", block.Where(rel(page)), line)
					continue
				}
				path := []string{}
				for _, word := range words[1:] {
					if !synopsisWord.MatchString(word) {
						break
					}
					path = append(path, word)
				}
				root := commands.NewRoot(cmdutil.Deps{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard})
				// Words past the command are literal arguments, such as
				// `tmh open docs`; a group of commands takes none.
				cmd, rest, err := root.Find(path)
				if err == nil && len(rest) > 0 {
					if cmd.HasSubCommands() {
						err = fmt.Errorf("unknown command %q", rest[0])
					} else {
						err = cmd.ValidateArgs(rest)
					}
				}
				if err != nil {
					t.Errorf("%s: %q: %v", block.Where(rel(page)), line, err)
					continue
				}
				for _, match := range synopsisFlags.FindAllStringSubmatch(line, -1) {
					name := strings.TrimLeft(match[1], "-")
					var flag *pflag.Flag
					if len(name) == 1 {
						flag = cmd.Flags().ShorthandLookup(name)
						if flag == nil {
							flag = cmd.InheritedFlags().ShorthandLookup(name)
						}
					} else {
						flag = cmd.Flag(name)
					}
					if flag == nil {
						t.Errorf("%s: %q: %s has no flag %s", block.Where(rel(page)), line, cmd.CommandPath(), match[1])
					}
				}
			}
		}
	}
	if checked < 20 {
		t.Errorf("checked only %d synopsis lines; are the topic pages annotated?", checked)
	}
}

// TestEveryJqFilterInTheDocsCompiles compiles each jq program in the docs,
// the examples and the code samples with the jq on PATH. Only a compile
// error (jq's exit code 3) fails: the programs run on null here, not on
// what tmh prints.
func TestEveryJqFilterInTheDocsCompiles(t *testing.T) {
	jq, err := exec.LookPath("jq")
	if err != nil {
		t.Skip("jq isn't installed")
	}
	sources := shellSources(t)
	for id, sample := range readCodeSamples(t).Samples {
		sources[gen.SamplesPath+" "+id] = sample
	}
	checked := 0
	for where, source := range sources {
		commands, err := doctest.SimpleCommands(source)
		if err != nil {
			t.Errorf("%s: %v", where, err)
			continue
		}
		for _, words := range commands {
			if len(words) < 2 || words[0] != "jq" {
				continue
			}
			args, program := jqProgram(words[1:])
			if program == "" {
				continue
			}
			checked++
			cmd := exec.Command(jq, append(append(args, "-n"), program)...)
			cmd.Stdin = strings.NewReader("")
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 3 {
				t.Errorf("%s: jq %s doesn't compile: %s", where, program, output)
			}
		}
	}
	if checked < 30 {
		t.Errorf("compiled only %d jq programs; is the shell parser finding them?", checked)
	}
}

// jqProgram splits jq's arguments into its options and its program: the
// first argument that isn't an option or an option's value.
func jqProgram(words []string) (options []string, program string) {
	takesTwo := []string{"--arg", "--argjson", "--slurpfile", "--rawfile"}
	for i := 0; i < len(words); i++ {
		switch {
		case slices.Contains(takesTwo, words[i]) && i+2 < len(words):
			options = append(options, words[i:i+3]...)
			i += 2
		case strings.HasPrefix(words[i], "-") && words[i] != "-":
			options = append(options, words[i])
		default:
			return options, words[i]
		}
	}
	return options, ""
}

// parseInvocation resolves args to a command of a new command tree and
// parses its flags, leaving the arguments in cmd.Flags().Args().
func parseInvocation(args []string) (*cobra.Command, error) {
	root := commands.NewRoot(cmdutil.Deps{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard})
	cmd, rest, err := root.Find(args)
	if err != nil {
		return nil, err
	}
	return cmd, cmd.ParseFlags(rest)
}

// promptedCommands are a console block's commands: each `$ ` line, joined
// with the lines its trailing backslashes continue onto.
func promptedCommands(body string) []string {
	commands := []string{}
	lines := strings.Split(body, "\n")
	for i := 0; i < len(lines); i++ {
		command, ok := strings.CutPrefix(lines[i], "$ ")
		if !ok {
			continue
		}
		for strings.HasSuffix(command, "\\") && i+1 < len(lines) {
			i++
			command = strings.TrimSuffix(command, "\\") + "\n" + lines[i]
		}
		commands = append(commands, command)
	}
	return commands
}

// step is one command of a console session and the output the docs show.
type step struct {
	command string
	want    string
}

func steps(body string) []step {
	out := []step{}
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		command, ok := strings.CutPrefix(lines[i], "$ ")
		if !ok {
			if len(out) == 0 {
				return nil
			}
			out[len(out)-1].want += lines[i] + "\n"
			continue
		}
		for strings.HasSuffix(command, "\\") && i+1 < len(lines) {
			i++
			command = strings.TrimSuffix(command, "\\") + "\n" + lines[i]
		}
		out = append(out, step{command: command})
	}
	return out
}

// docSession is the machine a console session runs on. Its commands run
// in a temporary working directory, on a terminal with NO_COLOR set, the
// way a reader would see them.
type docSession struct {
	t   *testing.T
	env *testkit.Env
	// ctx ends the session's commands; a setup cancels it to stop
	// `events tail` the way Ctrl-C does.
	ctx    context.Context
	cancel context.CancelFunc
}

func (s *docSession) writeFile(name, contents string) {
	s.t.Helper()
	if err := os.WriteFile(name, []byte(contents), 0o600); err != nil {
		s.t.Fatal(err)
	}
}

var redirect = regexp.MustCompile(`\s+>\s*(\S+)\s*$`)

// run executes one step: `tmh …` with optional leading assignments and a
// trailing `> file`, `export NAME=value`, `echo $?`, `cat <file>`, or
// `printf %s "$NAME" | tmh …`.
func (s *docSession) run(command string, lastExit int) (output string, exit int) {
	s.t.Helper()
	target := ""
	if match := redirect.FindStringSubmatch(command); match != nil {
		target = match[1]
		command = command[:len(command)-len(match[0])]
	}
	parsed, err := doctest.SimpleCommands(command)
	if err != nil {
		s.t.Fatalf("%s: %v", command, err)
	}
	stdin := ""
	if len(parsed) == 2 && len(parsed[0]) == 3 && parsed[0][0] == "printf" && parsed[0][1] == "%s" {
		value, ok := strings.CutPrefix(parsed[0][2], "$")
		if !ok {
			s.t.Fatalf("%s: printf a variable such as \"$NAME\"", command)
		}
		stdin = s.env.Vars[value]
		parsed = parsed[1:]
	}
	if len(parsed) != 1 {
		s.t.Fatalf("%s: a tested session runs one command per line", command)
	}
	words := parsed[0]
	switch {
	case words[0] == "export" && len(words) == 2:
		name, value, _ := strings.Cut(words[1], "=")
		s.env.Vars[name] = value
		return "", 0
	case words[0] == "echo" && len(words) == 2 && words[1] == "$?":
		return fmt.Sprintln(lastExit), 0
	case words[0] == "cat" && len(words) == 2:
		return readFile(s.t, words[1]), 0
	}
	vars := map[string]string{}
	for len(words) > 0 && strings.Contains(words[0], "=") && words[0] != "tmh" {
		name, value, _ := strings.Cut(words[0], "=")
		vars[name], words = value, words[1:]
	}
	if len(words) == 0 || words[0] != "tmh" {
		s.t.Fatalf("%s: a tested session can't run %q", command, words)
	}
	saved := s.env.Vars
	s.env.Vars = map[string]string{}
	for name, value := range saved {
		s.env.Vars[name] = value
	}
	for name, value := range vars {
		s.env.Vars[name] = value
	}
	s.env.Stdin = stdin
	s.env.StdoutIsTTY = target == ""
	result := s.env.RunContext(s.ctx, words[1:]...)
	s.env.Vars = saved
	if target == "" {
		return result.Combined, result.ExitCode
	}
	if target != "/dev/null" {
		s.writeFile(target, result.Stdout)
	}
	return result.Stderr, result.ExitCode
}

func TestDocumentedSessionsPrintWhatTheDocsShow(t *testing.T) {
	used := map[string]bool{}
	for _, page := range documentedPages(t) {
		for _, block := range doctest.Blocks(readFile(t, page)) {
			if block.Lang != "console" || block.Annotation == "illustrative" || block.Annotation == "" {
				continue
			}
			setup := sessions[block.Annotation]
			if setup == nil {
				continue
			}
			used[block.Annotation] = true
			t.Run(block.Annotation, func(t *testing.T) {
				t.Chdir(t.TempDir())
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				env := testkit.NewEnv(t)
				env.Vars["NO_COLOR"] = "1"
				s := &docSession{t: t, env: env, ctx: ctx, cancel: cancel}
				setup(s)
				exit := 0
				session := steps(block.Body)
				if len(session) == 0 {
					t.Fatalf("%s: a session starts with a $ line", block.Where(rel(page)))
				}
				for i, st := range session {
					var got string
					got, exit = s.run(st.command, exit)
					if got != st.want {
						t.Errorf("%s: $ %s\ngot:\n%s\nthe docs show:\n%s", block.Where(rel(page)), st.command, got, st.want)
					}
					// A failure the docs don't point out would read as success.
					if exit != 0 && (i+1 == len(session) || session[i+1].command != "echo $?") {
						t.Errorf("%s: $ %s exited %d; show it with echo $?", block.Where(rel(page)), st.command, exit)
					}
				}
			})
		}
	}
	for name := range sessions {
		if !used[name] {
			t.Errorf("session %q has a setup but no console block in the docs", name)
		}
	}
}
