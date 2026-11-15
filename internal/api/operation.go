package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Operation is a long-running operation (AIP-151).
type Operation struct {
	// Name is `organizations/{orgId}/operations/{operationId}`.
	Name     string          `json:"name"`
	Done     bool            `json:"done"`
	Response json.RawMessage `json:"response"`
	Error    *struct {
		Code   string       `json:"code"`
		Title  string       `json:"title"`
		Status int          `json:"status"`
		Detail string       `json:"detail"`
		Errors []FieldError `json:"errors"`
	} `json:"error"`
	// Body is the Operation as the API sent it.
	Body []byte `json:"-"`
}

// ParseOperation reads an Operation from a response body.
func ParseOperation(body []byte) (*Operation, error) {
	var operation Operation
	if err := json.Unmarshal(body, &operation); err != nil || operation.Name == "" {
		return nil, errors.New("the response is not an Operation")
	}
	operation.Body = body
	return &operation, nil
}

// ID is the last segment of the Operation's name.
func (o *Operation) ID() string {
	return o.Name[strings.LastIndex(o.Name, "/")+1:]
}

// OperationFailedError means the Operation finished with a problem.
type OperationFailedError struct {
	Operation *Operation
	// Problem is the Operation's error, rendered like an API error.
	Problem *Error
}

// Error names the Operation and its problem.
func (e *OperationFailedError) Error() string {
	return fmt.Sprintf("operation %s failed: %s", e.Operation.Name, e.Problem.Error())
}

// Unwrap returns the problem, so it is handled like an API error.
func (e *OperationFailedError) Unwrap() error { return e.Problem }

// OperationTimeoutError means the Operation was still running when the wait
// ran out; Operation is the last state seen.
type OperationTimeoutError struct {
	Operation *Operation
	Timeout   time.Duration
}

// Error names the Operation and how long tmh waited.
func (e *OperationTimeoutError) Error() string {
	return fmt.Sprintf("operation %s is still running after %s", e.Operation.Name, e.Timeout)
}

const (
	operationPollInterval = time.Second
	maxOperationPoll      = 10 * time.Second
)

// WaitOperation polls the Operation until it is done, as the SDKs' wait()
// does: no request when it is already done, a failed Operation is an
// *OperationFailedError, and running past timeout an *OperationTimeoutError.
// It returns the last Operation seen with either error.
func (c *Client) WaitOperation(ctx context.Context, operation *Operation, timeout time.Duration) (*Operation, error) {
	path, err := operationPath(operation.Name)
	if err != nil {
		return operation, err
	}
	deadline := c.config.Clock.Now().Add(timeout)
	for poll := 1; !operation.Done; poll++ {
		remaining := deadline.Sub(c.config.Clock.Now())
		if remaining <= 0 {
			return operation, &OperationTimeoutError{Operation: operation, Timeout: timeout}
		}
		if err := c.config.Clock.Sleep(ctx, min(operationPollDelay(poll), remaining)); err != nil {
			return operation, err
		}
		response, err := c.Do(ctx, Request{Method: http.MethodGet, Path: path, RetrySafe: true})
		if err != nil {
			return operation, err
		}
		if operation, err = ParseOperation(response.Body); err != nil {
			return nil, err
		}
	}
	if p := operation.Error; p != nil {
		problem := &Error{Status: p.Status, Code: p.Code, Title: p.Title, Detail: p.Detail, Errors: p.Errors,
			message: joinNonEmpty(p.Title, p.Detail)}
		if problem.message == "" {
			problem.message = p.Code
		}
		return operation, &OperationFailedError{Operation: operation, Problem: problem}
	}
	return operation, nil
}

// operationPollDelay is the SDKs' wait() schedule: the ceiling doubles from
// the 1 s interval to 10 s, jitter picks 50–100 % of it, and nothing waits
// under the interval.
func operationPollDelay(poll int) time.Duration {
	ceiling := min(operationPollInterval<<min(poll-1, 10), maxOperationPoll)
	jittered := time.Duration(float64(ceiling) * (0.5 + rand.Float64()*0.5))
	return max(operationPollInterval, jittered)
}

// operationPath escapes each segment of a name the API sent, so it can only
// ever address an Operation.
func operationPath(name string) (string, error) {
	segments := strings.Split(name, "/")
	if len(segments) != 4 || segments[0] != "organizations" || segments[2] != "operations" ||
		segments[1] == "" || segments[3] == "" {
		return "", fmt.Errorf("%q is not an Operation name", name)
	}
	return "/organizations/" + url.PathEscape(segments[1]) + "/operations/" + url.PathEscape(segments[3]), nil
}

// Export is the file an export Operation's response holds inline
// (`ExportArtifact`).
type Export struct {
	Filename string
	RowCount int
	Data     []byte
}

// ExportOf decodes a finished export Operation's file.
func ExportOf(operation *Operation) (*Export, error) {
	var artifact struct {
		Filename   string  `json:"filename"`
		RowCount   int     `json:"rowCount"`
		DataBase64 *string `json:"dataBase64"`
	}
	if !operation.Done || json.Unmarshal(operation.Response, &artifact) != nil || artifact.DataBase64 == nil {
		return nil, fmt.Errorf("operation %s holds no export file", operation.Name)
	}
	data, err := base64.StdEncoding.DecodeString(*artifact.DataBase64)
	if err != nil {
		return nil, fmt.Errorf("operation %s: the export file is not valid base64", operation.Name)
	}
	return &Export{Filename: artifact.Filename, RowCount: artifact.RowCount, Data: data}, nil
}
