package runner

import (
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2"
)

// Plan is the resolved execution plan for a job, suitable for rendering
// in multiple formats (table, JSON, DOT, etc.).
type Plan struct {
	Job    string     `json:"job"`
	Nodes  []PlanNode `json:"nodes"` // topological order
	Output PlanOutput `json:"output"`
}

// PlanNode describes a single node in the execution plan.
type PlanNode struct {
	Kind      string   `json:"kind"` // "collector", "step", "collection"
	Type      string   `json:"type"`
	ID        string   `json:"id"`
	Collector string   `json:"collector,omitempty"`  // "type/id" if bound
	DependsOn []string `json:"depends_on,omitempty"` // "type/id" of predecessors
	ForEach   bool     `json:"for_each,omitempty"`

	// ForEachResolved is true when the for_each expression depends only on the
	// base context (literals, env, job vars) and was evaluated at plan time.
	// When false on a for_each node, the iteration set is computed at runtime
	// from upstream collector/step data.
	ForEachResolved bool     `json:"for_each_resolved,omitempty"`
	ForEachKeys     []string `json:"for_each_keys,omitempty"` // resolved iteration keys, sorted
}

// PlanOutput describes the configured output pipeline.
type PlanOutput struct {
	Encoding string   `json:"encoding"`
	Sink     string   `json:"sink"`
	Archive  string   `json:"archive,omitempty"`
	Steps    []string `json:"steps,omitempty"` // nil means all steps
}

// DryRun builds an execution plan without running any collectors or steps.
func (r *Runner) DryRun() (*Plan, error) {
	order, err := r.pipeline.dag.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("could not sort DAG: %w", err)
	}

	// Build a lookup from node key to Node for human-readable formatting.
	nodesByKey := make(map[string]Node, len(order))
	for _, n := range order {
		nodesByKey[n.Key()] = n
	}

	// Build reverse-edge index: for each node, which nodes must run before it.
	edges := r.pipeline.dag.Edges()
	reverseEdges := make(map[string][]string)
	for from, tos := range edges {
		for _, to := range tos {
			reverseEdges[to] = append(reverseEdges[to], from)
		}
	}

	nodes := make([]PlanNode, 0, len(order))
	for _, node := range order {
		meta, _ := r.pipeline.Meta(node)

		pn := PlanNode{
			Kind: node.Kind.String(),
			Type: node.Type,
			ID:   node.ID,
		}

		if meta != nil {
			if meta.CollectorAddr != nil {
				pn.Collector = nodeKey(meta.CollectorAddr.Type, meta.CollectorAddr.Name)
			}
			if meta.ForEach != nil {
				pn.ForEach = true
				if keys, ok := resolveForEachKeys(meta.ForEach, r.baseCtx); ok {
					pn.ForEachResolved = true
					pn.ForEachKeys = keys
				}
			}
		}

		if deps := reverseEdges[node.Key()]; len(deps) > 0 {
			readable := make([]string, 0, len(deps))
			for _, depKey := range deps {
				if dep, ok := nodesByKey[depKey]; ok {
					readable = append(readable, nodeKey(dep.Type, dep.ID))
				}
			}
			sort.Strings(readable)
			pn.DependsOn = readable
		}

		nodes = append(nodes, pn)
	}

	output := buildPlanOutput(r.tmpl.Output, r.pipeline.OutputSteps())

	return &Plan{
		Job:    r.tmpl.JobName(),
		Nodes:  nodes,
		Output: output,
	}, nil
}

// resolveForEachKeys evaluates a for_each expression at plan time against the
// base context (env, job vars, literals). It returns the sorted iteration keys
// and true only when the expression resolves to a known, valid for_each value.
// Expressions that reference upstream collector/step data fail to evaluate
// against the base context and are reported as unresolved (ok == false), since
// their iteration set is only known once those nodes run.
func resolveForEachKeys(expr hcl.Expression, ctx *hcl.EvalContext) (keys []string, ok bool) {
	val, diags := expr.Value(ctx)
	if diags.HasErrors() || val.IsNull() || !val.IsKnown() {
		return nil, false
	}
	if err := validateForEachValue(val); err != nil {
		return nil, false
	}

	it := val.ElementIterator()
	for it.Next() {
		k, _ := it.Element()
		keys = append(keys, k.AsString())
	}
	sort.Strings(keys)
	return keys, true
}

func buildPlanOutput(ob *OutputBlock, outputSteps map[string]struct{}) PlanOutput {
	po := PlanOutput{
		Encoding: "json",
		Sink:     "stdout",
	}

	if ob == nil {
		return po
	}

	if ob.Encoding != nil {
		po.Encoding = ob.Encoding.Kind
	}
	if ob.Sink != nil {
		po.Sink = ob.Sink.Kind
	}
	if ob.Archive != nil {
		po.Archive = ob.Archive.Kind
	}

	if outputSteps != nil {
		steps := make([]string, 0, len(outputSteps))
		for k := range outputSteps {
			steps = append(steps, k)
		}
		sort.Strings(steps)
		po.Steps = steps
	}

	return po
}
