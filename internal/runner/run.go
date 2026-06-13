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

	// scope owns the step.* / collector.* namespaces and builds each node's
	// EvalContext from them.
	scope *scope
}

func New(
	logger *zap.Logger,
	tmpl *JobTemplate,
	registry *engine.Registry,
	allowedEnv []string,
) (*Runner, hcl.Diagnostics) {
	logger.Debug("creating runner", zap.String("job_name", tmpl.JobName()))

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
		logger:     logger,
		tmpl:       tmpl,
		pipeline:   pipeline,
		baseCtx:    baseCtx,
		registry:   registry,
		collectors: make(map[string]engine.Collector),
		raw:        make(map[string]engine.Result),
		scope:      newScope(baseCtx),
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

// writeResults feeds every collected result to the ResultWriter built from the
// output block. Keys are sorted so concatenated output is reproducible despite
// Go's randomized map iteration. When the output block declares a `steps`
// filter, only the referenced steps are written.
func (r *Runner) writeResults(ctx context.Context) error {
	writer, err := buildResultWriter(ctx, r.tmpl.Output, r.baseCtx, r.tmpl.JobName())
	if err != nil {
		return fmt.Errorf("failed to build result writer: %w", err)
	}
	defer func() {
		if err := writer.Close(ctx); err != nil {
			r.logger.Warn("failed to close result writer", zap.Error(err))
		}
	}()

	allowed := r.pipeline.OutputSteps()
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
		if err := writer.Write(ctx, key, r.raw[key]); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) runCollector(ctx context.Context, node Node, meta *NodeMeta) error {
	ectx := r.scope.child()

	collector, diags := r.registry.CreateCollector(node.Type, meta.Body, ectx)
	if diags.HasErrors() {
		return fmt.Errorf("failed to create collector %s/%s: %s", node.Type, node.ID, diags.Error())
	}

	if err := collector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start collector %s/%s: %w", node.Type, node.ID, err)
	}

	r.collectors[nodeKey(node.Type, node.ID)] = collector
	r.scope.recordCollector(node)
	r.logger.Info("collector started",
		zap.String("type", node.Type),
		zap.String("id", node.ID),
	)
	return nil
}

func (r *Runner) runStep(ctx context.Context, node Node, meta *NodeMeta) error {
	collector, err := r.resolveStepCollector(node, meta)
	if err != nil {
		return err
	}

	addr := nodeKey(node.Type, node.ID)
	result, err := r.executeStepOnce(ctx, node, meta, collector, r.scope.child(), addr)
	if err != nil {
		return err
	}

	r.raw[addr] = result
	if err := r.scope.recordResult(node, result); err != nil {
		return fmt.Errorf("failed to convert result for %s: %w", addr, err)
	}

	r.logger.Info("step resolved",
		zap.String("type", node.Type),
		zap.String("id", node.ID),
	)
	return nil
}

// executeStepOnce creates and resolves a step against the given eval context.
// It is the shared kernel of a plain step run and one iteration of a for_each
// collection; label distinguishes the two in error messages
// ("<type>/<id>" vs "<type>/<id>[<key>]").
func (r *Runner) executeStepOnce(
	ctx context.Context,
	node Node,
	meta *NodeMeta,
	collector engine.Collector,
	ectx *hcl.EvalContext,
	label string,
) (engine.Result, error) {
	step, diags := r.registry.CreateStep(node.Type, node.ID, collector, meta.Body, ectx)
	if diags.HasErrors() {
		return engine.Result{}, fmt.Errorf("failed to create step %s: %s", label, diags.Error())
	}

	result, err := step.Resolve(ctx)
	if err != nil {
		return engine.Result{}, fmt.Errorf("failed to resolve step %s: %w", label, err)
	}
	return result, nil
}

func (r *Runner) runCollection(ctx context.Context, node Node, meta *NodeMeta) error {
	if meta.ForEach == nil {
		return fmt.Errorf("collection node %s/%s has no for_each expression", node.Type, node.ID)
	}

	// Build the per-node context once; each iteration only layers `each` onto
	// it, so the step.*/collector.* namespaces are materialised a single time
	// per collection rather than once per element.
	base := r.scope.child()

	forVal, diags := meta.ForEach.Value(base)
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

	iterRaw := make(map[string]engine.Result)

	it := forVal.ElementIterator()
	for it.Next() {
		key, val := it.Element()
		// validateForEachValue restricts for_each to maps, objects, and
		// sets of strings, so key is always a cty.String.
		keyStr := key.AsString()
		label := fmt.Sprintf("%s/%s[%s]", node.Type, node.ID, keyStr)

		result, err := r.executeStepOnce(ctx, node, meta, collector, withEach(base, key, val), label)
		if err != nil {
			return err
		}
		iterRaw[keyStr] = result
	}

	r.raw[nodeKey(node.Type, node.ID)] = engine.Result{Data: iterRaw}
	if err := r.scope.recordCollection(node, iterRaw); err != nil {
		return fmt.Errorf("failed to convert results for %s/%s: %w", node.Type, node.ID, err)
	}

	r.logger.Info("collection resolved",
		zap.String("type", node.Type),
		zap.String("id", node.ID),
		zap.Int("iterations", len(iterRaw)),
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
