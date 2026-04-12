package gendocs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHCLTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want *HCLTag
	}{
		{
			name: "simple required attribute",
			tag:  "name",
			want: &HCLTag{Name: "name"},
		},
		{
			name: "optional attribute",
			tag:  "count,optional",
			want: &HCLTag{Name: "count", Optional: true},
		},
		{
			name: "label field",
			tag:  "kind,label",
			want: &HCLTag{Name: "kind", Label: true},
		},
		{
			name: "block field",
			tag:  "auth,block",
			want: &HCLTag{Name: "auth", Block: true},
		},
		{
			name: "remain body",
			tag:  ",remain",
			want: &HCLTag{Remain: true},
		},
		{
			name: "empty tag",
			tag:  "",
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseHCLTag(tc.tag)
			if tc.want == nil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tc.want.Name, got.Name)
			assert.Equal(t, tc.want.Optional, got.Optional)
			assert.Equal(t, tc.want.Label, got.Label)
			assert.Equal(t, tc.want.Block, got.Block)
			assert.Equal(t, tc.want.Remain, got.Remain)
		})
	}
}

func TestCleanDoc(t *testing.T) {
	assert.Equal(t, "Hello world.", CleanDoc("  Hello world.  "))
	assert.Equal(t, "Line one.\nLine two.", CleanDoc("  Line one.\n  Line two.  "))
}

func TestExtractDefault(t *testing.T) {
	assert.Equal(t, "10", ExtractDefault(`Count of items. Default: "10".`))
	assert.Equal(t, "gzip", ExtractDefault(`Compression. Defaults to "gzip".`))
	assert.Equal(t, "", ExtractDefault("No default here."))
}

func TestExtractEnum(t *testing.T) {
	assert.Equal(t, []string{"json", "yaml", "toml"}, ExtractEnum("One of: json, yaml, toml."))
	assert.Nil(t, ExtractEnum("No enum here."))
}
