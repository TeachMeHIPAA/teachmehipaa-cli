package gen

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Stale lists the files in dir that differ from files: changed, missing, or
// generated earlier for a resource the IR no longer has.
func Stale(dir string, files map[string][]byte) ([]string, error) {
	stale := []string{}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if !bytes.Equal(got, want) {
			stale = append(stale, name)
		}
	}
	leftovers, err := leftovers(dir, files)
	if err != nil {
		return nil, err
	}
	stale = append(stale, leftovers...)
	slices.Sort(stale)
	return stale, nil
}

// Write replaces the generated files in dir and deletes the ones no longer
// generated. Hand-written files there, which lack the banner, are kept.
func Write(dir string, files map[string][]byte) error {
	leftovers, err := leftovers(dir, files)
	if err != nil {
		return err
	}
	for _, name := range leftovers {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), contents, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func leftovers(dir string, files map[string][]byte) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if _, current := files[name]; current || entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		if bytes.HasPrefix(contents, []byte(Banner+"\n")) {
			names = append(names, name)
		}
	}
	return names, nil
}
