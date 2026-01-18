package runner

import (
	"context"
	"errors"
	"fmt"

	v1 "github.com/adrien-f/infracollect/apis/v1"
	"github.com/adrien-f/infracollect/pkg/engine"
	"github.com/goccy/go-yaml"
	"github.com/kaptinlin/jsonschema"
	"go.uber.org/zap"
)

type Runner struct {
	logger   *zap.Logger
	job      v1.CollectJob
	pipeline *engine.Pipeline
}

// ParseCollectJob parses a YAML or JSON job file and validates it against the JSON Schema
// generated from the v1.CollectJob struct. It returns a validated CollectJob struct or an error
// if parsing or validation fails.
func ParseCollectJob(data []byte) (v1.CollectJob, error) {
	// Generate JSON Schema from CollectJob struct
	schema, err := jsonschema.FromStruct[v1.CollectJob]()
	if err != nil {
		return v1.CollectJob{}, fmt.Errorf("failed to generate JSON Schema from CollectJob struct: %w", err)
	}

	// Parse YAML into a map for validation
	var jobData map[string]any
	if err := yaml.Unmarshal(data, &jobData); err != nil {
		return v1.CollectJob{}, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate kind is "CollectJob"
	if jobData["kind"] != "CollectJob" {
		return v1.CollectJob{}, fmt.Errorf("invalid kind: expected 'CollectJob', got '%s'", jobData["kind"])
	}

	// Validate the parsed data against the schema
	result := schema.ValidateMap(jobData)
	if !result.IsValid() {
		if len(result.Errors) == 0 {
			return v1.CollectJob{}, fmt.Errorf("job validation failed: unknown validation error")
		}

		var errs []error
		for field, err := range result.Errors {
			errs = append(errs, fmt.Errorf("field %s: %s", field, err.Message))
		}
		return v1.CollectJob{}, fmt.Errorf("job validation failed: %w", errors.Join(errs...))
	}

	// Unmarshal validated data into CollectJob struct
	var job v1.CollectJob
	if err := yaml.Unmarshal(data, &job); err != nil {
		return v1.CollectJob{}, fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	return job, nil
}

func New(logger *zap.Logger, job v1.CollectJob) (*Runner, error) {
	logger.Info("creating runner", zap.String("job_name", job.Metadata.Name))
	pipeline, err := createPipeline(logger.Named("pipeline"), job)
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline: %w", err)
	}

	return &Runner{
		logger:   logger,
		pipeline: pipeline,
		job:      job,
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
		if err := r.pipeline.Writer().Close(cleanupCtx); err != nil {
			r.logger.Error("failed to close writer", zap.Error(err))
		}
	}()

	if err := r.pipeline.Run(ctx); err != nil {
		return fmt.Errorf("failed to run pipeline: %w", err)
	}

	return nil
}
