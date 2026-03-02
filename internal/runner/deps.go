package runner

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// Known traversal roots. Any traversal whose first segment is not in this set
// is rejected at reference-extraction time.
const (
	RootEnv       = "env"
	RootJob       = "job"
	RootCollector = "collector"
	RootStep      = "step"
	RootEach      = "each"
)

// Reference is a single dependency extracted from an HCL expression. For
// collector / step roots it also carries the first label (Type) and second
// label (Name), so the pipeline can resolve it to a DAG node without a second
// walk.
type Reference struct {
	Root      string // one of the Root* constants
	Type      string // for collector/step only
	Name      string // for collector/step only
	Traversal hcl.Traversal
}

// ReferencesInBody walks every attribute in body, recursing into nested
// blocks, and returns the de-duplicated set of references found.
func ReferencesInBody(body hcl.Body) ([]Reference, hcl.Diagnostics) {
	var refs []Reference
	var diags hcl.Diagnostics
	seen := make(map[string]struct{})
	walkBodyForRefs(body, &refs, &diags, seen)
	return refs, diags
}

func walkBodyForRefs(body hcl.Body, refs *[]Reference, diags *hcl.Diagnostics, seen map[string]struct{}) {
	// JustAttributes honors hiddenAttrs set by prior PartialContent calls
	// (splitStepMeta hides for_each and collector). Reading hclsyntax.Body's
	// Attributes map directly would re-surface them. Diagnostics about
	// sub-blocks are dropped because we walk blocks ourselves below.
	attrs, _ := body.JustAttributes()
	for _, attr := range attrs {
		collectExprRefs(attr.Expr, refs, diags, seen)
	}

	syn, ok := body.(*hclsyntax.Body)
	if !ok {
		return
	}
	for _, block := range syn.Blocks {
		walkBodyForRefs(block.Body, refs, diags, seen)
	}
}

func collectExprRefs(expr hcl.Expression, refs *[]Reference, diags *hcl.Diagnostics, seen map[string]struct{}) {
	r, d := ReferencesInExpression(expr)
	*diags = append(*diags, d...)
	for _, ref := range r {
		key := refDedupeKey(ref)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		*refs = append(*refs, ref)
	}
}

func refDedupeKey(ref Reference) string {
	switch ref.Root {
	case RootCollector, RootStep:
		return ref.Root + "/" + ref.Type + "/" + ref.Name
	default:
		return ref.Root
	}
}

// ReferencesInExpression extracts dependencies from a single HCL expression
// via expr.Variables(). Traversals with unknown roots produce diagnostics at
// the traversal's source range.
func ReferencesInExpression(expr hcl.Expression) ([]Reference, hcl.Diagnostics) {
	if expr == nil {
		return nil, nil
	}

	var refs []Reference
	var diags hcl.Diagnostics

	for _, t := range expr.Variables() {
		ref, d := classifyTraversal(t)
		diags = append(diags, d...)
		if ref != nil {
			refs = append(refs, *ref)
		}
	}

	return refs, diags
}

func classifyTraversal(t hcl.Traversal) (*Reference, hcl.Diagnostics) {
	if len(t) == 0 {
		return nil, nil
	}

	root, ok := t[0].(hcl.TraverseRoot)
	if !ok {
		return nil, nil
	}

	switch root.Name {
	case RootEnv, RootJob, RootEach:
		return &Reference{Root: root.Name, Traversal: t}, nil

	case RootCollector, RootStep:
		// Expected shape: collector.<type>.<id>(.rest) or step.<type>.<id>(.rest).
		// Anything shorter is a static error.
		if len(t) < 3 {
			return nil, hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Incomplete %s reference", root.Name),
				Detail: fmt.Sprintf(
					"Expected %s.<type>.<name>(.<attr>...), got a shorter traversal.",
					root.Name,
				),
				Subject: t.SourceRange().Ptr(),
			}}
		}

		typeAttr, ok := t[1].(hcl.TraverseAttr)
		if !ok {
			return nil, hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Invalid %s reference", root.Name),
				Detail: fmt.Sprintf(
					"Expected an attribute (block type) after %s, got a different traversal step.",
					root.Name,
				),
				Subject: t.SourceRange().Ptr(),
			}}
		}
		nameAttr, ok := t[2].(hcl.TraverseAttr)
		if !ok {
			return nil, hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Invalid %s reference", root.Name),
				Detail: fmt.Sprintf(
					"Expected an attribute (block name) after %s.%s, got a different traversal step.",
					root.Name, typeAttr.Name,
				),
				Subject: t.SourceRange().Ptr(),
			}}
		}

		return &Reference{
			Root:      root.Name,
			Type:      typeAttr.Name,
			Name:      nameAttr.Name,
			Traversal: t,
		}, nil

	default:
		return nil, hcl.Diagnostics{&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Unknown reference",
			Detail: fmt.Sprintf(
				"%q is not a known namespace. Use one of: env, job, collector, step, each.",
				root.Name,
			),
			Subject: t.SourceRange().Ptr(),
		}}
	}
}
