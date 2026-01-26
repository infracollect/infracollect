package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandTemplates_String(t *testing.T) {
	type S struct {
		Path string `template:""`
	}
	in := S{Path: "${JOB_NAME}/data"}
	err := ExpandTemplates(&in, map[string]string{"JOB_NAME": "my-job"})
	require.NoError(t, err)
	assert.Equal(t, S{Path: "my-job/data"}, in)
}

func TestExpandTemplates_PtrString(t *testing.T) {
	type S struct {
		Path *string `template:""`
	}
	s := "${JOB_NAME}"
	in := S{Path: &s}
	err := ExpandTemplates(&in, map[string]string{"JOB_NAME": "my-job"})
	require.NoError(t, err)
	require.NotNil(t, in.Path)
	assert.Equal(t, "my-job", *in.Path)
}

func TestExpandTemplates_PtrStringNil(t *testing.T) {
	type S struct {
		Path *string `template:""`
	}
	in := S{Path: nil}
	err := ExpandTemplates(&in, map[string]string{})
	require.NoError(t, err)
	assert.Nil(t, in.Path)
}

func TestExpandTemplates_MapStringString(t *testing.T) {
	type S struct {
		Headers map[string]string `template:""`
	}
	in := S{Headers: map[string]string{"X-Job": "${JOB_NAME}"}}
	err := ExpandTemplates(&in, map[string]string{"JOB_NAME": "my-job"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"X-Job": "my-job"}, in.Headers)
}

func TestExpandTemplates_MapStringStringNil(t *testing.T) {
	type S struct {
		Headers map[string]string `template:""`
	}
	in := S{Headers: nil}
	err := ExpandTemplates(&in, map[string]string{})
	require.NoError(t, err)
	assert.Nil(t, in.Headers)
}

func TestExpandTemplates_NestedStruct(t *testing.T) {
	type Inner struct {
		Path string `template:""`
	}
	type Outer struct {
		Inner Inner `template:""`
	}
	in := Outer{Inner: Inner{Path: "${X}"}}
	err := ExpandTemplates(&in, map[string]string{"X": "expanded"})
	require.NoError(t, err)
	assert.Equal(t, Outer{Inner: Inner{Path: "expanded"}}, in)
}

func TestExpandTemplates_PtrStruct(t *testing.T) {
	type Inner struct {
		Path string `template:""`
	}
	type Outer struct {
		Inner *Inner `template:""`
	}
	in := Outer{Inner: &Inner{Path: "${X}"}}
	err := ExpandTemplates(&in, map[string]string{"X": "expanded"})
	require.NoError(t, err)
	require.NotNil(t, in.Inner)
	assert.Equal(t, "expanded", in.Inner.Path)
}

func TestExpandTemplates_PtrStructNil(t *testing.T) {
	type Inner struct {
		Path string `template:""`
	}
	type Outer struct {
		Inner *Inner `template:""`
	}
	in := Outer{Inner: nil}
	err := ExpandTemplates(&in, map[string]string{})
	require.NoError(t, err)
	assert.Nil(t, in.Inner)
}

func TestExpandTemplates_NonTemplateFieldCopied(t *testing.T) {
	type S struct {
		Path      string `template:""`
		Untouched int
	}
	in := S{Path: "${X}", Untouched: 42}
	err := ExpandTemplates(&in, map[string]string{"X": "y"})
	require.NoError(t, err)
	assert.Equal(t, 42, in.Untouched)
	assert.Equal(t, "y", in.Path)
}

func TestExpandTemplates_TemplateDashSkipped(t *testing.T) {
	type S struct {
		Path string `template:"-"`
	}
	in := S{Path: "${X}"}
	err := ExpandTemplates(&in, map[string]string{"X": "y"})
	require.NoError(t, err)
	assert.Equal(t, "${X}", in.Path)
}

func TestExpandTemplates_SliceOfStruct(t *testing.T) {
	type Item struct {
		Path string `template:""`
	}
	type S struct {
		Items []Item `template:""`
	}
	in := S{Items: []Item{{Path: "${X}"}, {Path: "${Y}"}}}
	err := ExpandTemplates(&in, map[string]string{"X": "a", "Y": "b"})
	require.NoError(t, err)
	assert.Equal(t, []Item{{Path: "a"}, {Path: "b"}}, in.Items)
}

func TestExpandTemplates_SliceOfPtrStruct(t *testing.T) {
	type Item struct {
		Path string `template:""`
	}
	type S struct {
		Items []*Item `template:""`
	}
	in := S{Items: []*Item{{Path: "${X}"}, nil, {Path: "${Y}"}}}
	err := ExpandTemplates(&in, map[string]string{"X": "a", "Y": "b"})
	require.NoError(t, err)
	require.Len(t, in.Items, 3)
	assert.Equal(t, "a", in.Items[0].Path)
	assert.Nil(t, in.Items[1])
	assert.Equal(t, "b", in.Items[2].Path)
}

func TestExpandTemplates_TopLevelPtrStruct(t *testing.T) {
	type S struct {
		Path string `template:""`
	}
	in := &S{Path: "${X}"}
	err := ExpandTemplates(in, map[string]string{"X": "y"})
	require.NoError(t, err)
	assert.Equal(t, "y", in.Path)
}

func TestExpandTemplates_TopLevelNil(t *testing.T) {
	type S struct {
		Path string `template:""`
	}
	var in *S
	err := ExpandTemplates(in, map[string]string{})
	require.NoError(t, err)
	assert.Nil(t, in)
}

func TestExpandTemplates_MissingVariable(t *testing.T) {
	type S struct {
		Path string `template:""`
	}
	in := S{Path: "${MISSING}"}
	err := ExpandTemplates(&in, map[string]string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MISSING")
}

func TestExpandTemplates_MapWithoutTagStillExpanded(t *testing.T) {
	type S struct {
		Headers map[string]string
	}
	in := S{Headers: map[string]string{"X-Job": "${JOB_NAME}"}}
	err := ExpandTemplates(&in, map[string]string{"JOB_NAME": "my-job"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"X-Job": "my-job"}, in.Headers)
}

func TestExpandTemplates_NestedStructWithoutTagExplored(t *testing.T) {
	type Inner struct {
		Path string `template:""`
	}
	type Outer struct {
		Inner Inner
	}
	in := Outer{Inner: Inner{Path: "${X}"}}
	err := ExpandTemplates(&in, map[string]string{"X": "expanded"})
	require.NoError(t, err)
	assert.Equal(t, "expanded", in.Inner.Path)
}

func TestExpandTemplates_StringWithoutTagNotExpanded(t *testing.T) {
	type S struct {
		Path string
	}
	in := S{Path: "${X}"}
	err := ExpandTemplates(&in, map[string]string{"X": "y"})
	require.NoError(t, err)
	assert.Equal(t, "${X}", in.Path)
}

func TestExpandTemplates_UnsupportedMapTypeSkipped(t *testing.T) {
	type S struct {
		M map[string]int
	}
	in := S{M: map[string]int{"k": 1}}
	err := ExpandTemplates(&in, map[string]string{})
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"k": 1}, in.M)
}

func TestExpand(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		variables  map[string]string
		want       string
		wantErr    bool
		errContain string
	}{
		{
			name:      "no variables",
			value:     "plain-text",
			variables: map[string]string{},
			want:      "plain-text",
		},
		{
			name:      "single variable",
			value:     "${JOB_NAME}",
			variables: map[string]string{"JOB_NAME": "my-job"},
			want:      "my-job",
		},
		{
			name:  "multiple variables",
			value: "${JOB_NAME}-${JOB_DATE_ISO8601}",
			variables: map[string]string{
				"JOB_NAME":         "infra-snapshot",
				"JOB_DATE_ISO8601": "20260124T103000Z",
			},
			want: "infra-snapshot-20260124T103000Z",
		},
		{
			name:  "env var from allowlist",
			value: "${API_TOKEN}",
			variables: map[string]string{
				"API_TOKEN": "secret123",
			},
			want: "secret123",
		},
		{
			name:       "disallowed env var",
			value:      "${SECRET_KEY}",
			variables:  map[string]string{},
			wantErr:    true,
			errContain: `environment variable "SECRET_KEY" is not in the allowed list`,
		},
		{
			name:       "missing env var",
			value:      "${MISSING_VAR}",
			variables:  map[string]string{"OTHER": "value"},
			wantErr:    true,
			errContain: `environment variable "MISSING_VAR" is not in the allowed list`,
		},
		{
			name:      "multiple errors accumulated",
			value:     "${NOT_ALLOWED}${ALSO_NOT_ALLOWED}",
			variables: map[string]string{},
			wantErr:   true,
		},
		{
			name:      "dollar sign without braces uses short form",
			value:     "$PLAIN",
			variables: map[string]string{"PLAIN": "value"},
			want:      "value",
		},
		{
			name:  "complex path pattern",
			value: "${JOB_NAME}/${JOB_DATE_ISO8601}/${AWS_ACCOUNT_ID}/data.json",
			variables: map[string]string{
				"JOB_NAME":         "infra-collect",
				"JOB_DATE_ISO8601": "20260124T103000Z",
				"AWS_ACCOUNT_ID":   "123456789",
			},
			want: "infra-collect/20260124T103000Z/123456789/data.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Expand(tt.value, tt.variables)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExpandMap(t *testing.T) {
	tests := []struct {
		name       string
		values     map[string]string
		variables  map[string]string
		want       map[string]string
		wantErr    bool
		errContain string
	}{
		{
			name:      "nil map",
			values:    nil,
			variables: map[string]string{},
			want:      nil,
		},
		{
			name:      "empty map",
			values:    map[string]string{},
			variables: map[string]string{},
			want:      map[string]string{},
		},
		{
			name: "map with variables",
			values: map[string]string{
				"X-Job-Name": "${JOB_NAME}",
				"X-Date":     "${JOB_DATE_ISO8601}",
			},
			variables: map[string]string{
				"JOB_NAME":         "my-job",
				"JOB_DATE_ISO8601": "20260124T103000Z",
			},
			want: map[string]string{
				"X-Job-Name": "my-job",
				"X-Date":     "20260124T103000Z",
			},
		},
		{
			name: "map with env var",
			values: map[string]string{
				"Authorization": "Bearer ${TOKEN}",
			},
			variables: map[string]string{
				"TOKEN": "abc123",
			},
			want: map[string]string{
				"Authorization": "Bearer abc123",
			},
		},
		{
			name: "map with error in one value",
			values: map[string]string{
				"Good": "plain",
				"Bad":  "${NOT_ALLOWED}",
			},
			variables:  map[string]string{},
			wantErr:    true,
			errContain: "is not in the allowed list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandMap(tt.values, tt.variables)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
