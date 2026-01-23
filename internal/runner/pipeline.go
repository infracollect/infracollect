package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "github.com/adrien-f/infracollect/apis/v1"
	httpCollector "github.com/adrien-f/infracollect/internal/collectors/http"
	"github.com/adrien-f/infracollect/internal/collectors/terraform"
	"github.com/adrien-f/infracollect/internal/engine"
	"github.com/adrien-f/infracollect/internal/engine/encoders"
	"github.com/adrien-f/infracollect/internal/engine/sinks"
	tfclient "github.com/adrien-f/tf-data-client"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
)

func createPipeline(logger *zap.Logger, job v1.CollectJob) (*engine.Pipeline, error) {
	logger.Info("creating pipeline", zap.String("job_name", job.Metadata.Name))
	spec := job.Spec
	pipeline := engine.NewPipeline(job.Metadata.Name)

	tfClient, err := tfclient.New(tfclient.WithLogger(zapr.NewLogger(logger)))
	if err != nil {
		return nil, fmt.Errorf("failed to create terraform data client: %w", err)
	}

	for _, collectorSpec := range spec.Collectors {
		if collectorSpec.Terraform != nil {
			terraformCollector, err := terraform.NewCollector(tfClient, terraform.Config{
				Provider: collectorSpec.Terraform.Provider,
				Version:  collectorSpec.Terraform.Version,
				Args:     collectorSpec.Terraform.Args,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create terraform collector: %w", err)
			}

			if err := pipeline.AddCollector(collectorSpec.ID, terraformCollector); err != nil {
				return nil, fmt.Errorf("failed to add terraform collector: %w", err)
			}

			logger.Info("created terraform collector", zap.String("collector_id", collectorSpec.ID))
		} else if collectorSpec.HTTP != nil {
			httpCfg := httpCollector.Config{
				BaseURL: collectorSpec.HTTP.BaseURL,
				Headers: collectorSpec.HTTP.Headers,
			}
			if collectorSpec.HTTP.Timeout != nil {
				httpCfg.Timeout = time.Duration(*collectorSpec.HTTP.Timeout) * time.Second
			}
			if collectorSpec.HTTP.Auth != nil && collectorSpec.HTTP.Auth.Basic != nil {
				httpCfg.Auth = &httpCollector.AuthConfig{
					Basic: &httpCollector.BasicAuthConfig{
						Username: collectorSpec.HTTP.Auth.Basic.Username,
						Password: collectorSpec.HTTP.Auth.Basic.Password,
						Encoded:  collectorSpec.HTTP.Auth.Basic.Encoded,
					},
				}
			}

			collector, err := httpCollector.NewCollector(httpCfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create http collector: %w", err)
			}

			if err := pipeline.AddCollector(collectorSpec.ID, collector); err != nil {
				return nil, fmt.Errorf("failed to add http collector: %w", err)
			}

			logger.Info("created http collector", zap.String("collector_id", collectorSpec.ID))
		} else {
			logger.Error("unknown collector type", zap.String("collector_id", collectorSpec.ID))
			return nil, fmt.Errorf("unknown collector type: %s", collectorSpec.ID)
		}
	}

	for _, stepSpec := range spec.Steps {
		if stepSpec.TerraformDataSource != nil {
			collector, ok := pipeline.GetCollector(stepSpec.Collector)
			if !ok {
				return nil, fmt.Errorf("step %s has invalid collector reference: collector %s not found", stepSpec.ID, stepSpec.Collector)
			}

			tfcollector, ok := collector.(*terraform.Collector)
			if !ok {
				return nil, fmt.Errorf("step %s has invalid collector reference: collector %s is not a terraform collector", stepSpec.ID, stepSpec.Collector)
			}

			step := terraform.NewDataSourceStep(tfcollector, stepSpec.TerraformDataSource.Name, stepSpec.TerraformDataSource.Args)
			if err := pipeline.AddStep(stepSpec.ID, step); err != nil {
				return nil, fmt.Errorf("failed to add terraform data source step: %w", err)
			}

			logger.Info("created terraform data source step", zap.String("step_id", stepSpec.ID))
		} else if stepSpec.HTTPGet != nil {
			collector, ok := pipeline.GetCollector(stepSpec.Collector)
			if !ok {
				return nil, fmt.Errorf("step %s has invalid collector reference: collector %s not found", stepSpec.ID, stepSpec.Collector)
			}

			httpColl, ok := collector.(*httpCollector.Collector)
			if !ok {
				return nil, fmt.Errorf("step %s has invalid collector reference: collector %s is not an http collector", stepSpec.ID, stepSpec.Collector)
			}

			step, err := httpCollector.NewGetStep(httpColl, httpCollector.GetConfig{
				Path:         stepSpec.HTTPGet.Path,
				Headers:      stepSpec.HTTPGet.Headers,
				Params:       stepSpec.HTTPGet.Params,
				ResponseType: stepSpec.HTTPGet.ResponseType,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create http get step: %w", err)
			}

			if err := pipeline.AddStep(stepSpec.ID, step); err != nil {
				return nil, fmt.Errorf("failed to add http get step: %w", err)
			}

			logger.Info("created http get step", zap.String("step_id", stepSpec.ID))
		} else {
			logger.Error("unknown step type", zap.String("step_id", stepSpec.ID))
			return nil, fmt.Errorf("unknown step type: %s", stepSpec.ID)
		}
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
func buildSink(ctx context.Context, pipeline *engine.Pipeline, job v1.CollectJob) (engine.Sink, error) {
	// No output spec = stdout
	if job.Spec.Output == nil {
		return sinks.NewStreamSink(os.Stdout), nil
	}

	sinkSpec := job.Spec.Output.Sink

	// No sink specified = stdout
	if sinkSpec == nil {
		return sinks.NewStreamSink(os.Stdout), nil
	}

	// Explicit sink configuration
	if sinkSpec.Stdout != nil {
		return sinks.NewStreamSink(os.Stdout), nil
	}
	if sinkSpec.Filesystem != nil {
		return buildFilesystemSink(pipeline, job)
	}
	if sinkSpec.S3 != nil {
		return buildS3Sink(ctx, pipeline, job)
	}

	return nil, fmt.Errorf("invalid sink configuration: no sink type specified")
}

func buildFilesystemSink(pipeline *engine.Pipeline, job v1.CollectJob) (engine.Sink, error) {
	var path string
	var prefix string

	// Extract path and prefix from spec if available
	if job.Spec.Output != nil && job.Spec.Output.Sink != nil && job.Spec.Output.Sink.Filesystem != nil {
		fs := job.Spec.Output.Sink.Filesystem
		if fs.Path != nil {
			path = *fs.Path
		}
		if fs.Prefix != nil {
			prefix = *fs.Prefix
		}
	}

	// Default path to current working directory
	if path == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		path = wd
	}

	// Expand prefix variables
	prefix = strings.ReplaceAll(prefix, "$JOB_NAME", job.Metadata.Name)
	prefix = strings.ReplaceAll(prefix, "$JOB_DATE_ISO8601", pipeline.Date().Format(engine.ISO8601Basic))
	prefix = strings.ReplaceAll(prefix, "$JOB_DATE_RFC3339", pipeline.Date().Format(time.RFC3339))

	return sinks.NewFilesystemSinkFromPath(filepath.Join(path, prefix))
}

func buildS3Sink(ctx context.Context, pipeline *engine.Pipeline, job v1.CollectJob) (engine.Sink, error) {
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

	// Expand prefix variables
	cfg.Prefix = strings.ReplaceAll(cfg.Prefix, "$JOB_NAME", job.Metadata.Name)
	cfg.Prefix = strings.ReplaceAll(cfg.Prefix, "$JOB_DATE_ISO8601", pipeline.Date().Format(engine.ISO8601Basic))
	cfg.Prefix = strings.ReplaceAll(cfg.Prefix, "$JOB_DATE_RFC3339", pipeline.Date().Format(time.RFC3339))

	return sinks.NewS3Sink(ctx, cfg)
}
