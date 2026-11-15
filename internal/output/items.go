package output

import (
	"bytes"
	"encoding/json"
	"io"
)

// Items writes list items as they arrive, so `--all` shows the first page
// before the last one is fetched. As an array, the closing bracket comes
// only from Close, so a listing cut short by an error or Ctrl-C never parses
// as a complete one.
type Items struct {
	w     io.Writer
	style Style
	// lines writes NDJSON: one compact item per line and no array.
	lines bool
	count int
}

// NewArray writes the items as one JSON array, indented on a terminal.
func NewArray(w io.Writer, style Style) *Items { return &Items{w: w, style: style} }

// NewLines writes the items as NDJSON.
func NewLines(w io.Writer, style Style) *Items {
	return &Items{w: w, style: Style{Color: style.Color}, lines: true}
}

// Write adds one item, which must be valid JSON.
func (it *Items) Write(item []byte) error {
	var formatted bytes.Buffer
	var err error
	if it.style.Pretty {
		err = json.Indent(&formatted, item, "  ", "  ")
	} else {
		err = json.Compact(&formatted, item)
	}
	if err != nil {
		return err
	}
	out := formatted.Bytes()
	if it.style.Color {
		out = colorize(out)
	}
	var prefix string
	switch {
	case it.lines:
	case it.count == 0 && it.style.Pretty:
		prefix = "[\n  "
	case it.count == 0:
		prefix = "["
	case it.style.Pretty:
		prefix = ",\n  "
	default:
		prefix = ","
	}
	if it.lines {
		out = append(out, '\n')
	}
	it.count++
	_, err = it.w.Write(append([]byte(prefix), out...))
	return err
}

// Close ends the array; NDJSON needs no ending.
func (it *Items) Close() error {
	if it.lines {
		return nil
	}
	var end string
	switch {
	case it.count == 0:
		end = "[]\n"
	case it.style.Pretty:
		end = "\n]\n"
	default:
		end = "]\n"
	}
	_, err := io.WriteString(it.w, end)
	return err
}
