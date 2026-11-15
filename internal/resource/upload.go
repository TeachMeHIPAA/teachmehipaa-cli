package resource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"mime"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
)

// multipartBody builds a multipart/form-data body: one part per file flag,
// then one per top-level field, objects as JSON parts the way the API's
// `encoding` declares them.
func multipartBody(f *cmdutil.Factory, body *Body, in *input) ([]byte, string, error) {
	for _, part := range body.FileParts {
		if *in.files[part.Name] == "" {
			return nil, "", &cmdutil.UsageError{Message: fmt.Sprintf("use --%s <path> to choose the file to upload", part.Name)}
		}
	}
	fields := map[string]json.RawMessage{}
	if document, err := f.Body(nil, "application/json", bodyFields(body, in)...); err != nil {
		return nil, "", err
	} else if document != nil {
		if err := json.Unmarshal(document, &fields); err != nil {
			return nil, "", err
		}
	}
	var out bytes.Buffer
	writer := multipart.NewWriter(&out)
	for _, part := range body.FileParts {
		data, filename, err := readUpload(f, *in.files[part.Name], part.ContentType)
		if err != nil {
			return nil, "", err
		}
		if err := writePart(writer, part.Name, filename, part.ContentType, data); err != nil {
			return nil, "", err
		}
	}
	for _, name := range slices.Sorted(maps.Keys(fields)) {
		value := fields[name]
		contentType, data := "application/json", []byte(value)
		var text string
		if json.Unmarshal(value, &text) == nil {
			contentType, data = "text/plain; charset=utf-8", []byte(text)
		}
		if err := writePart(writer, name, "", contentType, data); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return out.Bytes(), writer.FormDataContentType(), nil
}

// readUpload reads a file flag's path, or stdin for "-". The API checks the
// filename's extension, so stdin is named for the part's media type.
func readUpload(f *cmdutil.Factory, path, contentType string) ([]byte, string, error) {
	if path == "-" {
		data, err := f.ReadAll()
		_, subtype, _ := strings.Cut(contentType, "/")
		return []byte(data), "stdin." + subtype, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", &cmdutil.UsageError{Message: fmt.Sprintf("reading the upload: %v", err)}
	}
	return data, filepath.Base(path), nil
}

func writePart(writer *multipart.Writer, name, filename, contentType string, data []byte) error {
	disposition := map[string]string{"name": name}
	if filename != "" {
		disposition["filename"] = filename
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", disposition))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = part.Write(data)
	return err
}
