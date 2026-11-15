// Package resource runs the generated resource commands. The generator turns
// each IR method into a Method spec; this package turns a spec into a cobra
// command whose flags become the query and body, sends it with the request
// flags the IR sets, and prints the response like every other command.
package resource

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/api"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
)

// Method is one IR method as a command.
type Method struct {
	// Name is the IR's `resource.method`, e.g. "partners.organizations.list".
	Name  string
	Use   string
	Short string
	Long  string

	HTTPMethod string
	// Path is relative to the base URL, after the /organizations/{orgId}
	// prefix when OrganizationScoped, with a {name} for each of PathParams
	// and any `:verb` literal.
	Path               string
	PathParams         []string
	OrganizationScoped bool
	// Public operations send no Access token.
	Public    bool
	RetrySafe bool
	// IdempotencyKeyRequired generates a key when --idempotency-key is unset.
	IdempotencyKeyRequired bool
	IfMatch                Requirement

	Query []Flag
	// Body is nil when the operation takes none.
	Body *Body
	// BinaryResponse is set when the response is a file, not JSON.
	BinaryResponse bool
	// Accept is the Accept header for a BinaryResponse, naming the file's
	// media type and problem+json for errors.
	Accept string
	// Pagination is set on list methods, which get --all and --ndjson.
	Pagination *Pagination
	// ReturnsOperation is set when the response is an Operation, which
	// gets --wait.
	ReturnsOperation bool
	// Export is set on Operations whose response holds a file, which get
	// -o/--output.
	Export bool
	// Preview is the command a --preview flag stands for, e.g.
	// "enrollments preview-import".
	Preview string
}

// Body is the request body a method sends.
type Body struct {
	ContentType string
	Fields      []Flag
	// Fixed values are set by the command itself and win over every flag:
	// the IR's const constraints, which split one operation into two
	// methods (grilling Q25).
	Fixed []Fixed
	// FileParts are the multipart parts that carry a file, one --<part>
	// flag each.
	FileParts []FilePart
}

// FilePart is a multipart part read from a file.
type FilePart struct {
	Name        string
	ContentType string
}

// Pagination is how a list method pages.
type Pagination struct {
	// Param is the query parameter that asks for the next page.
	Param string
	// ItemID, when set, is the item field whose last value is the next
	// page's Param, as the event log's `after` is; otherwise the page's
	// nextCursor is.
	ItemID string
}

// Fixed is a body field with a value the caller can't change.
type Fixed struct {
	Path []string
	// JSON is the value as a JSON literal.
	JSON string
}

// Requirement is whether a method takes a header such as If-Match.
type Requirement int

const (
	None Requirement = iota
	Optional
	Required
)

// FlagType is how a flag's value is parsed and sent.
type FlagType int

const (
	String FlagType = iota
	Int
	Number
	Bool
	// StringList is repeatable and comma-separated.
	StringList
)

// Flag is a query parameter or body field set from the command line.
type Flag struct {
	// Name is kebab-case, dotted for nested body fields: "payment.return-url".
	Name string
	// Path is the wire name, through nested objects for body fields.
	Path  []string
	Type  FlagType
	Usage string
	// Values are the known values of an open enum, offered for completion.
	Values []string
	// Required makes the command refuse to run without the flag. Body
	// fields never set it, since -d or --data may supply them.
	Required bool
}

// NewGroup is `tmh <name>` holding one command per method, plus any nested
// resource groups.
func NewGroup(f *cmdutil.Factory, name, short string, methods []Method, groups ...*cobra.Command) *cobra.Command {
	group := &cobra.Command{Use: name, Short: short}
	for _, method := range methods {
		group.AddCommand(newCommand(f, method, previewOf(method, methods)))
	}
	group.AddCommand(groups...)
	return group
}

// input holds what the command line set, read once the flags are parsed.
type input struct {
	flags   *pflag.FlagSet
	values  map[string]func() any
	data    []string
	ifMatch string
	output  string
	files   map[string]*string
	preview bool
	all     bool
	ndjson  bool
	wait    bool
	// waitTimeout bounds --wait.
	waitTimeout time.Duration
}

// previewOf is the method a --preview flag switches to: the sibling command
// Method.Preview names.
func previewOf(method Method, siblings []Method) *Method {
	if method.Preview == "" {
		return nil
	}
	words := strings.Fields(method.Preview)
	for i := range siblings {
		if strings.Fields(siblings[i].Use)[0] == words[len(words)-1] {
			return &siblings[i]
		}
	}
	return nil
}

// newCommand is the command for one method. With --preview it sends
// preview instead, which takes the same flags.
func newCommand(f *cmdutil.Factory, method Method, preview *Method) *cobra.Command {
	in := &input{values: map[string]func() any{}, files: map[string]*string{}}
	command := &cobra.Command{
		Use:         method.Use,
		Short:       method.Short,
		Long:        method.Long,
		Args:        cobra.ExactArgs(len(method.PathParams)),
		Annotations: map[string]string{"method": method.Name},
		RunE: func(cmd *cobra.Command, args []string) error {
			in.flags = cmd.Flags()
			if in.preview {
				if in.wait {
					return &cmdutil.UsageError{Message: "--wait doesn't apply to --preview, which answers at once"}
				}
				return run(cmd, f, *preview, in, args)
			}
			return run(cmd, f, method, in, args)
		},
	}
	flags := command.Flags()
	for _, flag := range method.Query {
		in.bind(command, flag)
		if flag.Required {
			_ = command.MarkFlagRequired(flag.Name)
		}
	}
	if body := method.Body; body != nil {
		for _, flag := range body.Fields {
			in.bind(command, flag)
		}
		for _, part := range body.FileParts {
			in.files[part.Name] = flags.String(part.Name, "", "path of the file to upload as "+part.Name+", or - for stdin")
		}
		if len(body.FileParts) == 0 {
			flags.StringArrayVarP(&in.data, "data", "d", nil,
				"body field as key=value (dotted keys nest, key[]=v appends, key:=<json> sends JSON), "+
					"or the whole body as @file, @- for stdin, or a JSON document; repeatable. "+
					"Flags override -d fields, which override the document")
		}
	}
	if method.IfMatch != None {
		flags.StringVar(&in.ifMatch, "if-match", "", "send If-Match with this ETag, for a conditional write")
		if method.IfMatch == Required {
			_ = command.MarkFlagRequired("if-match")
		}
	}
	if method.BinaryResponse {
		flags.StringVarP(&in.output, "output", "o", "", "file to save the response to, or - for stdout when it is piped")
	}
	if method.ReturnsOperation {
		flags.BoolVar(&in.wait, "wait", false, "wait until the Operation is done and print it; a failed Operation exits 7")
		flags.DurationVar(&in.waitTimeout, "wait-timeout", 5*time.Minute, "how long --wait waits before giving up")
	}
	if method.Export {
		flags.StringVarP(&in.output, "output", "o", "", "save the export's CSV to this file, or - for stdout, once it is done (implies --wait)")
	}
	if method.Pagination != nil {
		flags.BoolVar(&in.all, "all", false, "follow every page and print all the items as one JSON array; --limit sets the page size")
		flags.BoolVar(&in.ndjson, "ndjson", false, "with --all, print one compact JSON item per line as each page arrives")
	}
	if preview != nil {
		flags.BoolVar(&in.preview, "preview", false, "check the upload without committing it, as `tmh "+method.Preview+"` does")
	}
	return command
}

func (in *input) bind(command *cobra.Command, flag Flag) {
	flags := command.Flags()
	switch flag.Type {
	case Int:
		value := flags.Int64(flag.Name, 0, flag.Usage)
		in.values[flag.Name] = func() any { return *value }
	case Number:
		value := flags.Float64(flag.Name, 0, flag.Usage)
		in.values[flag.Name] = func() any { return *value }
	case Bool:
		value := flags.Bool(flag.Name, false, flag.Usage)
		in.values[flag.Name] = func() any { return *value }
	case StringList:
		value := flags.StringSlice(flag.Name, nil, flag.Usage)
		in.values[flag.Name] = func() any { return *value }
	default:
		value := flags.String(flag.Name, "", flag.Usage)
		in.values[flag.Name] = func() any { return *value }
	}
	if len(flag.Values) > 0 {
		_ = command.RegisterFlagCompletionFunc(flag.Name, cobra.FixedCompletions(flag.Values, cobra.ShellCompDirectiveNoFileComp))
	}
}

// value is the flag's value, or nil when the command line didn't set it.
func (in *input) value(flag Flag) any {
	if !in.flags.Changed(flag.Name) {
		return nil
	}
	return in.values[flag.Name]()
}

func run(cmd *cobra.Command, f *cmdutil.Factory, method Method, in *input, args []string) error {
	path, err := expandPath(method, args)
	if err != nil {
		return err
	}
	if in.ndjson && !in.all {
		return &cmdutil.UsageError{Message: "--ndjson needs --all"}
	}
	if method.ReturnsOperation && in.waitTimeout <= 0 {
		return &cmdutil.UsageError{Message: "--wait-timeout must be more than 0"}
	}
	query := url.Values{}
	for _, flag := range method.Query {
		addQuery(query, flag.Path[0], in.value(flag))
	}
	if method.BinaryResponse {
		if in.output == "" {
			return &cmdutil.UsageError{Message: "use -o/--output <file> to choose where to save the download"}
		}
		if err := f.CheckOutput(in.output, true); err != nil {
			return err
		}
	}
	var body []byte
	contentType := ""
	switch {
	case method.Body == nil:
	case len(method.Body.FileParts) > 0:
		// Read once, so every retry resends the same bytes.
		if body, contentType, err = multipartBody(f, method.Body, in); err != nil {
			return err
		}
	default:
		contentType = method.Body.ContentType
		if body, err = jsonBody(f, method.Body, in); err != nil {
			return err
		}
	}
	client, err := apiClient(f, method.Public)
	if err != nil {
		return err
	}
	request := api.Request{
		Method:             method.HTTPMethod,
		Path:               path,
		OrganizationScoped: method.OrganizationScoped,
		Query:              query,
		RetrySafe:          method.RetrySafe,
		IdempotencyKey:     f.IdempotencyKey(method.IdempotencyKeyRequired),
		Body:               body,
		ContentType:        contentType,
		IfMatch:            in.ifMatch,
		Public:             method.Public,
		Accept:             method.Accept,
	}
	if in.all {
		return listAll(cmd.Context(), client, request, method.Pagination, f.Items(in.ndjson))
	}
	response, err := client.Do(cmd.Context(), request)
	if err != nil {
		return err
	}
	if method.BinaryResponse {
		return f.SaveDownload(in.output, response.Body)
	}
	if method.ReturnsOperation && (in.wait || in.output != "") {
		return waitForOperation(cmd.Context(), f, client, response.Body, in)
	}
	return f.PrintJSON(response.Body)
}

func apiClient(f *cmdutil.Factory, public bool) (*api.Client, error) {
	if public {
		return f.PublicAPIClient()
	}
	client, _, err := f.APIClient()
	return client, err
}

// expandPath escapes each id in one pass, so an id can't inject another
// placeholder or a path segment.
func expandPath(method Method, args []string) (string, error) {
	replacements := []string{}
	for i, name := range method.PathParams {
		if args[i] == "" {
			return "", &cmdutil.UsageError{Message: fmt.Sprintf("<%s> can't be empty", name)}
		}
		replacements = append(replacements, "{"+name+"}", url.PathEscape(args[i]))
	}
	return strings.NewReplacer(replacements...).Replace(method.Path), nil
}

func addQuery(query url.Values, name string, value any) {
	switch value := value.(type) {
	case string:
		query.Add(name, value)
	case int64:
		query.Add(name, strconv.FormatInt(value, 10))
	case float64:
		query.Add(name, strconv.FormatFloat(value, 'f', -1, 64))
	case bool:
		query.Add(name, strconv.FormatBool(value))
	case []string:
		for _, item := range value {
			query.Add(name, item)
		}
	}
}

func bodyFields(body *Body, in *input) []cmdutil.BodyField {
	fields := []cmdutil.BodyField{}
	for _, flag := range body.Fields {
		if value := in.value(flag); value != nil {
			fields = append(fields, cmdutil.BodyField{Source: "--" + flag.Name, Path: flag.Path, Value: value})
		}
	}
	for _, fixed := range body.Fixed {
		fields = append(fields, cmdutil.BodyField{
			Source: "the command's own " + strings.Join(fixed.Path, "."),
			Path:   fixed.Path,
			Value:  json.RawMessage(fixed.JSON),
		})
	}
	return fields
}

func jsonBody(f *cmdutil.Factory, body *Body, in *input) ([]byte, error) {
	return f.Body(in.data, body.ContentType, bodyFields(body, in)...)
}
