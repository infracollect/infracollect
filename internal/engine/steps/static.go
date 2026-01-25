package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/infracollect/infracollect/internal/engine"
	"github.com/spf13/afero"
)

type StaticStepConfig struct {
	Filepath *string
	Value    *string
	ParseAs  *string
}

func NewStaticStep(name string, cfg StaticStepConfig) (engine.Step, error) {
	if cfg.Filepath != nil && cfg.Value != nil {
		return nil, fmt.Errorf("both filepath and value are set")
	}

	if cfg.Filepath == nil && cfg.Value == nil {
		return nil, fmt.Errorf("neither filepath nor value are set")
	}

	if cfg.Filepath != nil {
		rootDir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}

		fs := afero.NewBasePathFs(afero.NewOsFs(), rootDir)
		return newStaticFileStep(name, fs, cfg), nil
	} else if cfg.Value != nil {
		return newStaticValueStep(name, *cfg.Value, cfg.ParseAs), nil
	}

	return nil, fmt.Errorf("invalid static step configuration")
}

func newStaticFileStep(name string, fs afero.Fs, cfg StaticStepConfig) engine.Step {
	return engine.StepFunction(name, "static", func(ctx context.Context) (engine.Result, error) {
		data, err := afero.ReadFile(fs, *cfg.Filepath)
		if err != nil {
			return engine.Result{}, fmt.Errorf("failed to read filepath %s: %w", *cfg.Filepath, err)
		}

		hasJSONExtension := strings.HasSuffix(*cfg.Filepath, ".json")
		shouldParseAsJSON := hasJSONExtension && (cfg.ParseAs == nil || *cfg.ParseAs == "json")
		if shouldParseAsJSON {
			var parsed any
			if err := json.Unmarshal(data, &parsed); err != nil {
				return engine.Result{}, fmt.Errorf("failed to parse as json %s: %w", *cfg.Filepath, err)
			}
			return engine.Result{Data: parsed}, nil
		}

		return engine.Result{Data: map[string]any{filepath.Base(*cfg.Filepath): string(data)}}, nil
	})
}

func newStaticValueStep(name string, value string, parseAs *string) engine.Step {
	return engine.StepFunction(name, "static", func(ctx context.Context) (engine.Result, error) {
		if parseAs != nil && *parseAs == "json" {
			var parsed any
			if err := json.Unmarshal([]byte(value), &parsed); err != nil {
				return engine.Result{}, fmt.Errorf("failed to parse as json %s: %w", value, err)
			}
			return engine.Result{Data: parsed}, nil
		}
		// TODO: should we add a RawData field to the result?
		return engine.Result{Data: map[string]any{"value": value}}, nil
	})
}
