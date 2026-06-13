package runner

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

// evalExpr parses and evaluates an HCL expression against ctx, the way a node
// body references resolve at run time.
func evalExpr(t *testing.T, ctx *hcl.EvalContext, expr string) cty.Value {
	t.Helper()
	e, diags := hclsyntax.ParseExpression([]byte(expr), "test.hcl", hcl.InitialPos)
	require.False(t, diags.HasErrors(), "parse: %s", diags.Error())
	v, diags := e.Value(ctx)
	require.False(t, diags.HasErrors(), "eval %q: %s", expr, diags.Error())
	return v
}

func newTestScope() *scope {
	return newScope(&hcl.EvalContext{})
}

// assertCty compares cty values by mathematical equality. Numbers that survive
// a JSON round-trip carry a wider big.Float precision than NumberIntVal, so a
// deep reflect compare would spuriously fail even when the values are equal.
func assertCty(t *testing.T, want, got cty.Value) {
	t.Helper()
	assert.True(t, want.RawEquals(got), "want %#v, got %#v", want, got)
}

func TestScope_EmptyNamespaces(t *testing.T) {
	ctx := newTestScope().child()
	assert.True(t, evalExpr(t, ctx, "step").RawEquals(cty.EmptyObjectVal))
	assert.True(t, evalExpr(t, ctx, "collector").RawEquals(cty.EmptyObjectVal))
}

func TestScope_RecordResult(t *testing.T) {
	s := newTestScope()
	require.NoError(t, s.recordResult(
		Node{Kind: NodeTypeStep, Type: "terraform", ID: "pods"},
		engine.Result{Data: map[string]any{"count": 3}, Meta: map[string]string{"source": "k8s"}},
	))

	ctx := s.child()
	assertCty(t, cty.NumberIntVal(3), evalExpr(t, ctx, "step.terraform.pods.data.count"))
	assertCty(t, cty.StringVal("k8s"), evalExpr(t, ctx, "step.terraform.pods.meta.source"))
}

func TestScope_RecordCollector_Sentinel(t *testing.T) {
	s := newTestScope()
	s.recordCollector(Node{Kind: NodeTypeCollector, Type: "terraform", ID: "k8s"})

	// The sentinel exists only so a `collector.<type>.<id>` traversal
	// type-checks; it carries no data.
	v := evalExpr(t, s.child(), "collector.terraform.k8s")
	assert.True(t, v.RawEquals(cty.EmptyObjectVal))
}

func TestScope_RecordCollection_KeyedByEach(t *testing.T) {
	s := newTestScope()
	require.NoError(t, s.recordCollection(
		Node{Kind: NodeTypeCollection, Type: "http", ID: "pages"},
		map[string]engine.Result{
			"a": {Data: map[string]any{"n": 1}},
			"b": {Data: map[string]any{"n": 2}},
		},
	))

	ctx := s.child()
	assertCty(t, cty.NumberIntVal(1), evalExpr(t, ctx, "step.http.pages.a.data.n"))
	assertCty(t, cty.NumberIntVal(2), evalExpr(t, ctx, "step.http.pages.b.data.n"))
}

func TestScope_RecordCollection_EmptyResolvesToKnownObject(t *testing.T) {
	s := newTestScope()
	require.NoError(t, s.recordCollection(
		Node{Kind: NodeTypeCollection, Type: "http", ID: "pages"},
		map[string]engine.Result{},
	))

	// An empty collection must still resolve, so downstream traversals into
	// it do not fail with "unknown object".
	assert.True(t, evalExpr(t, s.child(), "step.http.pages").RawEquals(cty.EmptyObjectVal))
}

func TestWithEach_LayersEachOverNamespaces(t *testing.T) {
	s := newTestScope()
	require.NoError(t, s.recordResult(
		Node{Kind: NodeTypeStep, Type: "terraform", ID: "pods"},
		engine.Result{Data: "x"},
	))

	ctx := withEach(s.child(), cty.StringVal("k1"), cty.NumberIntVal(7))

	// each.* is visible for this iteration...
	assertCty(t, cty.StringVal("k1"), evalExpr(t, ctx, "each.key"))
	assertCty(t, cty.NumberIntVal(7), evalExpr(t, ctx, "each.value"))
	// ...and the parent step.* namespace is still reachable.
	assertCty(t, cty.StringVal("x"), evalExpr(t, ctx, "step.terraform.pods.data"))
}
