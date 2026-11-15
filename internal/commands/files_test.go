package commands_test

import (
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

const (
	certID      = "cert_01JD85"
	pdfRoute    = "GET /v1/organizations/" + testkit.OrgID + "/certificates/" + certID + "/content"
	importPath  = "/v1/organizations/" + testkit.OrgID + "/enrollments:import"
	importRoute = "POST " + importPath
)

// everyByte is 0x00 through 0xFF, which a text decode would corrupt.
func everyByte() string {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return string(b)
}

func pdfReply(body string) testkit.Reply {
	return testkit.Reply{Status: 200, Header: map[string]string{"Content-Type": "application/pdf"}, Body: body}
}

func TestDownloadPDFWritesTheBytesUnchanged(t *testing.T) {
	out := filepath.Join(t.TempDir(), "cert.pdf")
	env := loggedInByEnvironment(t)
	env.Transport.Script(pdfRoute, pdfReply(everyByte()))

	result := env.Run("certificates", "download-pdf", certID, "-o", out)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != everyByte() {
		t.Errorf("file = %x, want bytes 00–ff", written)
	}
	assertHeader(t, env.Transport.Requests()[2], "Accept", "application/pdf, application/problem+json")
	if result.Stdout != "" {
		t.Errorf("stdout = %q, want nothing", result.Stdout)
	}
	if !strings.Contains(result.Stderr, out) || !strings.Contains(result.Stderr, "256 bytes") {
		t.Errorf("stderr = %q, want the file name and size", result.Stderr)
	}
}

func TestDownloadPDFToAPipedStdout(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(pdfRoute, pdfReply(everyByte()))

	result := env.Run("certificates", "download-pdf", certID, "-o", "-")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if result.Stdout != everyByte() {
		t.Errorf("stdout = %x, want bytes 00–ff", result.Stdout)
	}
}

func TestDownloadPDFRefusesToPrintToATerminal(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.StdoutIsTTY = true

	result := env.Run("certificates", "download-pdf", certID, "-o", "-")

	if result.ExitCode != 1 || !strings.Contains(result.Stderr, "terminal") {
		t.Errorf("exit code = %d, stderr = %q; want 1 refusing the terminal", result.ExitCode, result.Stderr)
	}
	if result.Stdout != "" {
		t.Errorf("stdout = %q, want nothing", result.Stdout)
	}
	if routes := env.Transport.Routes(); len(routes) != 0 {
		t.Errorf("sent %v, want no requests", routes)
	}
}

func TestDownloadPDFRendersAProblemAndWritesNothing(t *testing.T) {
	out := filepath.Join(t.TempDir(), "cert.pdf")
	env := loggedInByEnvironment(t)
	env.Transport.Script(pdfRoute, testkit.Reply{
		Status: 404,
		Header: map[string]string{"Content-Type": "application/problem+json", "X-Request-Id": "req_1"},
		Body:   `{"code":"not_found","title":"Not found","status":404,"detail":"No certificate cert_01JD85."}`,
	})

	result := env.Run("certificates", "download-pdf", certID, "-o", out)

	if result.ExitCode != 3 {
		t.Errorf("exit code = %d, want 3", result.ExitCode)
	}
	if want := "Error: Not found: No certificate cert_01JD85.\nRequest ID: req_1\n"; result.Stderr != want {
		t.Errorf("stderr = %q, want %q", result.Stderr, want)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("%s exists, want no file", out)
	}
}

func TestDownloadPDFRejectsAnHTMLPage(t *testing.T) {
	out := filepath.Join(t.TempDir(), "cert.pdf")
	env := loggedInByEnvironment(t)
	env.Transport.Script(pdfRoute, testkit.Reply{Status: 200, Header: map[string]string{"Content-Type": "text/html"}, Body: "<html>"})

	result := env.Run("certificates", "download-pdf", certID, "-o", out)

	if result.ExitCode != 7 {
		t.Errorf("exit code = %d, want 7; stderr = %q", result.ExitCode, result.Stderr)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("%s exists, want no file", out)
	}
}

func TestRawGetSavesANonJSONResponseWithOutput(t *testing.T) {
	out := filepath.Join(t.TempDir(), "cert.pdf")
	env := loggedInByEnvironment(t)
	env.Transport.Script(pdfRoute, pdfReply(everyByte()))

	result := env.Run("get", "/organizations/{orgId}/certificates/"+certID+"/content", "-o", out)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != everyByte() {
		t.Errorf("file = %x, want bytes 00–ff", written)
	}
	assertHeader(t, env.Transport.Requests()[2], "Accept", "*/*")
}

func TestRawGetRefusesToPrintAFileToATerminal(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.StdoutIsTTY = true

	result := env.Run("get", "/organizations/{orgId}/certificates/"+certID+"/content", "-o", "-")

	if result.ExitCode != 1 || !strings.Contains(result.Stderr, "terminal") {
		t.Errorf("exit code = %d, stderr = %q; want 1 refusing the terminal", result.ExitCode, result.Stderr)
	}
	if routes := env.Transport.Routes(); len(routes) != 0 {
		t.Errorf("sent %v, want no requests", routes)
	}
}

const rosterCSV = "email,name\r\nana@example.com,Ana Example\r\n"

type part struct {
	name, filename, contentType, body string
}

// parts reads a multipart/form-data request body.
func parts(t *testing.T, r testkit.Request) []part {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		t.Fatalf("Content-Type = %q, want multipart/form-data", r.Header.Get("Content-Type"))
	}
	reader := multipart.NewReader(strings.NewReader(r.Body), params["boundary"])
	out := []part{}
	for {
		p, err := reader.NextPart()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(p)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, part{p.FormName(), p.FileName(), p.Header.Get("Content-Type"), string(body)})
	}
}

func writeRoster(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "roster.csv")
	if err := os.WriteFile(path, []byte(rosterCSV), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImportSendsTheCSVAndOptionsAsMultipart(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(importRoute, accepted(`{"name":"`+operationName+`","done":true,"response":{"invited":1}}`))

	result := env.Run("enrollments", "import", "--file", writeRoster(t), "--options.course-id", "hipaa-2026")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	request := env.Transport.Requests()[2]
	want := []part{
		{"file", "roster.csv", "text/csv", rosterCSV},
		{"options", "", "application/json", `{"courseId":"hipaa-2026","validateOnly":false}`},
	}
	if got := parts(t, request); !slices.Equal(got, want) {
		t.Errorf("parts = %q\nwant    %q", got, want)
	}
	if !uuidV4.MatchString(request.Header.Get("Idempotency-Key")) {
		t.Errorf("Idempotency-Key = %q, want a generated UUIDv4", request.Header.Get("Idempotency-Key"))
	}
}

func TestImportResendsTheSameBytesAndKeyOnRetry(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(importRoute, unavailable(), accepted(`{"name":"`+operationName+`","done":true}`))

	result := env.Run("enrollments", "import", "--file", writeRoster(t))

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	keys := idempotencyKeys(env, importRoute)
	if len(keys) != 2 || !uuidV4.MatchString(keys[0]) || keys[1] != keys[0] {
		t.Errorf("Idempotency-Keys = %q, want one UUIDv4 on both attempts", keys)
	}
	requests := env.Transport.Requests()
	if requests[2].Body != requests[3].Body || requests[2].Header.Get("Content-Type") != requests[3].Header.Get("Content-Type") {
		t.Errorf("the retry sent a different body")
	}
}

func TestImportPreviewSendsValidateOnlyAndPrintsThePreview(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(importRoute, testkit.JSON(`{"rows":1,"errors":[]}`))

	result := env.Run("enrollments", "import", "--file", writeRoster(t), "--preview")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	got := parts(t, env.Transport.Requests()[2])
	if len(got) != 2 || got[1].body != `{"validateOnly":true}` {
		t.Errorf("parts = %q, want options {\"validateOnly\":true}", got)
	}
	if result.Stdout != `{"rows":1,"errors":[]}`+"\n" {
		t.Errorf("stdout = %q, want the preview", result.Stdout)
	}
}

func TestImportReadsTheCSVFromStdin(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Stdin = rosterCSV
	env.Transport.Script(importRoute, testkit.JSON(`{"rows":1}`))

	result := env.Run("enrollments", "preview-import", "--file", "-")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	got := parts(t, env.Transport.Requests()[2])
	if want := (part{"file", "stdin.csv", "text/csv", rosterCSV}); len(got) == 0 || got[0] != want {
		t.Errorf("parts = %q, want first %q", got, want)
	}
}

func TestImportPreviewCannotWait(t *testing.T) {
	env := loggedInByEnvironment(t)

	result := env.Run("enrollments", "import", "--file", writeRoster(t), "--preview", "--wait")

	if result.ExitCode != 1 || !strings.Contains(result.Stderr, "--wait") {
		t.Errorf("exit code = %d, stderr = %q; want 1 naming --wait", result.ExitCode, result.Stderr)
	}
	if routes := env.Transport.Routes(); len(routes) != 0 {
		t.Errorf("sent %v, want no requests", routes)
	}
}
