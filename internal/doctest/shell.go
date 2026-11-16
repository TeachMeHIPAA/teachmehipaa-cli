package doctest

import (
	"fmt"
	"slices"
	"strings"
)

// SimpleCommands splits POSIX shell source into its simple commands, as
// lists of unquoted words: pipelines, lists, newlines and command
// substitutions separate them, redirections and comments are dropped, and
// no variable is expanded. It covers the shell the docs use, not all of sh.
func SimpleCommands(source string) ([][]string, error) {
	p := &shellParser{src: []rune(source)}
	if err := p.parse(false); err != nil {
		return nil, err
	}
	return p.commands, nil
}

// Invocations returns the arguments of each `tmh` command in source, with
// leading variable assignments and shell keywords skipped.
func Invocations(source string) ([][]string, error) {
	commands, err := SimpleCommands(source)
	if err != nil {
		return nil, err
	}
	keywords := []string{"if", "then", "else", "elif", "do", "while", "until", "!", "time", "exec", "{", "}"}
	invocations := [][]string{}
	for _, words := range commands {
		for len(words) > 0 && (slices.Contains(keywords, words[0]) || isAssignment(words[0])) {
			words = words[1:]
		}
		if len(words) > 0 && words[0] == "tmh" {
			invocations = append(invocations, words[1:])
		}
	}
	return invocations, nil
}

func isAssignment(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found || name == "" {
		return false
	}
	for i, r := range name {
		if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

type shellParser struct {
	src      []rune
	pos      int
	commands [][]string
}

// parse reads commands until the end, or in a command substitution until
// its closing ), which it leaves unread.
func (p *shellParser) parse(nested bool) error {
	words := []string{}
	var word strings.Builder
	inWord, skipNext := false, false
	endWord := func() {
		if inWord {
			if skipNext {
				skipNext = false
			} else {
				words = append(words, word.String())
			}
		}
		word.Reset()
		inWord = false
	}
	endCommand := func() {
		endWord()
		if len(words) > 0 {
			p.commands = append(p.commands, words)
		}
		words = []string{}
	}
	for p.pos < len(p.src) {
		r := p.src[p.pos]
		switch {
		case r == ')' && nested:
			endCommand()
			return nil
		case r == '\\':
			if p.pos+1 < len(p.src) && p.src[p.pos+1] != '\n' {
				word.WriteRune(p.src[p.pos+1])
				inWord = true
			}
			p.pos += 2
		case r == '\'':
			end := strings.IndexRune(string(p.src[p.pos+1:]), '\'')
			if end < 0 {
				return fmt.Errorf("unterminated ' at offset %d", p.pos)
			}
			word.WriteString(string(p.src[p.pos+1 : p.pos+1+end]))
			inWord = true
			p.pos += end + 2
		case r == '"':
			if err := p.doubleQuoted(&word); err != nil {
				return err
			}
			inWord = true
		case r == '$' && p.peek(1) == '(':
			if err := p.substitution(); err != nil {
				return err
			}
			word.WriteString("$(…)")
			inWord = true
		case r == '#' && !inWord:
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
		case r == '\n' || r == ';' || r == '|' || r == '&':
			if r == '&' && p.peek(1) == '>' {
				endWord()
				p.pos += 2
				skipNext = true
				continue
			}
			endCommand()
			p.pos++
			if (r == '|' || r == '&') && p.peek(0) == r {
				p.pos++
			}
		case r == '>' || r == '<':
			// A redirection's target is not an argument; a leading fd
			// number belongs to the redirection too.
			if inWord && isDigits(word.String()) {
				word.Reset()
				inWord = false
			}
			endWord()
			p.pos++
			if p.peek(0) == '>' {
				p.pos++
			}
			if p.peek(0) == '&' {
				p.pos++
				for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
					p.pos++
				}
				continue
			}
			skipNext = true
		case r == ' ' || r == '\t':
			endWord()
			p.pos++
		default:
			word.WriteRune(r)
			inWord = true
			p.pos++
		}
	}
	if nested {
		return fmt.Errorf("unterminated $( in %q", string(p.src))
	}
	endCommand()
	return nil
}

// substitution parses the commands of the $(…) at pos and moves past it.
func (p *shellParser) substitution() error {
	p.pos += 2
	if err := p.parse(true); err != nil {
		return err
	}
	p.pos++
	return nil
}

func (p *shellParser) doubleQuoted(word *strings.Builder) error {
	p.pos++
	for p.pos < len(p.src) {
		r := p.src[p.pos]
		switch {
		case r == '"':
			p.pos++
			return nil
		case r == '\\' && strings.ContainsRune("\"\\$`\n", p.peek(1)):
			if p.peek(1) != '\n' {
				word.WriteRune(p.peek(1))
			}
			p.pos += 2
		case r == '$' && p.peek(1) == '(':
			if err := p.substitution(); err != nil {
				return err
			}
			word.WriteString("$(…)")
		default:
			word.WriteRune(r)
			p.pos++
		}
	}
	return fmt.Errorf("unterminated \" in %q", string(p.src))
}

func (p *shellParser) peek(offset int) rune {
	if p.pos+offset < len(p.src) {
		return p.src[p.pos+offset]
	}
	return 0
}

func isDigits(s string) bool {
	return s != "" && strings.Trim(s, "0123456789") == ""
}
