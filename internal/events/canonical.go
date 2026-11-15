package events

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Canonicalize re-encodes a JSON document byte for byte as the platform's
// canonicalizeEventBody does: JSON.stringify of the value with object keys
// sorted recursively. The body hash covers these bytes, so they must match
// what a real Delivery of the same Event would carry.
func Canonicalize(document []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, fmt.Errorf("more than one JSON value")
	}
	var out bytes.Buffer
	if err := writeCanonical(&out, value); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeCanonical(out *bytes.Buffer, value any) error {
	switch value := value.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		out.WriteString(strconv.FormatBool(value))
	case json.Number:
		number, err := javaScriptNumber(value)
		if err != nil {
			return err
		}
		out.WriteString(number)
	case string:
		writeString(out, value)
	case []any:
		out.WriteByte('[')
		for i, item := range value {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeCanonical(out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case map[string]any:
		out.WriteByte('{')
		for i, key := range javaScriptKeyOrder(value) {
			if i > 0 {
				out.WriteByte(',')
			}
			writeString(out, key)
			out.WriteByte(':')
			if err := writeCanonical(out, value[key]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	}
	return nil
}

// javaScriptKeyOrder is the order JSON.stringify emits a sorted object in:
// toSorted() compares UTF-16 code units, but a JavaScript object always
// enumerates array-index keys first, in numeric order, whatever order they
// were inserted in.
func javaScriptKeyOrder(object map[string]any) []string {
	indices, names := []string{}, []string{}
	for key := range object {
		if _, ok := arrayIndex(key); ok {
			indices = append(indices, key)
		} else {
			names = append(names, key)
		}
	}
	slices.SortFunc(indices, func(a, b string) int {
		x, _ := arrayIndex(a)
		y, _ := arrayIndex(b)
		return cmp.Compare(x, y)
	})
	slices.SortFunc(names, func(a, b string) int {
		return slices.Compare(utf16.Encode([]rune(a)), utf16.Encode([]rune(b)))
	})
	return append(indices, names...)
}

// arrayIndex accepts the canonical decimal form of 0 ≤ n < 2³²−1.
func arrayIndex(key string) (int64, bool) {
	if key == "" || (len(key) > 1 && key[0] == '0') {
		return 0, false
	}
	n, err := strconv.ParseUint(key, 10, 32)
	if err != nil || n == math.MaxUint32 || strconv.FormatUint(n, 10) != key {
		return 0, false
	}
	return int64(n), true
}

// javaScriptNumber formats as Number.prototype.toString: the shortest
// round-trip digits, positional between 1e-6 and 1e21, else an exponent
// without leading zeros.
func javaScriptNumber(number json.Number) (string, error) {
	f, err := strconv.ParseFloat(string(number), 64)
	if err != nil {
		return "", err
	}
	if f == 0 {
		return "0", nil
	}
	if abs := math.Abs(f); abs >= 1e-6 && abs < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64), nil
	}
	mantissa, exponent, _ := strings.Cut(strconv.FormatFloat(f, 'e', -1, 64), "e")
	sign, digits := exponent[:1], strings.TrimLeft(exponent[1:], "0")
	return mantissa + "e" + sign + digits, nil
}

// writeString escapes as JSON.stringify does: unlike encoding/json, it
// leaves <, >, &, U+2028 and U+2029 alone.
func writeString(out *bytes.Buffer, s string) {
	const hexDigits = "0123456789abcdef"
	out.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if c < 0x20 {
				out.WriteString(`\u00`)
				out.WriteByte(hexDigits[c>>4])
				out.WriteByte(hexDigits[c&0xf])
			} else {
				out.WriteByte(c)
			}
		}
	}
	out.WriteByte('"')
}
