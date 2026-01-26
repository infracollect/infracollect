package runner

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
	v1 "github.com/infracollect/infracollect/apis/v1"
	"github.com/infracollect/infracollect/internal/engine"
	"go.uber.org/zap"
)

type Runner struct {
	logger   *zap.Logger
	job      v1.CollectJob
	pipeline *engine.Pipeline
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

func New(ctx context.Context, logger *zap.Logger, job v1.CollectJob) (*Runner, error) {
	logger.Info("creating runner", zap.String("job_name", job.Metadata.Name))

	pipeline, err := createPipeline(ctx, logger.Named("pipeline"), job)
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
	for id, collector := range r.pipeline.Collectors() {
		if err := collector.Start(ctx); err != nil {
			return fmt.Errorf("failed to start collector '%s' (%s): %w", id, collector.Name(), err)
		}
	}

	defer func() {
		// Use a background context for cleanup to ensure we always attempt cleanup
		// even if the original context was cancelled
		cleanupCtx := context.Background()
		for id, collector := range r.pipeline.Collectors() {
			if err := collector.Close(cleanupCtx); err != nil {
				r.logger.Error("failed to close collector", zap.String("collector_id", id), zap.String("collector_name", collector.Name()), zap.Error(err))
			}
		}
	}()

	results, err := r.pipeline.Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to run pipeline: %w", err)
	}

	if err := r.WriteResults(ctx, results); err != nil {
		return fmt.Errorf("failed to write results: %w", err)
	}

	return nil
}

// WriteResults writes results to the sink, encoding each result and wrapping with name/data
// structure for stdout sinks.
func (r *Runner) WriteResults(ctx context.Context, results map[string]engine.Result) error {
	for stepID, result := range results {
		reader, err := r.encoder.EncodeResult(ctx, result)
		if err != nil {
			return fmt.Errorf("failed to encode result for step %s: %w", stepID, err)
		}

		filename := fmt.Sprintf("%s.%s", stepID, r.encoder.FileExtension())
		if err := r.sink.Write(ctx, filename, reader); err != nil {
			return fmt.Errorf("failed to write result for step %s: %w", stepID, err)
		}
	}

	// Close the sink if needed
	if err := r.sink.Close(ctx); err != nil {
		return fmt.Errorf("failed to close sink: %w", err)
	}

	return nil
}
