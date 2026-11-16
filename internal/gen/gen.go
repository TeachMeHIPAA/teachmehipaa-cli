// Package gen writes the resource commands in internal/commands/generated
// from openapi/sdk-ir.json and nothing else (ADR-0001). It fails on anything
// in the IR it can't express as a command instead of dropping it.
package gen

import (
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"unicode"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/resource"
)

const orgPrefix = "/organizations/{orgId}"

// globalFlags are bound on the root command; a generated flag must not
// shadow one.
var globalFlags = []string{"help", "profile", "live", "max-retries", "idempotency-key", "verbose", "show-headers"}

// GlobalFlags are the root command's persistent flags, plus --help, which a
// generated flag must not shadow.
func GlobalFlags() []string { return slices.Clone(globalFlags) }

// RootCommands are the root command's hand-written subcommands, which a
// generated resource group must not collide with.
func RootCommands() []string { return slices.Clone(rootCommands) }

// afterItemID is the item field an `after`-style list resumes from. The IR
// names only the parameter; the event log is the one such list, and its
// `after` is the last Event's id.
const afterItemID = "eventId"

// binaryAcceptSuffix also names problem+json, so a failed download still explains
// itself.
const binaryAcceptSuffix = ", application/problem+json"

// rootCommands are the hand-written commands a resource group must not
// collide with.
var rootCommands = []string{"completion", "delete", "get", "help", "login", "logout", "open", "patch", "post", "put", "version", "whoami"}

// previews are CLI-only switches (CLI spec story 19): the method that gets a
// --preview flag, and the sibling Q25 split it stands for.
var previews = map[string]string{"enrollments.import": "enrollments.previewImport"}

// group is `tmh <path...>`: its methods and nested resources.
type group struct {
	path     []string
	methods  []resource.Method
	children []*group
}

type generator struct {
	types map[string]irType
}

// Generate returns every generated file, keyed by file name.
func Generate(raw []byte) (map[string][]byte, error) {
	ir, err := decodeIR(raw)
	if err != nil {
		return nil, err
	}
	g := &generator{types: map[string]irType{}}
	for _, t := range ir.Types {
		g.types[t.Name] = t
	}
	roots, err := g.groups(ir.Resources)
	if err != nil {
		return nil, err
	}
	return render(roots)
}

func (g *generator) groups(resources []irResource) ([]*group, error) {
	roots := []*group{}
	byPath := map[string]*group{}
	var find func(path []string) *group
	find = func(path []string) *group {
		key := strings.Join(path, ".")
		if existing, ok := byPath[key]; ok {
			return existing
		}
		created := &group{path: path}
		byPath[key] = created
		if len(path) == 1 {
			roots = append(roots, created)
		} else {
			parent := find(path[:len(path)-1])
			parent.children = append(parent.children, created)
		}
		return created
	}
	for _, r := range resources {
		target := find(r.Path)
		for _, m := range r.Methods {
			method, err := g.method(r.Path, m)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", method.Name, err)
			}
			target.methods = append(target.methods, method)
		}
	}
	for _, grp := range byPath {
		for _, m := range grp.methods {
			if m.PreviewMethod != "" && !slices.ContainsFunc(grp.methods, func(sibling resource.Method) bool {
				return sibling.Name == m.PreviewMethod
			}) {
				return nil, fmt.Errorf("%s: --preview stands for %s, which is not a sibling method", m.Name, m.PreviewMethod)
			}
		}
	}
	rootNames := slices.Clone(rootCommands)
	for _, root := range roots {
		name := kebab(root.path[0])
		if slices.Contains(rootNames, name) {
			return nil, fmt.Errorf("resource %s collides with another tmh %s", root.path[0], name)
		}
		rootNames = append(rootNames, name)
	}
	for key, grp := range byPath {
		names := []string{}
		for _, m := range grp.methods {
			names = append(names, strings.Fields(m.Use)[0])
		}
		for _, child := range grp.children {
			names = append(names, kebab(child.path[len(child.path)-1]))
		}
		for i, name := range names {
			if slices.Contains(names[:i], name) {
				return nil, fmt.Errorf("two commands named %s under %s", name, key)
			}
		}
	}
	return roots, nil
}

func (g *generator) method(resourcePath []string, m irMethod) (resource.Method, error) {
	method := resource.Method{
		Name:                   strings.Join(append(slices.Clone(resourcePath), m.Name), "."),
		Short:                  firstNonEmpty(m.Summary, humanize(m.Name)),
		HTTPMethod:             m.HTTPMethod,
		OrganizationScoped:     m.OrgScoped,
		RetrySafe:              m.RetrySafe,
		IdempotencyKeyRequired: m.IdempotencyKey == "required",
	}
	if !slices.Contains([]string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}, m.HTTPMethod) {
		return method, fmt.Errorf("unknown HTTP method %q", m.HTTPMethod)
	}
	if len(m.HeaderParams) > 0 {
		return method, fmt.Errorf("header parameters are not supported")
	}
	if !slices.Contains([]string{"required", "optional", "none"}, m.IdempotencyKey) {
		return method, fmt.Errorf("unknown idempotencyKey %q", m.IdempotencyKey)
	}
	// If-None-Match is an optional cache validator on reads; a command has no
	// cache to revalidate, so there is nothing to express.
	if !slices.Contains([]string{"required", "optional", "none"}, m.IfNoneMatch) {
		return method, fmt.Errorf("unknown ifNoneMatch %q", m.IfNoneMatch)
	}
	ifMatch, ok := map[string]resource.Requirement{
		"none": resource.None, "optional": resource.Optional, "required": resource.Required,
	}[m.IfMatch]
	if !ok {
		return method, fmt.Errorf("unknown ifMatch %q", m.IfMatch)
	}
	method.IfMatch = ifMatch

	path, err := g.path(m)
	if err != nil {
		return method, err
	}
	method.Path = path
	use := []string{kebab(m.Name)}
	for _, p := range m.PathParams {
		if p.Type.Kind != "primitive" || (p.Type.Type != "string" && p.Type.Type != "integer") {
			return method, fmt.Errorf("path parameter %s is not a string or integer", p.Name)
		}
		method.PathParams = append(method.PathParams, p.Name)
		use = append(use, "<"+p.Name+">")
	}
	method.Use = strings.Join(use, " ")

	switch m.Security.Kind {
	case "none":
		method.Public = true
	case "bearer":
	default:
		return method, fmt.Errorf("unknown security %q", m.Security.Kind)
	}

	switch m.Response.Kind {
	case "model", "writeResult", "empty":
	case "page":
		pagination, err := paginationOf(m.Pagination)
		if err != nil {
			return method, err
		}
		method.Pagination = pagination
	case "operation":
		method.ReturnsOperation = true
		// An export's Operation carries its file inline (research §8).
		method.Export = m.Name == "export"
	case "binary":
		if m.Response.ContentType == "" {
			return method, fmt.Errorf("binary response without a content type")
		}
		method.BinaryResponse = true
		method.Accept = m.Response.ContentType + binaryAcceptSuffix
	default:
		return method, fmt.Errorf("unknown response kind %q", m.Response.Kind)
	}
	if m.Pagination != nil && m.Response.Kind != "page" {
		return method, fmt.Errorf("pagination on a %s response", m.Response.Kind)
	}

	for _, p := range m.QueryParams {
		flag, err := g.queryFlag(p)
		if err != nil {
			return method, err
		}
		method.Query = append(method.Query, flag)
	}
	var body bodyHelp
	if m.RequestBody != nil {
		method.Body, body, err = g.body(*m.RequestBody)
		if err != nil {
			return method, err
		}
	}
	method.PreviewMethod = previews[method.Name]
	if err := checkFlagNames(method); err != nil {
		return method, err
	}
	method.Long = long(m, method, body)
	return method, nil
}

func paginationOf(p *irPagination) (*resource.Pagination, error) {
	if p == nil {
		return nil, fmt.Errorf("a page response without pagination")
	}
	switch p.Style {
	case "cursor":
		return &resource.Pagination{Param: p.CursorParam}, nil
	case "after":
		return &resource.Pagination{Param: p.CursorParam, ItemID: afterItemID}, nil
	}
	return nil, fmt.Errorf("unknown pagination style %q", p.Style)
}

// path strips the Organization prefix the client fills in and checks that
// the template's placeholders are exactly the path parameters, in order.
func (g *generator) path(m irMethod) (string, error) {
	template := m.Path.Template
	if m.OrgScoped {
		trimmed, ok := strings.CutPrefix(template, orgPrefix)
		if !ok {
			return "", fmt.Errorf("path %s is Organization-scoped but does not start with %s", template, orgPrefix)
		}
		template = trimmed
	}
	placeholders := []string{}
	for rest := template; ; {
		_, after, found := strings.Cut(rest, "{")
		if !found {
			break
		}
		name, remaining, closed := strings.Cut(after, "}")
		if !closed {
			return "", fmt.Errorf("unclosed placeholder in %s", m.Path.Template)
		}
		placeholders = append(placeholders, name)
		rest = remaining
	}
	params := []string{}
	for _, p := range m.PathParams {
		params = append(params, p.Name)
	}
	if !slices.Equal(placeholders, params) {
		return "", fmt.Errorf("path %s has placeholders %v but path parameters %v", m.Path.Template, placeholders, params)
	}
	if m.Path.CustomVerb != nil {
		template += ":" + *m.Path.CustomVerb
	}
	return template, nil
}

func (g *generator) queryFlag(p irParam) (resource.Flag, error) {
	flag := resource.Flag{Name: kebab(p.Name), Path: []string{p.Name}, Required: p.Required}
	kind, values, ok := g.scalar(p.Type)
	if !ok {
		return flag, fmt.Errorf("query parameter %s has a type a flag can't hold", p.Name)
	}
	flag.Type, flag.Values = kind, values
	flag.Usage = usage(p.Description, p.Name, values, false)
	return flag, nil
}

// scalar maps a type a single flag can hold: primitives, open enums, and
// arrays of strings or enums.
func (g *generator) scalar(ref irTypeRef) (resource.FlagType, []string, bool) {
	switch ref.Kind {
	case "primitive":
		switch ref.Type {
		case "string", "dateTime":
			return resource.String, nil, true
		case "integer":
			return resource.Int, nil, true
		case "number":
			return resource.Number, nil, true
		case "boolean":
			return resource.Bool, nil, true
		}
	case "ref":
		if t, ok := g.types[ref.Name]; ok && t.Kind == "enum" {
			return resource.String, t.Values, true
		}
	case "array":
		if ref.Items != nil {
			if kind, values, ok := g.scalar(*ref.Items); ok && kind == resource.String {
				return resource.StringList, values, true
			}
		}
	}
	return 0, nil, false
}

// bodyHelp is what the help text says about a body beyond its flags.
type bodyHelp struct {
	mergePatch bool
	// nullable are top-level fields a merge patch can clear.
	nullable []string
	// complex are fields with no flag, set with -d key:=<json>.
	complex []string
}

func (g *generator) body(b irBody) (*resource.Body, bodyHelp, error) {
	help := bodyHelp{}
	switch b.ContentType {
	case "application/json", "application/merge-patch+json", "multipart/form-data":
	default:
		return nil, help, fmt.Errorf("unsupported body content type %s", b.ContentType)
	}
	t, ok := g.objectType(b.Type)
	if !ok {
		return nil, help, fmt.Errorf("the request body is not an object")
	}
	multipart := b.ContentType == "multipart/form-data"
	if !multipart && len(b.BinaryParts) > 0 {
		return nil, help, fmt.Errorf("binary parts in a %s body", b.ContentType)
	}
	body := &resource.Body{ContentType: b.ContentType}
	fileParts := map[string]bool{}
	for _, part := range b.BinaryParts {
		body.FileParts = append(body.FileParts, resource.FilePart{Name: part.Name, ContentType: part.ContentType})
		fileParts[part.Name] = true
	}
	variants := map[string]string{}
	fixed := map[string]bool{}
	for _, c := range b.Constraints {
		key := strings.Join(c.Path, ".")
		switch c.Kind {
		case "const":
			body.Fixed = append(body.Fixed, resource.Fixed{Path: c.Path, JSON: string(c.Value)})
			fixed[key] = true
		case "variant":
			variants[key] = c.RequiredProperty
		default:
			return nil, help, fmt.Errorf("unknown constraint kind %q", c.Kind)
		}
	}
	w := &fieldWalker{g: g, variants: variants, fixed: fixed, used: map[string]bool{}, flags: map[string]int{}}
	fields := []irField{}
	for _, field := range t.Fields {
		if fileParts[field.Name] {
			continue
		}
		if field.Type.Kind == "primitive" && field.Type.Type == "binary" {
			return nil, help, fmt.Errorf("binary field %s is not a declared binary part", field.Name)
		}
		fields = append(fields, field)
		if field.Nullable {
			help.nullable = append(help.nullable, field.Name)
		}
	}
	if err := w.walk(nil, fields, true, map[string]bool{t.Name: true}); err != nil {
		return nil, help, err
	}
	for key := range variants {
		if !w.used["variant "+key] {
			return nil, help, fmt.Errorf("variant constraint on %s matches no union", key)
		}
	}
	for key := range fixed {
		if !w.used["fixed "+key] {
			return nil, help, fmt.Errorf("const constraint on %s matches no field", key)
		}
	}
	body.Fields = w.out
	help.mergePatch = t.MergePatch
	help.complex = w.complex
	return body, help, nil
}

func (g *generator) objectType(ref irTypeRef) (irType, bool) {
	if ref.Kind != "ref" {
		return irType{}, false
	}
	t, ok := g.types[ref.Name]
	return t, ok && t.Kind == "object"
}

// fieldWalker flattens a body into flags: nested objects and union members
// become dotted flags, and what no flag can hold is listed as complex.
type fieldWalker struct {
	g        *generator
	variants map[string]string
	fixed    map[string]bool
	used     map[string]bool
	out      []resource.Flag
	// flags indexes out by flag name, so union members share one flag.
	flags   map[string]int
	complex []string
}

func (w *fieldWalker) walk(prefix []string, fields []irField, required bool, seen map[string]bool) error {
	for _, field := range fields {
		path := append(slices.Clone(prefix), field.Name)
		key := strings.Join(path, ".")
		if w.fixed[key] {
			w.used["fixed "+key] = true
			continue
		}
		if err := w.field(path, field, required && field.Required, seen); err != nil {
			return err
		}
	}
	return nil
}

func (w *fieldWalker) field(path []string, field irField, required bool, seen map[string]bool) error {
	key := strings.Join(path, ".")
	if kind, values, ok := w.g.scalar(field.Type); ok {
		return w.add(resource.Flag{
			Name:   kebab(key),
			Path:   path,
			Type:   kind,
			Values: values,
			Usage:  usage(field.Description, field.Name, values, required),
		})
	}
	if field.Type.Kind != "ref" || seen[field.Type.Name] {
		w.complex = append(w.complex, key)
		return nil
	}
	t, ok := w.g.types[field.Type.Name]
	if !ok {
		return fmt.Errorf("field %s refers to unknown type %s", key, field.Type.Name)
	}
	nested := maps.Clone(seen)
	nested[t.Name] = true
	switch t.Kind {
	case "object":
		return w.walk(path, t.Fields, required, nested)
	case "union":
		return w.union(path, t, required, nested)
	}
	w.complex = append(w.complex, key)
	return nil
}

// union gives every member's fields a flag; a variant constraint narrows it
// to the one member that has the required property (grilling Q25).
func (w *fieldWalker) union(path []string, t irType, required bool, seen map[string]bool) error {
	key := strings.Join(path, ".")
	d := t.Discriminator
	if d == nil || (d.Kind != "property" && d.Kind != "presence") {
		return fmt.Errorf("union %s has no discriminator this generator knows", t.Name)
	}
	members := []irType{}
	for _, name := range slices.Sorted(maps.Keys(d.Mapping)) {
		member, ok := w.g.types[d.Mapping[name]]
		if !ok || member.Kind != "object" {
			return fmt.Errorf("union %s member %s is not an object", t.Name, d.Mapping[name])
		}
		members = append(members, member)
	}
	if property, ok := w.variants[key]; ok {
		w.used["variant "+key] = true
		members = slices.DeleteFunc(members, func(member irType) bool {
			return !slices.ContainsFunc(member.Fields, func(f irField) bool { return f.Name == property && f.Required })
		})
		if len(members) != 1 {
			return fmt.Errorf("variant %s of %s matches %d members, want 1", property, key, len(members))
		}
	}
	memberRequired := required && len(members) == 1
	for _, member := range members {
		for _, field := range member.Fields {
			if d.Kind == "property" && field.Name == d.Property {
				// The members type the discriminator as a plain string; its
				// values are the union's mapping keys.
				fieldPath := append(slices.Clone(path), field.Name)
				values := slices.Sorted(maps.Keys(d.Mapping))
				if err := w.add(resource.Flag{
					Name:   kebab(strings.Join(fieldPath, ".")),
					Path:   fieldPath,
					Type:   resource.String,
					Values: values,
					Usage:  usage(field.Description, field.Name, values, memberRequired && field.Required),
				}); err != nil {
					return err
				}
				continue
			}
			if err := w.walk(path, []irField{field}, memberRequired, seen); err != nil {
				return err
			}
		}
	}
	return nil
}

// add keeps one flag per name: union members that share a field share its
// flag, which needs the same type in each.
func (w *fieldWalker) add(flag resource.Flag) error {
	if i, ok := w.flags[flag.Name]; ok {
		if w.out[i].Type != flag.Type || !slices.Equal(w.out[i].Path, flag.Path) {
			return fmt.Errorf("--%s has different types in different union members", flag.Name)
		}
		return nil
	}
	w.flags[flag.Name] = len(w.out)
	w.out = append(w.out, flag)
	return nil
}

func checkFlagNames(method resource.Method) error {
	names := slices.Clone(globalFlags)
	add := func(name string) error {
		if slices.Contains(names, name) {
			return fmt.Errorf("flag --%s is defined twice", name)
		}
		names = append(names, name)
		return nil
	}
	flags := slices.Clone(method.Query)
	extra := []string{}
	if method.Body != nil {
		flags = append(flags, method.Body.Fields...)
		for _, part := range method.Body.FileParts {
			extra = append(extra, part.Name)
		}
		if len(method.Body.FileParts) == 0 {
			extra = append(extra, "data")
		}
	}
	if method.IfMatch != resource.None {
		extra = append(extra, "if-match")
	}
	if method.BinaryResponse || method.Export {
		extra = append(extra, "output")
	}
	if method.Pagination != nil {
		extra = append(extra, "all", "ndjson")
	}
	if method.ReturnsOperation {
		extra = append(extra, "wait", "wait-timeout")
	}
	if method.PreviewMethod != "" {
		extra = append(extra, "preview")
	}
	for _, flag := range flags {
		extra = append(extra, flag.Name)
	}
	for _, name := range extra {
		if err := add(name); err != nil {
			return err
		}
	}
	return nil
}

// long is the command's help: the IR's description, then what the finished
// method's flags do.
func long(m irMethod, method resource.Method, body bodyHelp) string {
	paragraphs := []string{firstNonEmpty(m.Description, firstNonEmpty(m.Summary, humanize(m.Name)))}
	if body.mergePatch {
		text := "Fields you leave out keep their current values."
		for _, name := range body.nullable {
			text += fmt.Sprintf(" Clear %s with -d %s:=null.", name, name)
		}
		paragraphs = append(paragraphs, text)
	}
	if len(body.complex) > 0 {
		examples := []string{}
		for _, key := range body.complex {
			examples = append(examples, "-d "+key+":=<json>")
		}
		paragraphs = append(paragraphs, fmt.Sprintf("Set %s, or send the whole body with --data @file.json.",
			strings.Join(examples, ", ")))
	}
	switch {
	case method.Pagination != nil:
		paragraphs = append(paragraphs, "--all follows every page, keeping the other flags, and prints all the items "+
			"as one JSON array, so `| jq` works; add --ndjson to print one compact item per line as each page arrives.")
	case method.ReturnsOperation:
		text := "--wait polls the Operation until it is done (up to --wait-timeout) and prints it; " +
			"if it failed, tmh prints its problem and exits as its HTTP status would, or 7 without one."
		if method.Export {
			text += " -o <file> saves the finished export's CSV instead."
		}
		paragraphs = append(paragraphs, text)
	}
	paragraphs = append(paragraphs, "Scopes: "+scopes(m.Security))
	return strings.Join(paragraphs, "\n\n")
}

func scopes(security irSecurity) string {
	if security.Kind == "none" {
		return "none (public, no login needed)"
	}
	alternatives := []string{}
	for _, all := range security.AnyOf {
		alternatives = append(alternatives, strings.Join(all, " and "))
	}
	return strings.Join(alternatives, " or ")
}

func usage(description *string, name string, values []string, required bool) string {
	text := firstNonEmpty(description, humanize(name))
	text = strings.Join(strings.Fields(text), " ")
	if len(values) > 0 {
		text = strings.TrimSuffix(text, ".") + "; one of: " + strings.Join(values, ", ")
	}
	if required {
		text += " (required)"
	}
	return text
}

func firstNonEmpty(value *string, fallback string) string {
	if value != nil && strings.TrimSpace(*value) != "" {
		return strings.TrimSpace(*value)
	}
	return fallback
}

// humanize spells an IR name as words: "webhookSubscriptions" is "Webhook subscriptions".
func humanize(name string) string {
	words := strings.ReplaceAll(kebab(name), "-", " ")
	runes := []rune(words)
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return string(runes)
}

// kebab is tmh's spelling of an IR name: "upsertByExternalId" is
// "upsert-by-external-id", and dotted names stay dotted.
func kebab(name string) string {
	var out strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 && runes[i-1] != '.' {
			previousUpper := unicode.IsUpper(runes[i-1])
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if !previousUpper || nextLower {
				out.WriteByte('-')
			}
		}
		out.WriteRune(unicode.ToLower(r))
	}
	return out.String()
}
