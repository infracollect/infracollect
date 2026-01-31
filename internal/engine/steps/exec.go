package steps

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/infracollect/infracollect/internal/engine"
	"go.uber.org/zap"
)

const (
	ExecStepKind = "exec"

	defaultTimeout = 30 * time.Second
	defaultFormat  = "json"
)

type ExecStepConfig struct {
	Program    []string
	Input      map[string]any
	WorkingDir *string
	Timeout    *string
	Format     *string
	Env        map[string]string
}

func NewExecStep(name string, logger *zap.Logger, cfg ExecStepConfig) (engine.Step, error) {
	if len(cfg.Program) == 0 {
		return nil, fmt.Errorf("program is required")
	}

	timeout := defaultTimeout
	if cfg.Timeout != nil {
		parsed, err := time.ParseDuration(*cfg.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %q: %w", *cfg.Timeout, err)
		}
		timeout = parsed
	}

	format := defaultFormat
	if cfg.Format != nil {
		format = *cfg.Format
	}

	var workingDir string
	if cfg.WorkingDir != nil {
		if filepath.IsAbs(*cfg.WorkingDir) {
			workingDir = *cfg.WorkingDir
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to get working directory: %w", err)
			}
			workingDir = filepath.Join(cwd, *cfg.WorkingDir)
		}
	}

	return engine.StepFunction(name, ExecStepKind, func(ctx context.Context) (engine.Result, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, cfg.Program[0], cfg.Program[1:]...)

		if workingDir != "" {
			cmd.Dir = workingDir
		}

		cmd.Env = os.Environ()
		for k, v := range cfg.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		if cfg.Input != nil {
			inputJSON, err := json.Marshal(cfg.Input)
			if err != nil {
				return engine.Result{}, fmt.Errorf("failed to marshal input: %w", err)
			}
			cmd.Stdin = bytes.NewReader(inputJSON)
		}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		logger.Debug("invoking exec step",
			zap.String("step", name),
			zap.Strings("program", cfg.Program),
			zap.Duration("timeout", timeout),
			zap.String("working_dir", cmd.Dir),
		)
		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		logger.Debug("exec step finished",
			zap.String("step", name),
			zap.Int("exit_code", exitCode),
			zap.Duration("duration", duration),
		)

		if err != nil {
			stderrStr := strings.TrimSpace(stderr.String())
			if ctx.Err() == context.DeadlineExceeded {
				return engine.Result{}, fmt.Errorf("command timed out after %s: %s", timeout, stderrStr)
			}
			if stderrStr != "" {
				return engine.Result{}, fmt.Errorf("command failed: %w: %s", err, stderrStr)
			}
			return engine.Result{}, fmt.Errorf("command failed: %w", err)
		}

		meta := map[string]string{
			"exec_program": strings.Join(cfg.Program, " "),
			"exec_format":  format,
		}

		if format == "json" {
			var parsed any
			if err := json.NewDecoder(&stdout).Decode(&parsed); err != nil {
				return engine.Result{}, fmt.Errorf("failed to parse output as JSON: %w", err)
			}
			return engine.Result{Data: parsed, Meta: meta}, nil
		}

		var encodedBuf bytes.Buffer
		enc := base64.NewEncoder(base64.StdEncoding, &encodedBuf)
		if _, err := io.Copy(enc, &stdout); err != nil {
			return engine.Result{}, fmt.Errorf("failed to encode output: %w", err)
		}
		if err := enc.Close(); err != nil {
			return engine.Result{}, fmt.Errorf("failed to flush base64 encoder: %w", err)
		}

		return engine.Result{Data: map[string]any{"output": encodedBuf.String()}, Meta: meta}, nil
	}), nil
}
