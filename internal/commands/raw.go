package commands

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/api"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
)

// orgIDToken is how the API reference writes Organization-scoped paths, so a
// path copied from it works as-is.
const orgIDToken = "{orgId}"

type rawOptions struct {
	query       []string
	data        []string
	contentType string
	ifMatch     string
}

func newRawCommands(f *cmdutil.Factory) []*cobra.Command {
	return []*cobra.Command{
		newRawCommand(f, http.MethodGet),
		newRawCommand(f, http.MethodPost),
		newRawCommand(f, http.MethodPatch),
		newRawCommand(f, http.MethodPut),
		newRawCommand(f, http.MethodDelete),
	}
}

// defaultContentType follows the API: PATCH bodies are JSON merge patches.
func defaultContentType(method string) string {
	if method == http.MethodPatch {
		return "application/merge-patch+json"
	}
	return "application/json"
}

var rawRetries = map[string]string{
	http.MethodGet:    "Transient failures are retried.",
	http.MethodPost:   "Each run sends a generated Idempotency-Key, so transient failures are retried safely.",
	http.MethodPut:    "Transient failures are retried only when you set --idempotency-key.",
	http.MethodPatch:  "Transient failures are never retried, since the change may already have been applied.",
	http.MethodDelete: "Transient failures are never retried, since the delete may already have happened.",
}

var rawExamples = map[string]string{
	http.MethodGet: "  tmh get /organizations/{orgId}/learners --query limit=5",
	http.MethodPost: "  tmh post /organizations/{orgId}/learners -d email=ana@example.com -d name=\"Ana Example\"\n" +
		"  tmh post /organizations/{orgId}/webhook-subscriptions --data @subscription.json",
	http.MethodPatch:  "  tmh patch /organizations/{orgId}/learners/sub_123 -d externalId:=null --if-match '\"v7\"'",
	http.MethodPut:    "  tmh put /organizations/{orgId}/learners/by-external-id/hr-123 -d email=ana@example.com --idempotency-key sync-hr-123",
	http.MethodDelete: "  tmh delete /organizations/{orgId}/webhook-subscriptions/whs_123",
}

func newRawCommand(f *cmdutil.Factory, method string) *cobra.Command {
	options := &rawOptions{}
	command := &cobra.Command{
		Use:   strings.ToLower(method) + " <path>",
		Short: fmt.Sprintf("Send a %s request to any API path", method),
		Long: fmt.Sprintf("Sends a %s request to a path relative to the API base URL, which already ends in /v1, "+
			"with the same authentication and error handling as the resource commands. %s "+
			"A literal %s in the path is replaced by the Organization your credentials are bound to.",
			method, rawRetries[method], orgIDToken),
		Example: rawExamples[method],
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRaw(cmd, f, method, args[0], options)
		},
	}
	flags := command.Flags()
	flags.StringArrayVar(&options.query, "query", nil, "query parameter as key=value; repeatable")
	if method == http.MethodGet {
		return command
	}
	flags.StringVar(&options.ifMatch, "if-match", "", "send If-Match with this ETag, for a conditional write")
	if method == http.MethodDelete {
		return command
	}
	flags.StringArrayVarP(&options.data, "data", "d", nil,
		"body field as key=value (dotted keys nest, key[]=v appends, key:=<json> sends JSON), "+
			"or the whole body as @file, @- for stdin, or a JSON document; repeatable")
	flags.StringVar(&options.contentType, "content-type", defaultContentType(method), "Content-Type of the body")
	return command
}

func runRaw(cmd *cobra.Command, f *cmdutil.Factory, method, rawPath string, options *rawOptions) error {
	path, query, err := splitRawPath(rawPath)
	if err != nil {
		return err
	}
	for _, pair := range options.query {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return &cmdutil.UsageError{Message: fmt.Sprintf("--query %q: use key=value", pair)}
		}
		query.Add(key, value)
	}
	body, err := f.Body(options.data, options.contentType)
	if err != nil {
		return err
	}
	client, _, err := f.APIClient()
	if err != nil {
		return err
	}
	if strings.Contains(path, orgIDToken) {
		organizationID, err := client.OrganizationID(cmd.Context())
		if err != nil {
			return err
		}
		path = strings.ReplaceAll(path, orgIDToken, url.PathEscape(organizationID))
	}
	response, err := client.Do(cmd.Context(), api.Request{
		Method: method,
		Path:   path,
		Query:  query,
		// Only POST gets a generated key: the API dedupes POSTs by key, and a
		// raw PUT is replayed only when the caller vouches with their own.
		IdempotencyKey: f.IdempotencyKey(method == http.MethodPost),
		Body:           body,
		ContentType:    options.contentType,
		IfMatch:        options.ifMatch,
	})
	if err != nil {
		return err
	}
	return f.PrintJSON(response.Body)
}

// splitRawPath separates a query string written into the path, so it merges
// with --query instead of producing a second "?".
func splitRawPath(rawPath string) (string, url.Values, error) {
	if strings.Contains(rawPath, "://") {
		return "", nil, &cmdutil.UsageError{Message: fmt.Sprintf("%q: give a path relative to the API base URL, not a URL", rawPath)}
	}
	path, rawQuery, _ := strings.Cut(rawPath, "?")
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", nil, &cmdutil.UsageError{Message: fmt.Sprintf("%q: %v", rawPath, err)}
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Paths copied from the reference often keep the /v1 the base URL has.
	if trimmed, ok := strings.CutPrefix(path, "/v1/"); ok {
		path = "/" + trimmed
	}
	return path, query, nil
}
