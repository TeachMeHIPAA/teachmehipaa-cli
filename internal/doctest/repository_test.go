package doctest_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/doctest"
)

const repoRoot = "../.."

// markdownPages is every Markdown file in the repository that GitHub or the
// docs site renders.
func markdownPages(t *testing.T) []string {
	t.Helper()
	pages := []string{}
	err := filepath.WalkDir(repoRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && path != repoRoot && (strings.HasPrefix(entry.Name(), ".") ||
			entry.Name() == "bin" || entry.Name() == "dist" || entry.Name() == "_site") {
			return filepath.SkipDir
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".md") {
			pages = append(pages, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return pages
}

func TestEveryRelativeLinkAndAnchorInTheDocsResolves(t *testing.T) {
	anchors := map[string]map[string]bool{}
	anchorsOf := func(path string) map[string]bool {
		if cached, ok := anchors[path]; ok {
			return cached
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		anchors[path] = doctest.Anchors(string(raw))
		return anchors[path]
	}
	pages := markdownPages(t)
	links := 0
	for _, page := range pages {
		raw, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		where, _ := filepath.Rel(repoRoot, page)
		for _, link := range doctest.Links(string(raw)) {
			target := link.Target
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			links++
			if strings.HasPrefix(target, "/") {
				t.Errorf("%s:%d: %s is absolute; link relative to the page so it works on GitHub and the docs site",
					where, link.Line, target)
				continue
			}
			file, fragment, _ := strings.Cut(target, "#")
			resolved := page
			if file != "" {
				resolved = filepath.Join(filepath.Dir(page), file)
				if _, err := os.Stat(resolved); err != nil {
					t.Errorf("%s:%d: %s doesn't exist", where, link.Line, target)
					continue
				}
			}
			if fragment != "" && strings.HasSuffix(resolved, ".md") && !anchorsOf(resolved)[fragment] {
				t.Errorf("%s:%d: %s has no heading with the anchor #%s", where, link.Line, file, fragment)
			}
		}
	}
	if len(pages) < 20 || links < 100 {
		t.Errorf("checked %d links in %d pages; is the walk finding the docs?", links, len(pages))
	}
}
