package runner

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
	v1 "github.com/infracollect/infracollect/apis/v1"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/infracollect/infracollect/internal/engine/steps"
	"github.com/infracollect/infracollect/internal/integrations/http"
	"github.com/infracollect/infracollect/internal/integrations/terraform"
	"go.uber.org/zap"
)

type Runner struct {
	logger   *zap.Logger
	job      v1.CollectJob
	pipeline *Pipeline
	encoder  engine.Encoder
	sink     engine.Sink
}

var (
	defaultValidator = validator.New(validator.WithRequiredStructEnabled())
)

// ParseCollectJob parses a YAML or JSON job file and validates it against the JSON Schema
// generated from the v1.CollectJob struct. It returns a validated CollectJob struct or an error
// if parsing or validation fails.
func ParseCollectJob(data []byte) (v1.CollectJob, error) {
	var job v1.CollectJob
	if err := yaml.Unmarshal(data, &job); err != nil {
		return v1.CollectJob{}, fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	if err := defaultValidator.Struct(job); err != nil {
		return v1.CollectJob{}, fmt.Errorf("failed to validate job: %w", err)
	}

	return job, nil
}

func BuildRegistry(logger *zap.Logger) *engine.Registry {
	registry := engine.NewRegistry(logger)
	terraform.Register(registry)
	http.Register(registry)
	steps.Register(registry)
	return registry
}

func New(ctx context.Context, logger *zap.Logger, job v1.CollectJob, allowedEnv []string) (*Runner, error) {
	logger.Info("creating runner", zap.String("job_name", job.Metadata.Name))

	registry := BuildRegistry(logger)
	registry.RegisterDependency(engine.AllowedEnvVarsDepKey, allowedEnv)

	pipeline, err := BuildPipeline(ctx, logger.Named("pipeline"), registry, job)
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline: %w", err)
	}

	encoder, err := buildEncoder(job.Spec.Output)
	if err != nil {
		return nil, fmt.Errorf("failed to build encoder: %w", err)
	}

	sink, err := buildSink(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("failed to build sink: %w", err)
	}

	return &Runner{
		logger:   logger,
		pipeline: pipeline,
		job:      job,
		encoder:  encoder,
		sink:     sink,
	}, nil
}

func (r *Runner) Run(ctx context.Context) error {
	nodes, err := r.pipeline.dag.TopologicalSort()
	if err != nil {
		return fmt.Errorf("could not build graph: %w", err)
	}

	var results []engine.Result

	for _, node := range nodes {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		switch node.Kind {
		case NodeTypeCollector:
			collector, ok := r.pipeline.collectors[node.ID]
			if !ok {
				return fmt.Errorf("collector node %s not registered", node.ID)
			}

			if err := collector.Start(ctx); err != nil {
				return fmt.Errorf("failed to start collector '%s' (%s): %w", node.ID, collector.Name(), err)
			}
		case NodeTypeStep:
			step, ok := r.pipeline.steps[node.ID]
			if !ok {
				return fmt.Errorf("step node %s not registered", node.ID)
			}

			result, err := step.Resolve(ctx)
			if err != nil {
				return fmt.Errorf("failed to resolve step '%s': %w", node.ID, err)
			}

			result.ID = node.ID
			results = append(results, result)
		}
	}

	if err := r.WriteResults(ctx, results); err != nil {
		return fmt.Errorf("failed to write results: %w", err)
	}

	return nil
}

// WriteResults writes results to the sink in order, encoding each result and wrapping with
// name/data structure for stdout sinks.
func (r *Runner) WriteResults(ctx context.Context, results []engine.Result) error {
	for _, result := range results {
		reader, err := r.encoder.EncodeResult(ctx, result)
		if err != nil {
			return fmt.Errorf("failed to encode result for step %s: %w", result.ID, err)
		}

		filename := fmt.Sprintf("%s.%s", result.ID, r.encoder.FileExtension())
		if err := r.sink.Write(ctx, filename, reader); err != nil {
			return fmt.Errorf("failed to write result for step %s: %w", result.ID, err)
		}

		if len(result.Meta) > 0 {
			metaReader, err := r.encoder.EncodeMeta(ctx, result.Meta)
			if err != nil {
				return fmt.Errorf("failed to encode meta for step %s: %w", result.ID, err)
			}

			metaFilename := fmt.Sprintf("%s.meta.%s", result.ID, r.encoder.FileExtension())
			if err := r.sink.Write(ctx, metaFilename, metaReader); err != nil {
				return fmt.Errorf("failed to write meta for step %s: %w", result.ID, err)
			}
		}
	}

	// Close the sink if needed
	if err := r.sink.Close(ctx); err != nil {
		return fmt.Errorf("failed to close sink: %w", err)
	}

	return nil
}
