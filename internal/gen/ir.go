package gen

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// The IR as openapi/sdk-ir.schema.json describes it, down to what the
// command tree needs. Decoding rejects unknown fields, so a newer IR shape
// fails here instead of producing commands that silently drop something.

type irDocument struct {
	IRVersion int `json:"irVersion"`
	Source    struct {
		Release string `json:"release"`
		SHA256  string `json:"sha256"`
	} `json:"source"`
	Resources []irResource    `json:"resources"`
	Types     []irType        `json:"types"`
	Servers   json.RawMessage `json:"servers"`
	Auth      json.RawMessage `json:"auth"`
	Events    json.RawMessage `json:"events"`
	Errors    json.RawMessage `json:"errors"`
}

type irResource struct {
	Path    []string   `json:"path"`
	Methods []irMethod `json:"methods"`
}

type irMethod struct {
	Name           string        `json:"name"`
	OperationID    string        `json:"operationId"`
	Summary        *string       `json:"summary"`
	Description    *string       `json:"description"`
	HTTPMethod     string        `json:"httpMethod"`
	Path           irPath        `json:"path"`
	OrgScoped      bool          `json:"orgScoped"`
	PathParams     []irParam     `json:"pathParams"`
	QueryParams    []irParam     `json:"queryParams"`
	HeaderParams   []irParam     `json:"headerParams"`
	IdempotencyKey string        `json:"idempotencyKey"`
	IfMatch        string        `json:"ifMatch"`
	IfNoneMatch    string        `json:"ifNoneMatch"`
	RetrySafe      bool          `json:"retrySafe"`
	Security       irSecurity    `json:"security"`
	RequestBody    *irBody       `json:"requestBody"`
	Response       irResponse    `json:"response"`
	Pagination     *irPagination `json:"pagination"`
}

type irPath struct {
	Template   string  `json:"template"`
	CustomVerb *string `json:"customVerb"`
}

type irParam struct {
	Name        string    `json:"name"`
	Required    bool      `json:"required"`
	Description *string   `json:"description"`
	Type        irTypeRef `json:"type"`
}

type irSecurity struct {
	Kind  string     `json:"kind"`
	AnyOf [][]string `json:"anyOf"`
}

type irBody struct {
	ContentType string         `json:"contentType"`
	Required    bool           `json:"required"`
	Type        irTypeRef      `json:"type"`
	BinaryParts []irBinaryPart `json:"binaryParts"`
	Constraints []irConstraint `json:"constraints"`
}

type irBinaryPart struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
}

type irConstraint struct {
	Kind             string          `json:"kind"`
	Path             []string        `json:"path"`
	Value            json.RawMessage `json:"value"`
	RequiredProperty string          `json:"requiredProperty"`
}

type irResponse struct {
	Kind        string     `json:"kind"`
	Type        *irTypeRef `json:"type"`
	ContentType string     `json:"contentType"`
	Existing    *irTypeRef `json:"existing"`
	Created     *irTypeRef `json:"created"`
}

type irPagination struct {
	Style       string    `json:"style"`
	CursorParam string    `json:"cursorParam"`
	LimitParam  string    `json:"limitParam"`
	MaxLimit    int       `json:"maxLimit"`
	ItemType    irTypeRef `json:"itemType"`
}

type irTypeRef struct {
	Kind   string     `json:"kind"`
	Type   string     `json:"type"`
	Name   string     `json:"name"`
	Items  *irTypeRef `json:"items"`
	Values *irTypeRef `json:"values"`
}

type irType struct {
	Kind             string           `json:"kind"`
	Name             string           `json:"name"`
	Description      *string          `json:"description"`
	Fields           []irField        `json:"fields"`
	AdditionalFields bool             `json:"additionalFields"`
	MergePatch       bool             `json:"mergePatch"`
	Values           []string         `json:"values"`
	Open             bool             `json:"open"`
	Discriminator    *irDiscriminator `json:"discriminator"`
}

type irField struct {
	Name        string    `json:"name"`
	Type        irTypeRef `json:"type"`
	Required    bool      `json:"required"`
	Nullable    bool      `json:"nullable"`
	Description *string   `json:"description"`
}

type irDiscriminator struct {
	Kind     string            `json:"kind"`
	Property string            `json:"property"`
	Mapping  map[string]string `json:"mapping"`
	Fallback *string           `json:"fallback"`
}

const supportedIRVersion = 1

func decodeIR(raw []byte) (*irDocument, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var document irDocument
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("openapi/sdk-ir.json does not match the IR this generator knows: %w", err)
	}
	if document.IRVersion != supportedIRVersion {
		return nil, fmt.Errorf("irVersion %d: this generator reads version %d", document.IRVersion, supportedIRVersion)
	}
	return &document, nil
}
