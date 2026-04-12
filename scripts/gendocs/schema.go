package gendocs

// Schema kind constants.
const (
	SchemaKindUnion    = "union"
	SchemaKindFreeform = "freeform"
	SchemaKindPage     = "page"
)

// DocSchema is the v2 JSON schema emitted for each target.
type DocSchema struct {
	SchemaVersion int             `json:"schemaVersion"`
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	BlockHeader   string          `json:"blockHeader,omitempty"`
	Description   string          `json:"description,omitempty"`
	Kind          string          `json:"kind,omitempty"`
	LabelName     string          `json:"labelName,omitempty"`
	Attributes    []DocAttribute  `json:"attributes,omitempty"`
	Blocks        []DocBlockRef   `json:"blocks,omitempty"`
	Variants      []DocVariant    `json:"variants,omitempty"`
	Remain        *DocRemain      `json:"remain,omitempty"` // non-nil when the struct has hcl:",remain"
}

// DocAttribute describes a single HCL attribute (non-block field).
type DocAttribute struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Description string   `json:"description,omitempty"`
	Default     string   `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// DocBlockRef references a nested block within a schema.
type DocBlockRef struct {
	Name        string `json:"name"`
	Ref         string `json:"ref,omitempty"` // target id of the nested block's schema
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

// DocVariant is one arm of a labeled union.
type DocVariant struct {
	Label string `json:"label"`
	Ref   string `json:"ref,omitempty"` // target id, empty for label-only variants
}

// DocRemain documents a ",remain" body on the struct.
type DocRemain struct {
	Description string `json:"description,omitempty"`
}
