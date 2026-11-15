package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"strings"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
)

// rawField is one `-d key=value`; Value is a string, or a json.RawMessage
// for `key:=<json>`.
type rawField struct {
	Key   string
	Value any
}

// rawBody turns -d/--data values into a request body: at most one document
// (`@file`, `@-` for stdin, or literal JSON) with key=value fields set on top
// of it. It is nil when there is nothing to send.
func rawBody(f *cmdutil.Factory, values []string, contentType string) ([]byte, error) {
	var document []byte
	fields := []rawField{}
	for _, value := range values {
		next, field, err := parseDataValue(f, value)
		if err != nil {
			return nil, err
		}
		if field != nil {
			fields = append(fields, *field)
			continue
		}
		if document != nil {
			return nil, &cmdutil.UsageError{Message: "give at most one --data @file or JSON document"}
		}
		document = next
	}
	if !isJSONMediaType(contentType) {
		if len(fields) > 0 {
			return nil, &cmdutil.UsageError{Message: fmt.Sprintf("-d key=value builds JSON, so it can't be sent as %s", contentType)}
		}
		return document, nil
	}
	if document != nil && !json.Valid(document) {
		return nil, &cmdutil.UsageError{Message: "--data is not valid JSON"}
	}
	if len(fields) == 0 {
		return document, nil
	}
	return mergeFields(document, fields)
}

func parseDataValue(f *cmdutil.Factory, value string) ([]byte, *rawField, error) {
	if path, ok := strings.CutPrefix(value, "@"); ok {
		if path == "-" {
			all, err := f.ReadAll()
			return []byte(all), nil, err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, &cmdutil.UsageError{Message: fmt.Sprintf("--data: %v", err)}
		}
		return contents, nil, nil
	}
	if trimmed := strings.TrimSpace(value); strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return []byte(value), nil, nil
	}
	key, raw, ok := strings.Cut(value, "=")
	if !ok || key == "" || key == ":" {
		return nil, nil, &cmdutil.UsageError{Message: fmt.Sprintf("-d %q: use key=value, key:=<json>, @file or a JSON document", value)}
	}
	if key, ok := strings.CutSuffix(key, ":"); ok {
		if !json.Valid([]byte(raw)) {
			return nil, nil, &cmdutil.UsageError{Message: fmt.Sprintf("-d %q: the value after := is not valid JSON", value)}
		}
		return nil, &rawField{Key: key, Value: json.RawMessage(raw)}, nil
	}
	return nil, &rawField{Key: key, Value: raw}, nil
}

func mergeFields(document []byte, fields []rawField) ([]byte, error) {
	root := map[string]any{}
	if document != nil {
		decoder := json.NewDecoder(bytes.NewReader(document))
		// Numbers stay as written instead of passing through float64.
		decoder.UseNumber()
		var decoded any
		if err := decoder.Decode(&decoded); err != nil {
			return nil, &cmdutil.UsageError{Message: "--data is not valid JSON"}
		}
		object, ok := decoded.(map[string]any)
		if !ok {
			return nil, &cmdutil.UsageError{Message: "-d key=value needs --data to be a JSON object"}
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

// setField follows dotted keys into nested objects; a final `name[]`
// appends to an array.
func setField(root map[string]any, field rawField) error {
	conflict := &cmdutil.UsageError{Message: fmt.Sprintf("-d %s conflicts with another value for the same field", field.Key)}
	names := strings.Split(field.Key, ".")
	object := root
	for _, name := range names[:len(names)-1] {
		if name == "" || strings.HasSuffix(name, "[]") {
			return &cmdutil.UsageError{Message: fmt.Sprintf("-d %s: [] can only end a key, and dots need a name on each side", field.Key)}
		}
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
	last := names[len(names)-1]
	name, appends := strings.CutSuffix(last, "[]")
	if name == "" {
		return &cmdutil.UsageError{Message: fmt.Sprintf("-d %s: the key ends without a name", field.Key)}
	}
	if !appends {
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
