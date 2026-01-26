package main

import (
	"context"
	"fmt"
	"os"

	"github.com/infracollect/infracollect/internal/runner"
	"github.com/urfave/cli/v3"
)

var collectCommand = &cli.Command{
	Name:  "collect",
	Usage: "Collect infrastructure data",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "allowed-env",
			Usage: "Environment variables allowed in job configuration (can be repeated)",
		},
	},
	Arguments: []cli.Argument{
		&cli.StringArg{
			Name:      "job",
			UsageText: "The job file to collect data from",
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		logger := getLogger(ctx)

		jobFilename := command.StringArg("job")
		if jobFilename == "" {
			return fmt.Errorf("no job file provided")
		}

		jobFile, err := os.ReadFile(jobFilename)
		if err != nil {
			return fmt.Errorf("failed to read job file: %w", err)
		}

		job, err := runner.ParseCollectJob(jobFile)
		if err != nil {
			return fmt.Errorf("failed to parse job: %w", err)
		}

		allowedEnv := command.StringSlice("allowed-env")

		variables, err := runner.BuildVariables(job, allowedEnv)
		if err != nil {
			return fmt.Errorf("failed to build variables: %w", err)
		}

		if err := runner.ExpandTemplates(&job, variables); err != nil {
			return fmt.Errorf("failed to expand templates: %w", err)
		}

		r, err := runner.New(ctx, logger.Named("runner"), job)
		if err != nil {
			return fmt.Errorf("failed to create runner: %w", err)
		}

		if err := r.Run(ctx); err != nil {
			return fmt.Errorf("failed to run job: %w", err)
		}

		return nil
	},
}
