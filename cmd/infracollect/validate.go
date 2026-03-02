package main

import (
	"context"
	"fmt"
	"os"

	"github.com/infracollect/infracollect/internal/runner"
	"github.com/urfave/cli/v3"
	"go.uber.org/zap"
)

var validateCommand = &cli.Command{
	Name:  "validate",
	Usage: "Validate a job file",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "pass-env",
			Usage: "Environment variables to pass through to job execution (can be repeated)",
		},
	},
	Arguments: []cli.Argument{
		&cli.StringArg{
			Name:      "job",
			UsageText: "The job file to validate",
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		logger := getLogger(ctx)

		jobFilename := command.StringArg("job")
		if jobFilename == "" {
			return fmt.Errorf("no job file provided")
		}

		logger = logger.With(zap.String("job_filename", jobFilename))
		logger.Debug("validating job file")

		jobFile, _, err := readJobFile(ctx, jobFilename)
		if err != nil {
			return fmt.Errorf("failed to read job file '%s': %w", jobFilename, err)
		}

		tmpl, diags := runner.ParseJobTemplate(jobFile, jobFilename)
		if diags.HasErrors() {
			writeDiags(diags)
			return fmt.Errorf("job file '%s' is invalid", jobFilename)
		}

		allowedEnv := command.StringSlice("pass-env")
		registry, err := buildRegistry(logger.Named("registry"), allowedEnv)
		if err != nil {
			return fmt.Errorf("failed to build registry: %w", err)
		}
		if _, diags := runner.New(logger.Named("runner"), tmpl, registry, allowedEnv); diags.HasErrors() {
			writeDiags(diags)
			return fmt.Errorf("job file '%s' is invalid", jobFilename)
		}

		fmt.Fprintf(os.Stdout, "OK %s (job: %s)\n", jobFilename, tmpl.JobName())
		return nil
	},
}
