package runner

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/infracollect/infracollect/internal/engine"
	"go.uber.org/zap"
)

// CollectorAddr is the parsed `collector.<type>.<id>` binding of a step.
type CollectorAddr struct {
	Type string
	Name string
}

// Pipeline is the resolvable shape of a collect job: a DAG of collector and
// step nodes with per-node metadata attached for execution.
type Pipeline struct {
	dag  *DirectedAcyclicGraph
	meta map[Node]*NodeMeta

	// outputSteps is the set of "type/id" keys that the output block's
	// `steps` attribute selected. nil means "all steps".
	outputSteps map[string]struct{}
}

// NodeMeta is pipeline-local so the DAG core stays comparable.
type NodeMeta struct {
	Body          hcl.Body
	Refs          []Reference
	ForEach       hcl.Expression // nil unless this is a Collection node
	CollectorAddr *CollectorAddr // step-only; parsed collector binding
	DefRange      hcl.Range
}

func (p *Pipeline) Dag() *DirectedAcyclicGraph { return p.dag }

func (p *Pipeline) Meta(n Node) (*NodeMeta, bool) {
	m, ok := p.meta[n]
	return m, ok
}

func (p *Pipeline) OutputSteps() map[string]struct{} { return p.outputSteps }

// BuildPipeline extracts references via HCL's native Variables() walk and
// builds a DAG with one node per collector and one per step. Steps that
// declared a for_each become NodeTypeCollection. Structural errors
// (unknown kind, dangling reference, cycle, each.* outside a collection)
// surface as hcl.Diagnostics with source ranges intact.
func BuildPipeline(logger *zap.Logger, tmpl *JobTemplate, registry *engine.Registry) (*Pipeline, hcl.Diagnostics) {
	logger.Info("building pipeline", zap.String("job_name", tmpl.JobName()))

	p := &Pipeline{
		dag:  NewDirectedAcyclicGraph(),
		meta: make(map[Node]*NodeMeta),
	}
	var diags hcl.Diagnostics

	knownCollectors := registry.AvailableCollectors()

	// Ordered list of nodes in source order, used in the second pass so
	// diagnostics surface deterministically rather than in Go map order.
	var nodes []Node

	// First pass: add all nodes so subsequent edge resolution can tell the
	// difference between "referenced node exists" and "dangling reference".
	for _, c := range tmpl.Collectors {
		if !slices.Contains(knownCollectors, c.Type) {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Unknown collector type",
				Detail:   fmt.Sprintf("Collector type %q is not registered. Expected one of: %s.", c.Type, strings.Join(knownCollectors, ", ")),
				Subject:  c.DefRange.Ptr(),
			})
			continue
		}
		node := Node{Kind: NodeTypeCollector, Type: c.Type, ID: c.Name}
		if err := p.dag.AddNode(node); err != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Duplicate collector node",
				Detail:   err.Error(),
				Subject:  c.DefRange.Ptr(),
			})
			continue
		}

		refs, rd := ReferencesInBody(c.Body)
		diags = append(diags, rd...)

		p.meta[node] = &NodeMeta{
			Body:     c.Body,
			Refs:     refs,
			DefRange: c.DefRange,
		}
		nodes = append(nodes, node)
	}

	for _, s := range tmpl.Steps {
		desc, known := registry.StepDescriptor(s.Type)
		if !known {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Unknown step type",
				Detail: fmt.Sprintf(
					"Step type %q is not registered. Expected one of: %s.",
					s.Type, strings.Join(registry.AvailableSteps(), ", "),
				),
				Subject: s.DefRange.Ptr(),
			})
			continue
		}
		kind := NodeTypeStep
		if s.ForEach != nil {
			kind = NodeTypeCollection
		}
		node := Node{Kind: kind, Type: s.Type, ID: s.Name}
		if err := p.dag.AddNode(node); err != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Duplicate step node",
				Detail:   err.Error(),
				Subject:  s.DefRange.Ptr(),
			})
			continue
		}

		refs, rd := ReferencesInBody(s.Body)
		diags = append(diags, rd...)

		if s.ForEach != nil {
			forEachRefs, fd := ReferencesInExpression(s.ForEach)
			diags = append(diags, fd...)
			refs = append(refs, forEachRefs...)
		}

		var collectorAddr *CollectorAddr
		switch {
		case s.Collector == nil && desc.RequiresCollector:
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Step %q requires a collector", s.Type),
				Detail: fmt.Sprintf(
					"Step %q must declare `collector = collector.<kind>.<id>`. Accepted collector kinds: %s.",
					s.Name, strings.Join(desc.AllowedCollectorKinds, ", "),
				),
				Subject: s.DefRange.Ptr(),
			})
		case s.Collector != nil && len(desc.AllowedCollectorKinds) == 0:
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Step %q must not declare a collector", s.Type),
				Detail: fmt.Sprintf(
					"Step %q is collector-less; remove the `collector` attribute.",
					s.Name,
				),
				Subject: s.Collector.Range().Ptr(),
			})
		case s.Collector != nil:
			addr, trav, cd := parseCollectorBinding(s.Collector)
			diags = append(diags, cd...)
			if !cd.HasErrors() {
				if !slices.Contains(desc.AllowedCollectorKinds, addr.Type) {
					diags = append(diags, &hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  fmt.Sprintf("Incompatible collector for step %q", s.Type),
						Detail: fmt.Sprintf(
							"Step %q accepts collector kinds %v, but is bound to kind %q.",
							s.Name, desc.AllowedCollectorKinds, addr.Type,
						),
						Subject: s.Collector.Range().Ptr(),
					})
				} else {
					collectorAddr = &addr
					refs = append(refs, Reference{
						Root:      RootCollector,
						Type:      addr.Type,
						Name:      addr.Name,
						Traversal: trav,
					})
				}
			}
		}

		p.meta[node] = &NodeMeta{
			Body:          s.Body,
			Refs:          refs,
			ForEach:       s.ForEach,
			CollectorAddr: collectorAddr,
			DefRange:      s.DefRange,
		}
		nodes = append(nodes, node)
	}

	if diags.HasErrors() {
		return nil, diags
	}

	// Second pass: resolve references to DAG edges. Walking `nodes` (source
	// order) instead of the meta map keeps diagnostics deterministic.
	for _, to := range nodes {
		meta := p.meta[to]
		for _, ref := range meta.Refs {
			ed := p.addEdgeForRef(to, ref)
			diags = append(diags, ed...)
		}
	}

	if diags.HasErrors() {
		return nil, diags
	}

	if _, err := p.dag.TopologicalSort(); err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Cycle in collect job DAG",
			Detail:   err.Error(),
		})
		return nil, diags
	}

	// Validate the output.steps filter against the known step nodes.
	if tmpl.Output != nil && tmpl.Output.Steps != nil {
		od := p.validateOutputSteps(tmpl.Output.Steps)
		diags = append(diags, od...)
		if diags.HasErrors() {
			return nil, diags
		}
	}

	return p, diags
}

func (p *Pipeline) addEdgeForRef(to Node, ref Reference) hcl.Diagnostics {
	switch ref.Root {
	case RootEnv, RootJob:
		return nil

	case RootEach:
		if to.Kind != NodeTypeCollection {
			return hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "each.* used outside a for_each step",
				Detail: fmt.Sprintf(
					"%s %q references each.* but is not declared with for_each.",
					to.Kind.String(), to.ID,
				),
				Subject: ref.Traversal.SourceRange().Ptr(),
			}}
		}
		return nil

	case RootCollector, RootStep:
		from, ok := p.resolveRefNode(ref)
		if !ok {
			return hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Reference to unknown %s", ref.Root),
				Detail: fmt.Sprintf(
					"%s.%s.%s is not declared in this job.",
					ref.Root, ref.Type, ref.Name,
				),
				Subject: ref.Traversal.SourceRange().Ptr(),
			}}
		}
		if err := p.dag.AddEdgeUnchecked(from, to); err != nil {
			return hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Cannot add edge to DAG",
				Detail:   err.Error(),
				Subject:  ref.Traversal.SourceRange().Ptr(),
			}}
		}
		return nil
	}

	return nil
}

// resolveRefNode finds the node a collector/step reference points at. Step
// references can resolve to either NodeTypeStep or NodeTypeCollection.
func (p *Pipeline) resolveRefNode(ref Reference) (Node, bool) {
	switch ref.Root {
	case RootCollector:
		n := Node{Kind: NodeTypeCollector, Type: ref.Type, ID: ref.Name}
		if _, ok := p.meta[n]; ok {
			return n, true
		}
	case RootStep:
		for _, kind := range []NodeType{NodeTypeStep, NodeTypeCollection} {
			n := Node{Kind: kind, Type: ref.Type, ID: ref.Name}
			if _, ok := p.meta[n]; ok {
				return n, true
			}
		}
	}
	return Node{}, false
}

// parseTraversalRef validates that expr is a direct 3-segment traversal of
// the form <expectedRoot>.<type>.<id>. Returns the type and id strings, the
// raw traversal, and any diagnostics. The summary is used in error messages
// (e.g. "Invalid collector binding").
func parseTraversalRef(expr hcl.Expression, expectedRoot, summary string) (typeName, idName string, trav hcl.Traversal, diags hcl.Diagnostics) {
	invalid := func(detail string) hcl.Diagnostics {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  summary,
			Detail:   detail,
			Subject:  expr.Range().Ptr(),
		}}
	}

	scopeTrav, ok := expr.(*hclsyntax.ScopeTraversalExpr)
	if !ok {
		return "", "", nil, invalid(fmt.Sprintf(
			"Must be a direct traversal of the form `%s.<type>.<id>`. "+
				"Conditionals, function calls, string interpolations, "+
				"arithmetic, and concatenation are not permitted.",
			expectedRoot,
		))
	}

	t := scopeTrav.Traversal
	if len(t) != 3 {
		return "", "", nil, invalid(fmt.Sprintf(
			"Must be exactly `%s.<type>.<id>`; got %d segments.",
			expectedRoot, len(t),
		))
	}

	root, ok := t[0].(hcl.TraverseRoot)
	if !ok || root.Name != expectedRoot {
		return "", "", nil, invalid(fmt.Sprintf(
			"Must start with the `%s` namespace.", expectedRoot,
		))
	}
	typeAttr, ok := t[1].(hcl.TraverseAttr)
	if !ok {
		return "", "", nil, invalid(fmt.Sprintf(
			"Expected a type after `%s.`.", expectedRoot,
		))
	}
	nameAttr, ok := t[2].(hcl.TraverseAttr)
	if !ok {
		return "", "", nil, invalid(fmt.Sprintf(
			"Expected an id after `%s.%s.`.", expectedRoot, typeAttr.Name,
		))
	}

	return typeAttr.Name, nameAttr.Name, t, nil
}

// parseCollectorBinding validates a step's `collector = ...` expression. Only
// the exact traversal shape `collector.<type>.<id>` is accepted.
func parseCollectorBinding(expr hcl.Expression) (CollectorAddr, hcl.Traversal, hcl.Diagnostics) {
	typeName, idName, trav, diags := parseTraversalRef(expr, RootCollector, "Invalid collector binding")
	if diags.HasErrors() {
		return CollectorAddr{}, nil, diags
	}
	return CollectorAddr{Type: typeName, Name: idName}, trav, nil
}

// validateOutputSteps parses and validates the output.steps expression
// against the known step nodes in the pipeline. Each element must be a
// direct traversal of the form step.<type>.<id>. An empty list is
// rejected. On success the validated keys are stored in p.outputSteps.
func (p *Pipeline) validateOutputSteps(expr hcl.Expression) hcl.Diagnostics {
	tuple, ok := expr.(*hclsyntax.TupleConsExpr)
	if !ok {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Invalid output steps",
			Detail:   "The `steps` attribute must be a list of step references (step.<type>.<id>).",
			Subject:  expr.Range().Ptr(),
		}}
	}
	if len(tuple.Exprs) == 0 {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Empty output steps",
			Detail:   "The `steps` list must not be empty; omit the attribute to include all steps.",
			Subject:  expr.Range().Ptr(),
		}}
	}

	allowed := make(map[string]struct{}, len(tuple.Exprs))
	var diags hcl.Diagnostics

	for _, elem := range tuple.Exprs {
		typeName, idName, _, ed := parseTraversalRef(elem, RootStep, "Invalid step reference")
		if ed.HasErrors() {
			diags = append(diags, ed...)
			continue
		}

		if _, found := p.resolveRefNode(Reference{Root: RootStep, Type: typeName, Name: idName}); !found {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Reference to unknown step",
				Detail: fmt.Sprintf(
					"step.%s.%s is not declared in this job.",
					typeName, idName,
				),
				Subject: elem.Range().Ptr(),
			})
			continue
		}

		allowed[nodeKey(typeName, idName)] = struct{}{}
	}

	if !diags.HasErrors() {
		p.outputSteps = allowed
	}
	return diags
}
