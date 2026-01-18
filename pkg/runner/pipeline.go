package runner

import (
	"fmt"
	"os"

	v1 "github.com/adrien-f/infracollect/apis/v1"
	"github.com/adrien-f/infracollect/pkg/collectors/terraform"
	"github.com/adrien-f/infracollect/pkg/engine"
	jsonencoder "github.com/adrien-f/infracollect/pkg/engine/encoders/json"
	"github.com/adrien-f/infracollect/pkg/engine/writers/bundle"
	"github.com/adrien-f/infracollect/pkg/engine/writers/folder"
	"github.com/adrien-f/infracollect/pkg/engine/writers/stream"
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
		} else {
			logger.Error("unknown collector type", zap.String("collector_id", collectorSpec.ID))
			return nil, fmt.Errorf("unknown collector type: %s", collectorSpec.ID)
		}
	}

	for _, stepSpec := range spec.Steps {
		if stepSpec.TerraformDataSource != nil {
			collector, ok := pipeline.GetCollector(stepSpec.TerraformDataSource.Collector)
			if !ok {
				return nil, fmt.Errorf("step %s has invalid collector reference: collector %s not found", stepSpec.ID, stepSpec.TerraformDataSource.Collector)
			}

			tfcollector, ok := collector.(*terraform.Collector)
			if !ok {
				return nil, fmt.Errorf("step %s has invalid collector reference: collector %s is not a terraform collector", stepSpec.ID, stepSpec.TerraformDataSource.Collector)
			}

			step, err := terraform.NewDataSourceStep(tfcollector, stepSpec.TerraformDataSource.Name, stepSpec.TerraformDataSource.Args)
			if err != nil {
				return nil, fmt.Errorf("failed to create terraform data source step: %w", err)
			}

			if err := pipeline.AddStep(stepSpec.ID, step); err != nil {
				return nil, fmt.Errorf("failed to add terraform data source step: %w", err)
			}

			logger.Info("created terraform data source step", zap.String("step_id", stepSpec.ID))
		} else {
			logger.Error("unknown step type", zap.String("step_id", stepSpec.ID))
			return nil, fmt.Errorf("unknown step type: %s", stepSpec.ID)
		}
	}

	// Build encoder and writer from output spec
	pipeline.SetEncoder(buildEncoder(spec.Output))
	pipeline.SetWriter(buildWriter(spec.Output))

	return pipeline, nil
}

// buildEncoder creates an encoder from the output spec.
// Defaults to compact JSON if no encoding is specified.
func buildEncoder(output *v1.OutputSpec) engine.Encoder {
	cfg := jsonencoder.Config{}

	if output != nil && output.Encoding != nil && output.Encoding.JSON != nil {
		cfg.Indent = output.Encoding.JSON.Indent
	}

	return jsonencoder.New(cfg)
}

// buildWriter creates a writer from the output spec.
// Defaults to stdout if no destination is specified.
func buildWriter(output *v1.OutputSpec) engine.Writer {
	if output == nil || output.Destination == nil {
		return stream.New(os.Stdout)
	}

	dest := output.Destination

	if dest.Folder != nil {
		return folder.New(dest.Folder.Path)
	}

	if dest.Zip != nil {
		return bundle.NewZipWriter(dest.Zip.Path)
	}

	// Default to stdout (including explicit stdout spec)
	return stream.New(os.Stdout)
}
