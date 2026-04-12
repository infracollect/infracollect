package testdata

import "github.com/hashicorp/hcl/v2"

// SimpleConfig is a basic HCL config with various field types.
type SimpleConfig struct {
	// Name is the resource name.
	Name string `hcl:"name"`
	// Count of items. Default: "10".
	Count *int `hcl:"count,optional"`
	// Tags applied to the resource.
	Tags map[string]string `hcl:"tags,optional"`
	// Items is a list of item names.
	Items []string `hcl:"items,optional"`
	// Enable toggles the feature.
	Enable bool `hcl:"enable,optional"`
}

// BlockConfig has a nested block and a remain body.
type BlockConfig struct {
	// Auth configures authentication.
	Auth *AuthBlockFixture `hcl:"auth,block"`
	Body hcl.Body          `hcl:",remain"`
}

// AuthBlockFixture is a labeled block.
type AuthBlockFixture struct {
	Kind     string `hcl:"kind,label"`
	Username string `hcl:"username,optional"`
	Password string `hcl:"password,optional"`
}

// UntaggedFields has some fields without hcl tags.
type UntaggedFields struct {
	Name     string `hcl:"name"`
	Internal string // no tag — should be skipped
}

// FormatConfig has a field with enum docs.
// One of: json, yaml, toml.
type FormatConfig struct {
	// Format for the output. One of: json, yaml, toml.
	Format string `hcl:"format"`
}
