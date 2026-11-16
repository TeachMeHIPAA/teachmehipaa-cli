// Package doctest reads the repository's Markdown and shell scripts for the
// documentation tests: fenced code blocks and their annotations, links and
// heading anchors, and the `tmh` invocations in shell code.
package doctest

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Block is a fenced code block.
type Block struct {
	// Line is the 1-based line of the opening fence.
	Line int
	// Lang is the info string's first word, such as "console" or "sh".
	Lang string
	// Annotation is the HTML comment on the line before the fence,
	// `<!-- doctest: <annotation> -->`, or "" without one.
	Annotation string
	Body       string
}

// Where is the block's position for a test failure.
func (b Block) Where(file string) string { return fmt.Sprintf("%s:%d", file, b.Line) }

var annotation = regexp.MustCompile(`^\s*<!--\s*doctest:\s*(.*?)\s*-->\s*$`)

// Blocks returns every fenced code block in markdown, in order.
func Blocks(markdown string) []Block {
	lines := strings.Split(markdown, "\n")
	blocks := []Block{}
	for i := 0; i < len(lines); i++ {
		fence, info, ok := openingFence(lines[i])
		if !ok {
			continue
		}
		block := Block{Line: i + 1}
		if fields := strings.Fields(info); len(fields) > 0 {
			block.Lang = fields[0]
		}
		if i > 0 {
			if match := annotation.FindStringSubmatch(lines[i-1]); match != nil {
				block.Annotation = match[1]
			}
		}
		body := []string{}
		for i++; i < len(lines) && !closesFence(lines[i], fence); i++ {
			body = append(body, lines[i])
		}
		block.Body = strings.Join(body, "\n")
		if len(body) > 0 {
			block.Body += "\n"
		}
		blocks = append(blocks, block)
	}
	return blocks
}

func openingFence(line string) (fence, info string, ok bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return "", "", false
	}
	for _, mark := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, mark) {
			n := len(trimmed) - len(strings.TrimLeft(trimmed, mark[:1]))
			return trimmed[:n], strings.TrimSpace(trimmed[n:]), true
		}
	}
	return "", "", false
}

func closesFence(line, fence string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]) == ""
}

// prose is markdown with fenced blocks and inline code blanked, so links
// and headings are only found where they render. Line numbers survive.
func prose(markdown string) []string {
	lines := strings.Split(markdown, "\n")
	out := make([]string, len(lines))
	for i := 0; i < len(lines); i++ {
		fence, _, ok := openingFence(lines[i])
		if !ok {
			out[i] = inlineCode.ReplaceAllString(lines[i], "``")
			continue
		}
		for i++; i < len(lines) && !closesFence(lines[i], fence); i++ {
		}
	}
	return out
}

var inlineCode = regexp.MustCompile("`+[^`]*`+")

// Link is an inline link or image target.
type Link struct {
	Line   int
	Target string
}

var linkTarget = regexp.MustCompile(`\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

// Links returns every inline link target outside code.
func Links(markdown string) []Link {
	links := []Link{}
	for i, line := range prose(markdown) {
		for _, match := range linkTarget.FindAllStringSubmatch(line, -1) {
			links = append(links, Link{Line: i + 1, Target: match[1]})
		}
	}
	return links
}

var (
	heading    = regexp.MustCompile(`^ {0,3}#{1,6}\s+(.*?)\s*#*\s*$`)
	htmlAnchor = regexp.MustCompile(`<a\s+(?:id|name)="([^"]+)"`)
)

// Anchors returns the fragment ids GitHub gives markdown's headings,
// numbering repeats as it does, plus explicit <a id> anchors.
func Anchors(markdown string) map[string]bool {
	anchors := map[string]bool{}
	seen := map[string]int{}
	original := strings.Split(markdown, "\n")
	for i, line := range prose(markdown) {
		if match := heading.FindStringSubmatch(line); match != nil {
			// Inline code was blanked to find the heading; its text counts.
			text := heading.FindStringSubmatch(original[i])[1]
			slug := Slug(text)
			if n := seen[slug]; n > 0 {
				anchors[fmt.Sprintf("%s-%d", slug, n)] = true
			} else {
				anchors[slug] = true
			}
			seen[slug]++
		}
		for _, match := range htmlAnchor.FindAllStringSubmatch(line, -1) {
			anchors[match[1]] = true
		}
	}
	return anchors
}

// Slug is GitHub's anchor for a heading: link text instead of the link,
// lowercased, punctuation besides hyphens and underscores removed, and
// spaces as hyphens.
func Slug(text string) string {
	text = markdownLink.ReplaceAllString(text, "$1")
	var out strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r == ' ':
			out.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			out.WriteRune(r)
		}
	}
	return out.String()
}

var markdownLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
