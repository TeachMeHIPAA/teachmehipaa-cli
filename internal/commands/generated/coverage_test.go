package generated_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/commands/generated"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/gen"
)

const (
	specPath    = "../../../openapi/openapi.developer.json"
	irPath      = "../../../openapi/sdk-ir.json"
	versionPath = "../../../OPENAPI_VERSION"
)

// expectedCommands is the command for every Developer API operation, taken
// from the Developer API reference (the vendored openapi/openapi.developer.json)
// and written down independently of the generator, with the renames
// settled since (grilling Q12 `retrieveLinks`; the IR's `auth.introspect`
// and `delete`) and the Q25 splits, in tmh's kebab-case.
var expectedCommands = []struct {
	operationID string
	method      string
	command     string
}{
	{"listAuditEvents", "auditEvents.list", "audit-events list"},
	{"introspectAccessToken", "auth.introspect", "auth introspect"},
	{"getCertificateVerification", "certificateVerifications.retrieve", "certificate-verifications retrieve"},
	{"downloadCertificatePdf", "certificates.downloadPdf", "certificates download-pdf"},
	{"exportCertificates", "certificates.export", "certificates export"},
	{"listCertificates", "certificates.list", "certificates list"},
	{"getCertificate", "certificates.retrieve", "certificates retrieve"},
	{"getCertificateCredentials", "certificates.retrieveLinks", "certificates retrieve-links"},
	{"getComplianceSummary", "compliance.retrieve", "compliance retrieve"},
	{"listCourses", "courses.list", "courses list"},
	{"getCourse", "courses.retrieve", "courses retrieve"},
	{"batchEnrollments", "enrollments.batch", "enrollments batch"},
	{"batchEnrollments", "enrollments.batchByFilter", "enrollments batch-by-filter"},
	{"cancelEnrollment", "enrollments.cancel", "enrollments cancel"},
	{"createEnrollment", "enrollments.create", "enrollments create"},
	{"exportEnrollments", "enrollments.export", "enrollments export"},
	{"importEnrollments", "enrollments.import", "enrollments import"},
	{"importEnrollments", "enrollments.previewImport", "enrollments preview-import"},
	{"listEnrollments", "enrollments.list", "enrollments list"},
	{"remindEnrollment", "enrollments.remind", "enrollments remind"},
	{"renewEnrollment", "enrollments.renew", "enrollments renew"},
	{"resendEnrollmentInvite", "enrollments.resendInvite", "enrollments resend-invite"},
	{"getEnrollment", "enrollments.retrieve", "enrollments retrieve"},
	{"listOrgEvents", "events.list", "events list"},
	{"batchUpsertLearners", "learners.batchUpsert", "learners batch-upsert"},
	{"createLearner", "learners.create", "learners create"},
	{"createLearnerLaunchLink", "learners.createLaunchLink", "learners create-launch-link"},
	{"exportLearners", "learners.export", "learners export"},
	{"listLearners", "learners.list", "learners list"},
	{"offboardLearner", "learners.offboard", "learners offboard"},
	{"remindLearner", "learners.remind", "learners remind"},
	{"getLearner", "learners.retrieve", "learners retrieve"},
	{"updateLearner", "learners.update", "learners update"},
	{"upsertLearnerByExternalId", "learners.upsertByExternalId", "learners upsert-by-external-id"},
	{"listMembers", "members.list", "members list"},
	{"getMember", "members.retrieve", "members retrieve"},
	{"getOperation", "operations.retrieve", "operations retrieve"},
	{"getSeatInventory", "seats.retrieve", "seats retrieve"},
	{"createWebhookSubscription", "webhookSubscriptions.create", "webhook-subscriptions create"},
	{"deleteWebhookSubscription", "webhookSubscriptions.delete", "webhook-subscriptions delete"},
	{"listWebhookSubscriptions", "webhookSubscriptions.list", "webhook-subscriptions list"},
	{"redriveWebhookSubscription", "webhookSubscriptions.redrive", "webhook-subscriptions redrive"},
	{"getWebhookSubscription", "webhookSubscriptions.retrieve", "webhook-subscriptions retrieve"},
	{"testWebhookSubscription", "webhookSubscriptions.test", "webhook-subscriptions test"},
	{"updateWebhookSubscription", "webhookSubscriptions.update", "webhook-subscriptions update"},
}

// handWrittenOperations are implemented by the HTTP client, not a command.
var handWrittenOperations = []string{"createAccessToken"}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

type irDocument struct {
	Source struct {
		Release string `json:"release"`
		SHA256  string `json:"sha256"`
	} `json:"source"`
	Resources []struct {
		Path    []string `json:"path"`
		Methods []struct {
			Name        string `json:"name"`
			OperationID string `json:"operationId"`
		} `json:"methods"`
	} `json:"resources"`
}

func readIR(t *testing.T) irDocument {
	t.Helper()
	var ir irDocument
	if err := json.Unmarshal(readFile(t, irPath), &ir); err != nil {
		t.Fatal(err)
	}
	return ir
}

func TestTheIRIsBuiltFromTheVendoredSpec(t *testing.T) {
	ir := readIR(t)
	sum := sha256.Sum256(readFile(t, specPath))
	if got := hex.EncodeToString(sum[:]); got != ir.Source.SHA256 {
		t.Errorf("spec SHA-256 = %s, IR source.sha256 = %s", got, ir.Source.SHA256)
	}
	if got := strings.TrimSpace(string(readFile(t, versionPath))); got != ir.Source.Release {
		t.Errorf("OPENAPI_VERSION = %q, IR source.release = %q", got, ir.Source.Release)
	}
}

func TestEverySpecOperationHasACommandExceptTheTokenEndpoint(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(readFile(t, specPath), &spec); err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, item := range spec.Paths {
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			if raw, ok := item[method]; ok {
				var operation struct {
					OperationID string `json:"operationId"`
				}
				if err := json.Unmarshal(raw, &operation); err != nil {
					t.Fatal(err)
				}
				if !slices.Contains(handWrittenOperations, operation.OperationID) {
					got = append(got, operation.OperationID)
				}
			}
		}
	}
	want := []string{}
	for _, c := range expectedCommands {
		if !slices.Contains(want, c.operationID) {
			want = append(want, c.operationID)
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("spec operations = %v\nwant %v", got, want)
	}
}

func TestEveryIRMethodIsInTheTable(t *testing.T) {
	got := []string{}
	for _, resource := range readIR(t).Resources {
		for _, method := range resource.Methods {
			got = append(got, strings.Join(append(slices.Clone(resource.Path), method.Name), ".")+" "+method.OperationID)
		}
	}
	want := []string{}
	for _, c := range expectedCommands {
		want = append(want, c.method+" "+c.operationID)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("IR methods = %v\nwant %v", got, want)
	}
}

// leafCommands maps each runnable command's path below root to the IR
// method it says it runs.
func leafCommands(command *cobra.Command, prefix string, into map[string]string) {
	for _, child := range command.Commands() {
		path := strings.TrimSpace(prefix + " " + child.Name())
		if child.Runnable() {
			into[path] = child.Annotations["method"]
		}
		leafCommands(child, path, into)
	}
}

func TestEveryIRMethodMapsToExactlyOneCommand(t *testing.T) {
	root := &cobra.Command{Use: "tmh"}
	generated.Register(root, cmdutil.NewFactory(cmdutil.Deps{}))
	got := map[string]string{}
	leafCommands(root, "", got)

	want := map[string]string{}
	for _, c := range expectedCommands {
		want[c.command] = c.method
	}
	for command, method := range want {
		if got[command] != method {
			t.Errorf("tmh %s runs %q, want %q", command, got[command], method)
		}
	}
	for command := range got {
		if _, ok := want[command]; !ok {
			t.Errorf("unexpected command tmh %s", command)
		}
	}
}

func TestTheCommittedCommandsMatchAFreshRun(t *testing.T) {
	files, err := gen.Generate(readFile(t, irPath))
	if err != nil {
		t.Fatal(err)
	}
	stale, err := gen.Stale(".", files)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) > 0 {
		t.Errorf("generated files out of date: %v; run make generate", stale)
	}
}
