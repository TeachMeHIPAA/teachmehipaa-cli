package gen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/resource"
)

// SamplesPath is where the API reference's shell samples live, relative to
// the repository root. The monorepo vendors the file into its Scalar
// reference as the `cli` entry of every operation's sample picker.
const SamplesPath = "docs/reference/code-samples.json"

// BodyInput is a body field a request must carry. Body fields are never
// required cobra flags, since -d or --data may supply them, so the sample
// tests check them against this instead.
type BodyInput struct {
	// Flag is the field's flag, or "" when only -d or --data can set it.
	Flag string
	// Key is the field's wire path, dotted: "select.enrollmentIds".
	Key string
	// File marks a multipart file part, which only its flag can set.
	File bool
}

// Operation is one IR operationId as the sample tests see it.
type Operation struct {
	OperationID string
	// Method is the IR's `resource.method` the sample calls: the first of
	// the methods that share the operationId, as with enrollments.batch and
	// enrollments.batchByFilter.
	Method string
	// Required are the body fields the request must carry.
	Required []BodyInput
}

// sampleValues are the example values every SDK's samples use, so the six
// samples on one operation line up. Path parameters and required flags are
// looked up here by name.
var sampleValues = map[string]string{
	"learnerId":      "sub_01JD80AB2C3D4E5F6G7H8J9K0M",
	"subjectId":      "sub_01JD80AB2C3D4E5F6G7H8J9K0M",
	"enrollmentId":   "enr_01JD8K2M4N6P8Q0R2S4T6V8W0X",
	"certId":         "cert_01JD8F3G5H7J9K1M3N5P7Q9R1S",
	"courseId":       "hipaa-2026",
	"externalId":     "hr-1042",
	"operationId":    "op_01JD8T4V6W8X0Y2Z4A6B8C0D2E",
	"subscriptionId": "whs_01JD8Q5R7S9T1V3W5X7Y9Z1A3B",
	"tinyId":         "a7Kq2Z",

	"email":                 "dana.whitfield@example.com",
	"name":                  "Dana Whitfield",
	"endpoint-url":          "https://example.com/webhooks/teachmehipaa",
	"event-types":           "learner.certified",
	"format":                "csv",
	"operation":             "remind",
	"select.enrollment-ids": "enr_01JD8K2M4N6P8Q0R2S4T6V8W0X",
	"file":                  "roster.csv",
	// items has no flag; it goes in as JSON with -d.
	"items": `[{"externalId":"hr-1042","email":"dana.whitfield@example.com","name":"Dana Whitfield"}]`,
}

// sampleSpec is what a sample adds to the command the IR implies: at most
// one telling optional flag, and how it uses the result.
type sampleSpec struct {
	// flags are rendered as written, after the required ones.
	flags []string
	// use follows the command: a pipeline stage or an && list.
	use string
	// before is a line that runs first, such as reading an ETag.
	before string
}

// etagOf reads the ETag a conditional update needs from the headers that
// --show-headers prints on stderr, the one place tmh shows them.
func etagOf(command string) string {
	return "etag=$(" + command + " \\\n  --show-headers 2>&1 >/dev/null | sed -n 's/^< Etag: //p')"
}

func tsv(fields ...string) string {
	return `| jq -r '.items[] | [` + strings.Join(fields, ", ") + `] | @tsv'`
}

// sampleSpecs are keyed by operationId. An operation missing here still
// gets a sample, of its required inputs alone.
var sampleSpecs = map[string]sampleSpec{
	"listAuditEvents":       {flags: []string{"--type learner.updated"}, use: tsv(".at", ".type", ".actorSubjectId")},
	"introspectAccessToken": {use: "| jq -r '.scopes[]'"},

	"getCertificateVerification": {use: "| jq -r .status"},
	"downloadCertificatePdf":     {flags: []string{"-o certificate.pdf"}},
	"exportCertificates":         {flags: []string{"-o certificates.csv"}},
	"listCertificates":           {flags: []string{"--subject-id sub_01JD80AB2C3D4E5F6G7H8J9K0M"}, use: tsv(".certificateId", ".status", ".expiresAt")},
	"getCertificate":             {use: "| jq -r .expiresAt"},
	"getCertificateCredentials":  {use: "| jq -r .shortLink"},
	"getComplianceSummary":       {flags: []string{"--course-id hipaa-2026"}, use: "| jq .compliantRate"},

	"listCourses": {use: tsv(".courseId", ".title")},
	"getCourse":   {use: "| jq -r .title"},

	"batchEnrollments":       {use: "| jq -r '.results[] | [.enrollmentId, .outcome] | @tsv'"},
	"cancelEnrollment":       {flags: []string{"--refund"}, use: "| jq -r .status"},
	"createEnrollment":       {flags: []string{"--course-id hipaa-2026"}, use: "| jq -r .enrollmentId"},
	"exportEnrollments":      {flags: []string{"-o enrollments.csv"}},
	"importEnrollments":      {flags: []string{"--wait"}, use: "| jq .response"},
	"listEnrollments":        {flags: []string{"--status CERTIFIED"}, use: tsv(".enrollmentId", ".subjectId", ".expiresAt")},
	"remindEnrollment":       {use: "| jq -r .action"},
	"renewEnrollment":        {use: "| jq .cycle"},
	"resendEnrollmentInvite": {use: "| jq -r .action"},
	"getEnrollment":          {use: "| jq -r .effectiveStatus"},

	"listOrgEvents": {flags: []string{"--type learner.certified"}, use: tsv(".occurredAt", ".eventId", ".data.email")},

	"batchUpsertLearners":       {flags: []string{"--wait"}, use: "| jq .response"},
	"createLearner":             {flags: []string{`--name "Dana Whitfield"`}, use: "| jq -r .learnerId"},
	"createLearnerLaunchLink":   {use: "| jq -r .url"},
	"exportLearners":            {flags: []string{"--status expired", "-o learners.csv"}},
	"listLearners":              {flags: []string{"--status in_progress"}, use: tsv(".learnerId", ".email", ".status")},
	"offboardLearner":           {use: "| jq -r .status"},
	"remindLearner":             {use: "| jq -r .action"},
	"getLearner":                {use: "| jq -r .status"},
	"upsertLearnerByExternalId": {flags: []string{`--name "Dana Whitfield"`}, use: "| jq -r .learnerId"},
	"updateLearner": {
		before: etagOf("tmh learners retrieve sub_01JD80AB2C3D4E5F6G7H8J9K0M"),
		flags:  []string{`--name "Dana Whitfield"`, `--if-match "$etag"`},
		use:    "| jq -r .name",
	},

	"listMembers":                {flags: []string{"--role admin"}, use: tsv(".email", ".name")},
	"getMember":                  {use: "| jq -r .role"},
	"getOperation":               {use: "| jq .done"},
	"getSeatInventory":           {use: "| jq .available"},
	"listWebhookSubscriptions":   {use: tsv(".subscriptionId", ".endpointUrl", ".enabled")},
	"createWebhookSubscription":  {use: "| jq -r .secret"},
	"deleteWebhookSubscription":  {use: "&& echo deleted"},
	"getWebhookSubscription":     {use: "| jq -r '.eventTypes[]'"},
	"redriveWebhookSubscription": {flags: []string{"--wait"}, use: "| jq .response"},
	"testWebhookSubscription":    {flags: []string{"--wait"}, use: "| jq .response"},
	"updateWebhookSubscription": {
		before: etagOf("tmh webhook-subscriptions retrieve whs_01JD8Q5R7S9T1V3W5X7Y9Z1A3B"),
		flags:  []string{"--enabled=false", `--if-match "$etag"`},
		use:    "| jq .enabled",
	},
}

// sampleWidth is where a sample's command wraps onto continuation lines.
const sampleWidth = 72

// Operations lists every operationId in the IR with the method its sample
// calls and the body fields that method requires, in IR order.
func Operations(raw []byte) ([]Operation, error) {
	g, ir, err := newGenerator(raw)
	if err != nil {
		return nil, err
	}
	ops := []Operation{}
	for _, r := range ir.Resources {
		for _, m := range r.Methods {
			if slices.ContainsFunc(ops, func(op Operation) bool { return op.OperationID == m.OperationID }) {
				continue
			}
			op := Operation{OperationID: m.OperationID, Method: strings.Join(append(slices.Clone(r.Path), m.Name), ".")}
			if m.RequestBody != nil {
				_, help, err := g.body(*m.RequestBody)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", op.Method, err)
				}
				op.Required = help.required
			}
			ops = append(ops, op)
		}
	}
	return ops, nil
}

// Samples renders docs/reference/code-samples.json: for each operationId,
// the `tmh` command that calls it, as a reader would paste it into a shell.
func Samples(raw []byte) ([]byte, error) {
	g, ir, err := newGenerator(raw)
	if err != nil {
		return nil, err
	}
	samples := map[string]string{}
	for _, r := range ir.Resources {
		for _, m := range r.Methods {
			if _, done := samples[m.OperationID]; done {
				continue
			}
			sample, err := g.sample(r.Path, m)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", m.OperationID, err)
			}
			samples[m.OperationID] = sample
		}
	}
	file := struct {
		Language string            `json:"language"`
		Release  string            `json:"release"`
		Samples  map[string]string `json:"samples"`
	}{"cli", ir.Source.Release, samples}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	// Samples are shell: `>`, `&&` and `<` must stay readable in the file.
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(file); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func newGenerator(raw []byte) (*generator, *irDocument, error) {
	ir, err := decodeIR(raw)
	if err != nil {
		return nil, nil, err
	}
	g := &generator{types: map[string]irType{}}
	for _, t := range ir.Types {
		g.types[t.Name] = t
	}
	return g, ir, nil
}

func (g *generator) sample(resourcePath []string, m irMethod) (string, error) {
	method, err := g.method(resourcePath, m)
	if err != nil {
		return "", err
	}
	spec := sampleSpecs[m.OperationID]
	words := []string{commandPath(resourcePath), kebab(m.Name)}
	for _, p := range method.PathParams {
		value, ok := sampleValues[p]
		if !ok {
			return "", fmt.Errorf("no sample value for the path parameter %s", p)
		}
		words = append(words, value)
	}
	flags := []string{}
	for _, flag := range method.Query {
		if flag.Required {
			value, err := sampleValue(flag.Name, flag.Values)
			if err != nil {
				return "", err
			}
			flags = append(flags, "--"+flag.Name+" "+shellQuote(value))
		}
	}
	if m.RequestBody != nil {
		_, help, err := g.body(*m.RequestBody)
		if err != nil {
			return "", err
		}
		for _, input := range help.required {
			flag, err := requiredFlag(method.Body, input)
			if err != nil {
				return "", err
			}
			flags = append(flags, flag)
		}
	}
	flags = append(flags, spec.flags...)

	lines := []string{}
	if method.Public {
		lines = append(lines, "# Public: works before tmh login.")
	} else {
		lines = append(lines, "# Runs as your sandbox profile (tmh login). Add --live for Live.")
	}
	if spec.before != "" {
		lines = append(lines, spec.before)
	}
	command := strings.Join(append(words, flags...), " ")
	if spec.use != "" {
		command += " " + spec.use
	}
	if len(command) <= sampleWidth {
		return strings.Join(append(lines, command), "\n"), nil
	}
	// Too wide for one line: one flag per continuation line, the way
	// people write long commands in scripts.
	parts := append([]string{strings.Join(words, " ")}, flags...)
	if spec.use != "" {
		parts = append(parts, spec.use)
	}
	for i, part := range parts {
		if i > 0 {
			part = "  " + part
		}
		if i < len(parts)-1 {
			part += " \\"
		}
		lines = append(lines, part)
	}
	return strings.Join(lines, "\n"), nil
}

// requiredFlag is how a sample sets a required body field: its flag when
// it has one, or -d with the field as JSON.
func requiredFlag(body *resource.Body, input BodyInput) (string, error) {
	if input.Flag == "" {
		value, ok := sampleValues[input.Key]
		if !ok {
			return "", fmt.Errorf("no sample value for the required field %s", input.Key)
		}
		return "-d " + shellQuote(input.Key+":="+value), nil
	}
	var values []string
	if body != nil {
		for _, f := range body.Fields {
			if f.Name == input.Flag {
				values = f.Values
			}
		}
	}
	value, err := sampleValue(input.Flag, values)
	if err != nil {
		return "", err
	}
	return "--" + input.Flag + " " + shellQuote(value), nil
}

func sampleValue(flag string, values []string) (string, error) {
	if value, ok := sampleValues[flag]; ok {
		return value, nil
	}
	if len(values) > 0 {
		return values[0], nil
	}
	return "", fmt.Errorf("no sample value for the required flag --%s", flag)
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

func shellQuote(value string) string {
	switch {
	case shellSafe.MatchString(value):
		return value
	case !strings.ContainsAny(value, "\"$`\\"):
		return `"` + value + `"`
	case !strings.Contains(value, "'"):
		return "'" + value + "'"
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`").Replace(value) + `"`
}
