package engine

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// EvalBodyToMap is the error-returning sibling of BodyToMap. The what
// argument is the noun used in the wrapped error message (e.g. "exec step
// input block").
func EvalBodyToMap(body hcl.Body, ctx *hcl.EvalContext, what string) (map[string]any, error) {
	m, diags := BodyToMap(body, ctx)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to evaluate %s: %w", what, diags)
	}
	return m, nil
}

// BodyToMap evaluates every attribute in body against ctx. The body must
// contain only attributes; nested blocks produce a diagnostic.
func BodyToMap(body hcl.Body, ctx *hcl.EvalContext) (map[string]any, hcl.Diagnostics) {
	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		return nil, diags
	}

	out := make(map[string]any, len(attrs))
	for name, attr := range attrs {
		val, d := attr.Expr.Value(ctx)
		diags = append(diags, d...)
		if d.HasErrors() {
			continue
		}
		gv, err := CtyToAny(val)
		if err != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Unrepresentable attribute value",
				Detail:   fmt.Sprintf("Attribute %q could not be converted to a generic value: %s", name, err),
				Subject:  attr.Expr.Range().Ptr(),
			})
			continue
		}
		out[name] = gv
	}
	return out, diags
}

// AnyToCty converts a generic Go value into a cty.Value by round-tripping
// through JSON.
func AnyToCty(v any) (cty.Value, error) {
	if v == nil {
		return cty.NullVal(cty.DynamicPseudoType), nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return cty.NilVal, fmt.Errorf("marshal to JSON: %w", err)
	}
	ty, err := ctyjson.ImpliedType(data)
	if err != nil {
		return cty.NilVal, fmt.Errorf("infer cty type from JSON: %w", err)
	}
	val, err := ctyjson.Unmarshal(data, ty)
	if err != nil {
		return cty.NilVal, fmt.Errorf("decode JSON into cty: %w", err)
	}
	return val, nil
}

// CtyToAny converts a cty.Value into a generic Go value. Numbers are decoded
// as json.Number so integer precision survives the round-trip.
func CtyToAny(v cty.Value) (any, error) {
	if !v.IsKnown() {
		return nil, fmt.Errorf("value is not known")
	}
	if v.IsNull() {
		return nil, nil
	}
	data, err := ctyjson.Marshal(v, v.Type())
	if err != nil {
		return nil, fmt.Errorf("marshal cty value to JSON: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var out any
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("decode JSON into generic value: %w", err)
	}
	return out, nil
}
