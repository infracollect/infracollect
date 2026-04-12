package gendocs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractSchema_Fixtures(t *testing.T) {
	reg := &Registry{
		Targets: []Target{
			{
				ID:          "simple",
				Package:     "github.com/infracollect/infracollect/scripts/gendocs/testdata",
				Type:        "SimpleConfig",
				Kind:        KindRootBlock,
				BlockHeader: `resource "test" "<id>"`,
			},
			{
				ID:          "block-config",
				Package:     "github.com/infracollect/infracollect/scripts/gendocs/testdata",
				Type:        "BlockConfig",
				Kind:        KindRootBlock,
				BlockHeader: "block",
			},
			{
				ID:      "auth-fixture",
				Package: "github.com/infracollect/infracollect/scripts/gendocs/testdata",
				Type:    "AuthBlockFixture",
				Kind:    KindUnion,
				BlockHeader: `auth "<kind>"`,
				Variants: map[string]*string{
					"basic": strPtr("auth-basic"),
				},
			},
			{
				ID:      "untagged",
				Package: "github.com/infracollect/infracollect/scripts/gendocs/testdata",
				Type:    "UntaggedFields",
				Kind:    KindRootBlock,
			},
			{
				ID:      "format",
				Package: "github.com/infracollect/infracollect/scripts/gendocs/testdata",
				Type:    "FormatConfig",
				Kind:    KindRootBlock,
			},
		},
	}

	loaded, err := LoadPackages(".", reg)
	require.NoError(t, err)

	idx := BuildBlockHeaderIndex(reg)

	t.Run("simple config attributes", func(t *testing.T) {
		schema, err := ExtractSchema(loaded, reg, idx, reg.Targets[0])
		require.NoError(t, err)

		assert.Equal(t, 2, schema.SchemaVersion)
		assert.Equal(t, "simple", schema.ID)
		assert.Equal(t, "SimpleConfig", schema.Name)

		require.Len(t, schema.Attributes, 5)

		name := schema.Attributes[0]
		assert.Equal(t, "name", name.Name)
		assert.Equal(t, "string", name.Type)
		assert.True(t, name.Required)

		count := schema.Attributes[1]
		assert.Equal(t, "count", count.Name)
		assert.Equal(t, "number", count.Type)
		assert.False(t, count.Required)
		assert.Equal(t, "10", count.Default)

		tags := schema.Attributes[2]
		assert.Equal(t, "tags", tags.Name)
		assert.Equal(t, "map(string)", tags.Type)
		assert.False(t, tags.Required)

		items := schema.Attributes[3]
		assert.Equal(t, "items", items.Name)
		assert.Equal(t, "list(string)", items.Type)

		enable := schema.Attributes[4]
		assert.Equal(t, "enable", enable.Name)
		assert.Equal(t, "bool", enable.Type)
	})

	t.Run("block config with nested block and remain", func(t *testing.T) {
		schema, err := ExtractSchema(loaded, reg, idx, reg.Targets[1])
		require.NoError(t, err)

		require.Len(t, schema.Blocks, 1)
		assert.Equal(t, "auth", schema.Blocks[0].Name)
		assert.False(t, schema.Blocks[0].Required, "pointer block should be optional")

		require.NotNil(t, schema.Remain)
	})

	t.Run("union schema", func(t *testing.T) {
		schema, err := ExtractSchema(loaded, reg, idx, reg.Targets[2])
		require.NoError(t, err)

		assert.Equal(t, SchemaKindUnion, schema.Kind)
		assert.Equal(t, "kind", schema.LabelName)
		require.Len(t, schema.Variants, 1)
		assert.Equal(t, "basic", schema.Variants[0].Label)
		assert.Equal(t, "auth-basic", schema.Variants[0].Ref)
	})

	t.Run("untagged fields are skipped", func(t *testing.T) {
		schema, err := ExtractSchema(loaded, reg, idx, reg.Targets[3])
		require.NoError(t, err)

		require.Len(t, schema.Attributes, 1)
		assert.Equal(t, "name", schema.Attributes[0].Name)
	})

	t.Run("enum extraction from doc", func(t *testing.T) {
		schema, err := ExtractSchema(loaded, reg, idx, reg.Targets[4])
		require.NoError(t, err)

		require.Len(t, schema.Attributes, 1)
		assert.Equal(t, []string{"json", "yaml", "toml"}, schema.Attributes[0].Enum)
	})
}

func strPtr(s string) *string { return &s }
