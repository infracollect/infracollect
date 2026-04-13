package runner

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/zclconf/go-cty/cty"
	"go.uber.org/zap"
)

const collectorCloseTimeout = 30 * time.Second

type Runner struct {
	logger   *zap.Logger
	tmpl     *JobTemplate
	pipeline *Pipeline
	baseCtx  *hcl.EvalContext
	registry *engine.Registry

	collectors map[string]engine.Collector // keyed by "<type>/<id>"
	raw        map[string]engine.Result    // keyed by "<type>/<id>"

	// Incremental mirrors of the step.* and collector.* namespaces, keyed
	// by type then by id. Updated in place as each node completes so
	// childCtxForNode does not rebuild them from scratch.
	stepByType      map[string]map[string]cty.Value
	collectorByType map[string]map[string]cty.Value
}

func New(
	logger *zap.Logger,
	tmpl *JobTemplate,
	registry *engine.Registry,
	allowedEnv []string,
) (*Runner, hcl.Diagnostics) {
	logger.Info("creating runner", zap.String("job_name", tmpl.JobName()))

	baseCtx, err := BuildBaseEvalContext(tmpl, allowedEnv)
	if err != nil {
		return nil, hcl.Diagnostics{&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Failed to build base eval context",
			Detail:   err.Error(),
		}}
	}

	pipeline, diags := BuildPipeline(logger.Named("pipeline"), tmpl, registry)
	if diags.HasErrors() {
		return nil, diags
	}

	return &Runner{
		logger:          logger,
		tmpl:            tmpl,
		pipeline:        pipeline,
		baseCtx:         baseCtx,
		registry:        registry,
		collectors:      make(map[string]engine.Collector),
		raw:             make(map[string]engine.Result),
		stepByType:      make(map[string]map[string]cty.Value),
		collectorByType: make(map[string]map[string]cty.Value),
	}, diags
}

// Run walks the DAG in topological order and executes each node, then
// streams the collected results through the encoder + sink pair described
// by the template's output {} block (defaulting to json + stdout when the
// block is absent).
func (r *Runner) Run(ctx context.Context) (map[string]engine.Result, error) {
	order, err := r.pipeline.dag.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("could not sort DAG: %w", err)
	}

	defer r.closeCollectors()

	for _, node := range order {
		meta, ok := r.pipeline.Meta(node)
		if !ok {
			return nil, fmt.Errorf("pipeline metadata missing for node %s", node.Key())
		}

		switch node.Kind {
		case NodeTypeCollector:
			if err := r.runCollector(ctx, node, meta); err != nil {
				return nil, err
			}
		case NodeTypeStep:
			if err := r.runStep(ctx, node, meta); err != nil {
				return nil, err
			}
		case NodeTypeCollection:
			if err := r.runCollection(ctx, node, meta); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown node kind %q", node.Kind.String())
		}
	}

	if err := r.writeResults(ctx); err != nil {
		return nil, err
	}

	return r.raw, nil
}

// writeResults encodes every collected result through the configured
// encoder and streams it to the configured sink. Keys are sorted so
// concatenated output is reproducible despite Go's randomized map
// iteration. When the output block declares a `steps` filter, only
// the referenced steps are written.
func (r *Runner) writeResults(ctx context.Context) error {
	encoder, sink, err := buildOutputPipeline(ctx, r.tmpl.Output, r.baseCtx, r.tmpl.JobName())
	if err != nil {
		return fmt.Errorf("failed to build output pipeline: %w", err)
	}
	defer func() {
		if err := sink.Close(ctx); err != nil {
			r.logger.Warn("failed to close sink", zap.Error(err))
		}
	}()

	allowed := r.pipeline.OutputSteps()

	ext := encoder.FileExtension()
	keys := make([]string, 0, len(r.raw))
	for k := range r.raw {
		if allowed != nil {
			if _, ok := allowed[k]; !ok {
				continue
			}
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		result := r.raw[key]
		reader, err := encoder.EncodeResult(ctx, result)
		if err != nil {
			return fmt.Errorf("failed to encode result %s: %w", key, err)
		}
		if err := sink.Write(ctx, key+"."+ext, reader); err != nil {
			return fmt.Errorf("failed to write result %s: %w", key, err)
		}

		if len(result.Meta) > 0 {
			metaReader, err := encoder.EncodeMeta(ctx, result.Meta)
			if err != nil {
				return fmt.Errorf("failed to encode meta %s: %w", key, err)
			}
			if err := sink.Write(ctx, key+".meta."+ext, metaReader); err != nil {
				return fmt.Errorf("failed to write meta %s: %w", key, err)
			}
		}
	}
	return nil
}

func (r *Runner) runCollector(ctx context.Context, node Node, meta *NodeMeta) error {
	ectx := r.childCtxForNode()

	collector, diags := r.registry.CreateCollector(node.Type, meta.Body, ectx)
	if diags.HasErrors() {
		return fmt.Errorf("failed to create collector %s/%s: %s", node.Type, node.ID, diags.Error())
	}

	if err := collector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start collector %s/%s: %w", node.Type, node.ID, err)
	}

	r.collectors[nodeKey(node.Type, node.ID)] = collector
	if r.collectorByType[node.Type] == nil {
		r.collectorByType[node.Type] = make(map[string]cty.Value)
	}
	// Sentinel so `collector = collector.<type>.<id>` traversals type-check
	// during step-body decode. resolveStepCollector walks the expression
	// directly rather than evaluating it.
	r.collectorByType[node.Type][node.ID] = cty.EmptyObjectVal
	r.logger.Info("collector started",
		zap.String("type", node.Type),
		zap.String("id", node.ID),
	)
	return nil
}

func (r *Runner) runStep(ctx context.Context, node Node, meta *NodeMeta) error {
	ectx := r.childCtxForNode()

	collector, err := r.resolveStepCollector(node, meta)
	if err != nil {
		return err
	}

	step, diags := r.registry.CreateStep(node.Type, node.ID, collector, meta.Body, ectx)
	if diags.HasErrors() {
		return fmt.Errorf("failed to create step %s/%s: %s", node.Type, node.ID, diags.Error())
	}

	result, err := step.Resolve(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve step %s/%s: %w", node.Type, node.ID, err)
	}

	resultCty, err := resultToCty(result)
	if err != nil {
		return fmt.Errorf("failed to convert result for %s/%s: %w", node.Type, node.ID, err)
	}
	if r.stepByType[node.Type] == nil {
		r.stepByType[node.Type] = make(map[string]cty.Value)
	}
	r.stepByType[node.Type][node.ID] = resultCty
	r.raw[nodeKey(node.Type, node.ID)] = result

	r.logger.Info("step resolved",
		zap.String("type", node.Type),
		zap.String("id", node.ID),
	)
	return nil
}

func (r *Runner) runCollection(ctx context.Context, node Node, meta *NodeMeta) error {
	if meta.ForEach == nil {
		return fmt.Errorf("collection node %s/%s has no for_each expression", node.Type, node.ID)
	}

	baseStepCtx := r.childCtxForNode()

	forVal, diags := meta.ForEach.Value(baseStepCtx)
	if diags.HasErrors() {
		return fmt.Errorf("failed to evaluate for_each for %s/%s: %s", node.Type, node.ID, diags.Error())
	}
	if err := validateForEachValue(forVal); err != nil {
		return fmt.Errorf("for_each for %s/%s is invalid: %w", node.Type, node.ID, err)
	}

	collector, err := r.resolveStepCollector(node, meta)
	if err != nil {
		return err
	}

	iterResults := make(map[string]cty.Value)
	iterRaw := make(map[string]engine.Result)

	it := forVal.ElementIterator()
	for it.Next() {
		key, val := it.Element()
		// validateForEachValue restricts for_each to maps, objects, and
		// sets of strings, so key is always a cty.String.
		keyStr := key.AsString()

		iterCtx := baseStepCtx.NewChild()
		iterCtx.Variables = map[string]cty.Value{
			"each": cty.ObjectVal(map[string]cty.Value{
				"key":   key,
				"value": val,
			}),
		}

		step, diags := r.registry.CreateStep(node.Type, node.ID, collector, meta.Body, iterCtx)
		if diags.HasErrors() {
			return fmt.Errorf("failed to create step %s/%s[%s]: %s", node.Type, node.ID, keyStr, diags.Error())
		}

		result, err := step.Resolve(ctx)
		if err != nil {
			return fmt.Errorf("failed to resolve step %s/%s[%s]: %w", node.Type, node.ID, keyStr, err)
		}

		resultCty, err := resultToCty(result)
		if err != nil {
			return fmt.Errorf("failed to convert result for %s/%s[%s]: %w", node.Type, node.ID, keyStr, err)
		}

		iterResults[keyStr] = resultCty
		iterRaw[keyStr] = result
	}

	// Empty collections need an explicit empty object so traversals into the
	// collection still resolve to a known value.
	var aggregated cty.Value
	if len(iterResults) == 0 {
		aggregated = cty.EmptyObjectVal
	} else {
		aggregated = cty.ObjectVal(iterResults)
	}
	if r.stepByType[node.Type] == nil {
		r.stepByType[node.Type] = make(map[string]cty.Value)
	}
	r.stepByType[node.Type][node.ID] = aggregated
	r.raw[nodeKey(node.Type, node.ID)] = engine.Result{Data: iterRaw}

	r.logger.Info("collection resolved",
		zap.String("type", node.Type),
		zap.String("id", node.ID),
		zap.Int("iterations", len(iterResults)),
	)
	return nil
}

func (r *Runner) resolveStepCollector(node Node, meta *NodeMeta) (engine.Collector, error) {
	if meta.CollectorAddr == nil {
		// Collector-less step kinds (static, exec).
		return nil, nil
	}
	key := nodeKey(meta.CollectorAddr.Type, meta.CollectorAddr.Name)
	c, ok := r.collectors[key]
	if !ok {
		return nil, fmt.Errorf("step %s/%s references unknown collector %s", node.Type, node.ID, key)
	}
	return c, nil
}

func (r *Runner) childCtxForNode() *hcl.EvalContext {
	child := r.baseCtx.NewChild()
	child.Variables = map[string]cty.Value{
		"step":      r.stepNamespace(),
		"collector": r.collectorNamespace(),
	}
	return child
}

func (r *Runner) stepNamespace() cty.Value {
	return wrapByType(r.stepByType)
}

// collectorNamespace returns an object of sentinels keyed by collector type
// and id. The sentinels exist purely so `collector = collector.<type>.<id>`
// traversals type-check during step-body decode; resolveStepCollector walks
// the expression directly rather than evaluating it.
func (r *Runner) collectorNamespace() cty.Value {
	return wrapByType(r.collectorByType)
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

// closeCollectors closes every started collector, continuing past errors so
// each gets a chance to release resources. Uses a fresh context so cleanup
// runs even when the run ctx is already canceled.
func (r *Runner) closeCollectors() {
	ctx, cancel := context.WithTimeout(context.Background(), collectorCloseTimeout)
	defer cancel()
	for key, c := range r.collectors {
		if err := c.Close(ctx); err != nil {
			r.logger.Warn("failed to close collector",
				zap.String("collector", key),
				zap.Error(err),
			)
		}
	}
}

func (r *Runner) Pipeline() *Pipeline { return r.pipeline }

func (r *Runner) EvalContext() *hcl.EvalContext { return r.baseCtx }

func nodeKey(typ, id string) string { return typ + "/" + id }

// validateForEachValue enforces the same rule Terraform applies: for_each
// must evaluate to a map, an object, or a set of strings. Tuples and lists
// are rejected on purpose — numeric indexes are not stable keys, and users
// who want keyed fan-out should project their list into a map first
// (e.g. `{ for x in list : x.id => x }`).
func validateForEachValue(v cty.Value) error {
	if v.IsNull() {
		return fmt.Errorf("for_each cannot be null")
	}
	if !v.IsKnown() {
		return fmt.Errorf("for_each value is not yet known")
	}
	ty := v.Type()
	switch {
	case ty.IsMapType(), ty.IsObjectType():
		return nil
	case ty.IsSetType():
		if ty.ElementType() != cty.String {
			return fmt.Errorf("for_each set must contain strings, got set of %s", ty.ElementType().FriendlyName())
		}
		return nil
	}
	return fmt.Errorf("for_each must be a map, object, or set of strings; got %s — project a list into a map with `{ for x in ... : x.key => x }`", ty.FriendlyName())
}
