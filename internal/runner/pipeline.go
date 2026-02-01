package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	v1 "github.com/infracollect/infracollect/apis/v1"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/infracollect/infracollect/internal/engine/archivers"
	"github.com/infracollect/infracollect/internal/engine/encoders"
	"github.com/infracollect/infracollect/internal/engine/sinks"
	"go.uber.org/zap"
)

func createPipeline(ctx context.Context, logger *zap.Logger, registry *engine.Registry, job v1.CollectJob) (*engine.Pipeline, error) {
	logger.Info("creating pipeline", zap.String("job_name", job.Metadata.Name))
	spec := job.Spec
	pipeline := engine.NewPipeline(job.Metadata.Name)

	for _, collectorSpec := range spec.Collectors {
		resolvedSpec, err := ResolveCollectorSpec(collectorSpec)
		if err != nil {
			return nil, err
		}

		collector, err := registry.CreateCollector(resolvedSpec.Kind, resolvedSpec.Spec)
		if err != nil {
			return nil, err
		}

		if err := pipeline.AddCollector(collectorSpec.ID, collector); err != nil {
			return nil, fmt.Errorf("failed to add collector: %w", err)
		}

		logger.Debug("created collector", zap.String("collector_id", collectorSpec.ID), zap.String("collector_kind", collector.Kind()), zap.String("collector_name", collector.Name()))
	}

	for _, stepSpec := range spec.Steps {
		resolvedSpec, err := ResolveStepSpec(stepSpec)
		if err != nil {
			return nil, err
		}

		var collector engine.Collector
		if stepSpec.Collector != nil {
			foundCollector, ok := pipeline.GetCollector(*stepSpec.Collector)
			if !ok {
				return nil, fmt.Errorf("collector %q not found for step %q", *stepSpec.Collector, stepSpec.ID)
			}
			collector = foundCollector
		}

		step, err := registry.CreateStep(resolvedSpec.Kind, stepSpec.ID, collector, resolvedSpec.Spec)
		if err != nil {
			return nil, err
		}

		if err := pipeline.AddStep(stepSpec.ID, step); err != nil {
			return nil, fmt.Errorf("failed to add step: %w", err)
		}

		logger.Debug("created step", zap.String("step_id", stepSpec.ID), zap.String("step_kind", step.Kind()), zap.String("step_name", step.Name()))
	}

	return pipeline, nil
}

// buildEncoder creates an encoder from the output spec.
// Defaults to compact JSON if no encoding is specified.
func buildEncoder(output *v1.OutputSpec) (engine.Encoder, error) {
	if output == nil || output.Encoding == nil {
		return encoders.NewJSONEncoder(""), nil
	}

	if output.Encoding.JSON != nil {
		return encoders.NewJSONEncoder(output.Encoding.JSON.Indent), nil
	}

	return nil, fmt.Errorf("unknown encoding type")
}

// buildSink creates a sink from the job spec.
//
// Default behavior:
//   - No output spec: stdout sink
//   - No sink specified: stdout sink
//   - Explicit stdout sink: stdout sink
//   - Explicit filesystem sink: filesystem sink
//
// If archive is configured, the inner sink is wrapped with an ArchiveSink.
func buildSink(ctx context.Context, job v1.CollectJob) (engine.Sink, error) {
	sink, err := buildInnerSink(ctx, job)
	if err != nil {
		return nil, err
	}

	if job.Spec.Output != nil && job.Spec.Output.Archive != nil {
		return wrapWithArchiveSink(job, sink)
	}

	return sink, nil
}

// buildInnerSink creates the underlying sink (stdout, filesystem, or S3).
func buildInnerSink(ctx context.Context, job v1.CollectJob) (engine.Sink, error) {
	if job.Spec.Output == nil || job.Spec.Output.Sink == nil || job.Spec.Output.Sink.Stdout != nil {
		if job.Spec.Output != nil && job.Spec.Output.Archive != nil {
			return nil, fmt.Errorf("stdout sink cannot be used with archive configuration")
		}
		return sinks.NewStreamSink(os.Stdout), nil
	}

	if job.Spec.Output.Sink.Filesystem != nil {
		return buildFilesystemSink(job)
	}

	if job.Spec.Output.Sink.S3 != nil {
		return buildS3Sink(ctx, job)
	}

	return nil, fmt.Errorf("invalid sink configuration: no sink type specified")
}

func wrapWithArchiveSink(job v1.CollectJob, inner engine.Sink) (engine.Sink, error) {
	archive := job.Spec.Output.Archive

	compression := archive.Compression
	if compression == "" {
		compression = "gzip"
	}

	archiver, err := archivers.NewTarArchiver(compression)
	if err != nil {
		return nil, fmt.Errorf("failed to create tar archiver: %w", err)
	}

	name := archive.Name
	if name == "" {
		name = job.Metadata.Name
	}

	return sinks.NewArchiveSink(inner, archiver, name), nil
}

func buildFilesystemSink(job v1.CollectJob) (engine.Sink, error) {
	var path string
	var prefix string

	if job.Spec.Output != nil && job.Spec.Output.Sink != nil && job.Spec.Output.Sink.Filesystem != nil {
		fs := job.Spec.Output.Sink.Filesystem
		if fs.Path != nil {
			path = *fs.Path
		}
		if fs.Prefix != nil {
			prefix = *fs.Prefix
		}
	}

	if path == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		path = wd
	}

	return sinks.NewFilesystemSinkFromPath(filepath.Join(path, prefix))
}

func buildS3Sink(ctx context.Context, job v1.CollectJob) (engine.Sink, error) {
	s3Spec := job.Spec.Output.Sink.S3

	cfg := sinks.S3Config{
		Bucket:         s3Spec.Bucket,
		ForcePathStyle: s3Spec.ForcePathStyle,
	}

	if s3Spec.Region != nil {
		cfg.Region = *s3Spec.Region
	}

	if s3Spec.Endpoint != nil {
		cfg.Endpoint = *s3Spec.Endpoint
	}

	if s3Spec.Prefix != nil {
		cfg.Prefix = *s3Spec.Prefix
	}

	if s3Spec.Credentials != nil {
		cfg.AccessKeyID = s3Spec.Credentials.AccessKeyID
		cfg.SecretAccessKey = s3Spec.Credentials.SecretAccessKey
	}

	return sinks.NewS3Sink(ctx, cfg)
}

// buildVariables creates the variables map for expansion.
// It includes built-in variables and reads allowed environment variables.
// If a variable is not set, an error is returned.
func BuildVariables(job v1.CollectJob, allowedEnv []string) (map[string]string, error) {
	date := time.Now().UTC()
	variables := map[string]string{
		"JOB_NAME":         job.Metadata.Name,
		"JOB_DATE_ISO8601": date.Format(engine.ISO8601Basic),
		"JOB_DATE_RFC3339": date.Format(time.RFC3339),
	}

	var errs error
	for _, envName := range allowedEnv {
		val, ok := os.LookupEnv(envName)
		if !ok {
			errs = errors.Join(errs, fmt.Errorf("environment variable %q is not set", envName))
			continue
		}
		variables[envName] = val
	}

	if errs != nil {
		return nil, errs
	}

	return variables, nil
}
