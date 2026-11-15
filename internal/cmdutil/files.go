package cmdutil

import (
	"fmt"
	"os"
)

// WriteOutput saves a downloaded file to path, or to stdout for "-". A
// binary file goes to stdout only when it is piped: on a terminal it would
// only garble the screen.
func (f *Factory) WriteOutput(path string, data []byte, binary bool) error {
	if err := f.CheckOutput(path, binary); err != nil {
		return err
	}
	if path == "-" {
		_, err := f.Stdout.Write(data)
		return err
	}
	// Downloads hold Learner data, so only their owner may read them.
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("saving the download: %w", err)
	}
	return nil
}

// CheckOutput fails as WriteOutput would on a terminal, so a command can
// refuse before downloading anything.
func (f *Factory) CheckOutput(path string, binary bool) error {
	if path == "-" && binary && f.StdoutIsTTY {
		return &UsageError{Message: "refusing to print a binary file to the terminal; pipe it or use -o <file>"}
	}
	return nil
}

// OutputName is how a summary line names where WriteOutput put the file.
func OutputName(path string) string {
	if path == "-" {
		return "stdout"
	}
	return path
}

// SaveDownload writes a downloaded file unchanged and says where on stderr.
func (f *Factory) SaveDownload(path string, data []byte) error {
	if err := f.WriteOutput(path, data, true); err != nil {
		return err
	}
	fmt.Fprintf(f.Stderr, "Saved %d bytes to %s\n", len(data), OutputName(path))
	return nil
}
