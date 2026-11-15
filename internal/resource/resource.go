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
	FileParts []string
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
		group.AddCommand(NewCommand(f, method))
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
}

// NewCommand is the command for one method.
func NewCommand(f *cmdutil.Factory, method Method) *cobra.Command {
	in := &input{values: map[string]func() any{}, files: map[string]*string{}}
	command := &cobra.Command{
		Use:         method.Use,
		Short:       method.Short,
		Long:        method.Long,
		Args:        cobra.ExactArgs(len(method.PathParams)),
		Annotations: map[string]string{"method": method.Name},
		RunE: func(cmd *cobra.Command, args []string) error {
			in.flags = cmd.Flags()
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
			in.files[part] = flags.String(part, "", "path of the file to upload as "+part)
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
		flags.StringVarP(&in.output, "output", "o", "", "file to save the response to")
	}
	if method.Preview != "" {
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
	query := url.Values{}
	for _, flag := range method.Query {
		addQuery(query, flag.Path[0], in.value(flag))
	}
	if method.BinaryResponse {
		if in.output == "" {
			return &cmdutil.UsageError{Message: "use -o/--output <file> to choose where to save the download"}
		}
		return &cmdutil.UsageError{Message: "this version of tmh can't save downloads yet"}
	}
	var body []byte
	contentType := ""
	if method.Body != nil {
		if len(method.Body.FileParts) > 0 {
			return uploadError(method.Body, in)
		}
		contentType = method.Body.ContentType
		if body, err = jsonBody(f, method.Body, in); err != nil {
			return err
		}
	}
	client, err := apiClient(f, method.Public)
	if err != nil {
		return err
	}
	response, err := client.Do(cmd.Context(), api.Request{
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
	})
	if err != nil {
		return err
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

func jsonBody(f *cmdutil.Factory, body *Body, in *input) ([]byte, error) {
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
	return f.Body(in.data, body.ContentType, fields...)
}

func uploadError(body *Body, in *input) error {
	for _, part := range body.FileParts {
		if *in.files[part] == "" {
			return &cmdutil.UsageError{Message: fmt.Sprintf("use --%s <path> to choose the file to upload", part)}
		}
	}
	return &cmdutil.UsageError{Message: "this version of tmh can't upload files yet"}
}
