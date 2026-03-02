package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestBuildBaseEvalContext_EnvAllowlist(t *testing.T) {
	t.Setenv("INFRACOLLECT_TEST_FOO", "bar")
	t.Setenv("INFRACOLLECT_TEST_BAZ", "qux")

	tmpl := &JobTemplate{Job: &JobBlock{Name: "j"}}
	ctx, err := BuildBaseEvalContext(tmpl, []string{"INFRACOLLECT_TEST_FOO", "INFRACOLLECT_TEST_BAZ"})
	require.NoError(t, err)

	envVal := ctx.Variables["env"]
	require.True(t, envVal.Type().IsObjectType())
	assert.Equal(t, cty.StringVal("bar"), envVal.GetAttr("INFRACOLLECT_TEST_FOO"))
	assert.Equal(t, cty.StringVal("qux"), envVal.GetAttr("INFRACOLLECT_TEST_BAZ"))
}

func TestBuildBaseEvalContext_MissingEnvVar(t *testing.T) {
	tmpl := &JobTemplate{Job: &JobBlock{Name: "j"}}
	_, err := BuildBaseEvalContext(tmpl, []string{"INFRACOLLECT_DEFINITELY_MISSING"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "INFRACOLLECT_DEFINITELY_MISSING")
	assert.ErrorContains(t, err, "not set")
}

func TestBuildBaseEvalContext_EmptyAllowlist(t *testing.T) {
	tmpl := &JobTemplate{Job: &JobBlock{Name: "j"}}
	ctx, err := BuildBaseEvalContext(tmpl, nil)
	require.NoError(t, err)

	envVal := ctx.Variables["env"]
	// Empty allowlist must still produce a traversable object (the empty
	// sentinel), not a null or a panic from cty.ObjectVal on an empty map.
	require.True(t, envVal.Type().IsObjectType())
	assert.True(t, envVal.Type().Equals(cty.EmptyObject))
}

func TestBuildBaseEvalContext_JobNameBinding(t *testing.T) {
	tmpl := &JobTemplate{Job: &JobBlock{Name: "my-job"}}
	ctx, err := BuildBaseEvalContext(tmpl, nil)
	require.NoError(t, err)

	jobVal := ctx.Variables["job"]
	assert.Equal(t, cty.StringVal("my-job"), jobVal.GetAttr("name"))
}

func TestBuildBaseEvalContext_TimeFunctions(t *testing.T) {
	tmpl := &JobTemplate{Job: &JobBlock{Name: "j"}}
	ctx, err := BuildBaseEvalContext(tmpl, nil)
	require.NoError(t, err)

	// Confirm the datetime functions are wired in. We don't assert on exact
	// values — only that the names exist, so this test doesn't drift with
	// each upstream revision of the function set.
	for _, name := range []string{"timestamp", "timeadd", "formatdate"} {
		_, ok := ctx.Functions[name]
		assert.True(t, ok, "function %q should be registered", name)
	}
}
