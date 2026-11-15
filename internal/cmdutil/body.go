package cmdutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"strings"
)

// DataFlagUsage is the help for -d/--data, which every command that sends a
// body reads with Body.
const DataFlagUsage = "body field as key=value (dotted keys nest, key[]=v appends, key:=<json> sends JSON), " +
	"or the whole body as @file, @- for stdin, or a JSON document; repeatable"

// IfMatchFlagUsage is the help for --if-match on every command that can
// write conditionally.
const IfMatchFlagUsage = "send If-Match with this ETag, for a conditional write"

// BodyField is one value placed in a JSON body. Path runs through nested
// objects to the field; Value is a string, number, bool, slice, or a
// json.RawMessage for `key:=<json>`.
type BodyField struct {
	// Source is how the caller wrote it (`-d a.b=c`, `--payment.method`), for
	// error messages.
	Source string
	Path   []string
	// Append adds Value to an array instead of replacing the field.
	Append bool
	Value  any
}

// Body turns -d/--data values into a request body: at most one document
// (`@file`, `@-` for stdin, or literal JSON), then each `-d key=value` set on
// top of it in order, then fields on top of those. It is nil when there is
// nothing to send.
func (f *Factory) Body(data []string, contentType string, fields ...BodyField) ([]byte, error) {
	var document []byte
	dataFields := []BodyField{}
	for _, value := range data {
		next, field, err := f.parseDataValue(value)
		if err != nil {
			return nil, err
		}
		if field != nil {
			dataFields = append(dataFields, *field)
			continue
		}
		if document != nil {
			return nil, &UsageError{Message: "give at most one --data @file or JSON document"}
		}
		document = next
	}
	fields = append(dataFields, fields...)
	if !isJSONMediaType(contentType) {
		if len(fields) > 0 {
			return nil, &UsageError{Message: fmt.Sprintf("%s builds JSON, so it can't be sent as %s", fields[0].Source, contentType)}
		}
		return document, nil
	}
	if document != nil && !json.Valid(document) {
		return nil, &UsageError{Message: "--data is not valid JSON"}
	}
	if len(fields) == 0 {
		return document, nil
	}
	return mergeFields(document, fields)
}

func (f *Factory) parseDataValue(value string) ([]byte, *BodyField, error) {
	if path, ok := strings.CutPrefix(value, "@"); ok {
		if path == "-" {
			all, err := f.ReadAll()
			return []byte(all), nil, err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, &UsageError{Message: fmt.Sprintf("--data: %v", err)}
		}
		return contents, nil, nil
	}
	if trimmed := strings.TrimSpace(value); strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return []byte(value), nil, nil
	}
	key, raw, ok := strings.Cut(value, "=")
	if !ok || key == "" || key == ":" {
		return nil, nil, &UsageError{Message: fmt.Sprintf("-d %q: use key=value, key:=<json>, @file or a JSON document", value)}
	}
	var fieldValue any = raw
	if trimmed, ok := strings.CutSuffix(key, ":"); ok {
		if !json.Valid([]byte(raw)) {
			return nil, nil, &UsageError{Message: fmt.Sprintf("-d %q: the value after := is not valid JSON", value)}
		}
		key, fieldValue = trimmed, json.RawMessage(raw)
	}
	field, err := dottedField(key, fieldValue)
	return nil, field, err
}

// dottedField follows dotted keys into nested objects; a final `name[]`
// appends to an array.
func dottedField(key string, value any) (*BodyField, error) {
	source := "-d " + key
	names := strings.Split(key, ".")
	for _, name := range names[:len(names)-1] {
		if name == "" || strings.HasSuffix(name, "[]") {
			return nil, &UsageError{Message: fmt.Sprintf("%s: [] can only end a key, and dots need a name on each side", source)}
		}
	}
	last, appends := strings.CutSuffix(names[len(names)-1], "[]")
	if last == "" {
		return nil, &UsageError{Message: fmt.Sprintf("%s: the key ends without a name", source)}
	}
	names[len(names)-1] = last
	return &BodyField{Source: source, Path: names, Append: appends, Value: value}, nil
}

func mergeFields(document []byte, fields []BodyField) ([]byte, error) {
	root := map[string]any{}
	if document != nil {
		decoder := json.NewDecoder(bytes.NewReader(document))
		// Numbers stay as written instead of passing through float64.
		decoder.UseNumber()
		var decoded any
		if err := decoder.Decode(&decoded); err != nil {
			return nil, &UsageError{Message: "--data is not valid JSON"}
		}
		object, ok := decoded.(map[string]any)
		if !ok {
			return nil, &UsageError{Message: fmt.Sprintf("%s needs --data to be a JSON object", fields[0].Source)}
		}
		root = object
	}
	for _, field := range fields {
		if err := setField(root, field); err != nil {
			return nil, err
		}
	}
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(root); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(body.Bytes(), []byte("\n")), nil
}

func setField(root map[string]any, field BodyField) error {
	conflict := &UsageError{Message: fmt.Sprintf("%s conflicts with another value for the same field", field.Source)}
	object := root
	for _, name := range field.Path[:len(field.Path)-1] {
		switch child := object[name].(type) {
		case nil:
			next := map[string]any{}
			object[name] = next
			object = next
		case map[string]any:
			object = child
		default:
			return conflict
		}
	}
	name := field.Path[len(field.Path)-1]
	if !field.Append {
		object[name] = field.Value
		return nil
	}
	switch list := object[name].(type) {
	case nil:
		object[name] = []any{field.Value}
	case []any:
		object[name] = append(list, field.Value)
	default:
		return conflict
	}
	return nil
}

func isJSONMediaType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json" ||
		(strings.HasPrefix(mediaType, "application/") && strings.HasSuffix(mediaType, "+json"))
}
