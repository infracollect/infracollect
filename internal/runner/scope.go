package runner

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/zclconf/go-cty/cty"
)

// scope is the lexical scope that later nodes resolve `step.*` and
// `collector.*` references against. It owns the incremental cty mirrors of
// those two namespaces and is the only place that builds an hcl.EvalContext
// for a node. Results and live collectors live on the Runner — scope holds
// only what a node needs to *see*, not what the run produces or manages.
//
// Execution is a sequential topological walk, so scope needs no locking.
type scope struct {
	base *hcl.EvalContext

	// step.<type>.<id> -> result value, and collector.<type>.<id> -> sentinel.
	// Updated in place as each node completes so child() does not rebuild them
	// from scratch.
	stepByType      map[string]map[string]cty.Value
	collectorByType map[string]map[string]cty.Value
}

func newScope(base *hcl.EvalContext) *scope {
	return &scope{
		base:            base,
		stepByType:      make(map[string]map[string]cty.Value),
		collectorByType: make(map[string]map[string]cty.Value),
	}
}

// recordCollector stamps a sentinel under collector.<type>.<id> so that a
// `collector = collector.<type>.<id>` traversal type-checks during step-body
// decode. The sentinel is never evaluated: resolveStepCollector walks the
// binding expression directly and looks the live collector up by address.
func (s *scope) recordCollector(node Node) {
	put(s.collectorByType, node.Type, node.ID, cty.EmptyObjectVal)
}

// recordResult exposes a single step's result under step.<type>.<id>.
func (s *scope) recordResult(node Node, result engine.Result) error {
	v, err := resultToCty(result)
	if err != nil {
		return err
	}
	put(s.stepByType, node.Type, node.ID, v)
	return nil
}

// recordCollection exposes a for_each step's per-key results as a single
// object under step.<type>.<id>, keyed by each.key. An empty collection
// records an empty object so traversals into it still resolve to a known
// value.
func (s *scope) recordCollection(node Node, results map[string]engine.Result) error {
	iter := make(map[string]cty.Value, len(results))
	for key, result := range results {
		v, err := resultToCty(result)
		if err != nil {
			return err
		}
		iter[key] = v
	}

	aggregated := cty.EmptyObjectVal
	if len(iter) > 0 {
		aggregated = cty.ObjectVal(iter)
	}
	put(s.stepByType, node.Type, node.ID, aggregated)
	return nil
}

// child returns a fresh per-node EvalContext carrying the step.* and
// collector.* namespaces as they stand.
func (s *scope) child() *hcl.EvalContext {
	child := s.base.NewChild()
	child.Variables = map[string]cty.Value{
		"step":      wrapByType(s.stepByType),
		"collector": wrapByType(s.collectorByType),
	}
	return child
}

// withEach layers `each = { key, value }` onto base, an already-built per-node
// context, for one iteration of a for_each step. base is built once per
// collection and reused across iterations — NewChild keeps each iteration's
// `each` in its own layer without rebuilding the step.*/collector.* namespaces.
func withEach(base *hcl.EvalContext, key, val cty.Value) *hcl.EvalContext {
	child := base.NewChild()
	child.Variables = map[string]cty.Value{
		"each": cty.ObjectVal(map[string]cty.Value{
			"key":   key,
			"value": val,
		}),
	}
	return child
}

func put(byType map[string]map[string]cty.Value, typ, id string, v cty.Value) {
	if byType[typ] == nil {
		byType[typ] = make(map[string]cty.Value)
	}
	byType[typ][id] = v
}

func wrapByType(byType map[string]map[string]cty.Value) cty.Value {
	if len(byType) == 0 {
		return cty.EmptyObjectVal
	}
	obj := make(map[string]cty.Value, len(byType))
	for typ, ids := range byType {
		obj[typ] = cty.ObjectVal(ids)
	}
	return cty.ObjectVal(obj)
}

func resultToCty(result engine.Result) (cty.Value, error) {
	dataCty, err := engine.AnyToCty(result.Data)
	if err != nil {
		return cty.NilVal, err
	}

	metaVal := cty.EmptyObjectVal
	if len(result.Meta) > 0 {
		m := make(map[string]cty.Value, len(result.Meta))
		for k, v := range result.Meta {
			m[k] = cty.StringVal(v)
		}
		metaVal = cty.ObjectVal(m)
	}

	return cty.ObjectVal(map[string]cty.Value{
		"data": dataCty,
		"meta": metaVal,
	}), nil
}
