// Package output prints API responses.
package output

import (
	"bytes"
	"encoding/json"
	"io"
)

// Style is how JSON reaches the reader: indented and colored for a person at
// a terminal, compact for a pipe so `tmh … | jq` sees one value per line.
type Style struct {
	Pretty bool
	Color  bool
}

const (
	reset        = "\x1b[0m"
	keyColor     = "\x1b[34m"
	stringColor  = "\x1b[32m"
	numberColor  = "\x1b[36m"
	literalColor = "\x1b[33m"
	nullColor    = "\x1b[90m"
)

// JSON writes body, which must be valid JSON, keeping the API's key order.
func JSON(w io.Writer, body []byte, style Style) error {
	var formatted bytes.Buffer
	var err error
	if style.Pretty {
		err = json.Indent(&formatted, body, "", "  ")
	} else {
		err = json.Compact(&formatted, body)
	}
	if err != nil {
		return err
	}
	out := formatted.Bytes()
	if style.Color {
		out = colorize(out)
	}
	_, err = w.Write(append(out, '\n'))
	return err
}

// colorize walks already-valid, already-formatted JSON, so it only has to
// tell strings, keys and literals apart.
func colorize(in []byte) []byte {
	var out bytes.Buffer
	for i := 0; i < len(in); {
		switch c := in[i]; {
		case c == '"':
			end := stringEnd(in, i)
			color := stringColor
			if isKey(in, end) {
				color = keyColor
			}
			out.WriteString(color)
			out.Write(in[i:end])
			out.WriteString(reset)
			i = end
		case c == '-' || (c >= '0' && c <= '9'):
			end := literalEnd(in, i)
			out.WriteString(numberColor)
			out.Write(in[i:end])
			out.WriteString(reset)
			i = end
		case c == 't' || c == 'f' || c == 'n':
			end := literalEnd(in, i)
			color := literalColor
			if c == 'n' {
				color = nullColor
			}
			out.WriteString(color)
			out.Write(in[i:end])
			out.WriteString(reset)
			i = end
		default:
			out.WriteByte(c)
			i++
		}
	}
	return out.Bytes()
}

// stringEnd returns the index just past the string starting at in[start].
func stringEnd(in []byte, start int) int {
	for i := start + 1; i < len(in); i++ {
		switch in[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return len(in)
}

func literalEnd(in []byte, start int) int {
	i := start
	for i < len(in) && bytes.IndexByte([]byte(",]} \n\t\r:"), in[i]) < 0 {
		i++
	}
	return i
}

func isKey(in []byte, after int) bool {
	for i := after; i < len(in); i++ {
		switch in[i] {
		case ' ', '\n', '\t', '\r':
			continue
		case ':':
			return true
		default:
			return false
		}
	}
	return false
}
