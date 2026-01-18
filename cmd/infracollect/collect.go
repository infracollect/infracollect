package main

import (
	"context"
	"fmt"
	"os"

	"github.com/adrien-f/infracollect/pkg/runner"
	"github.com/urfave/cli/v3"
)

var collectCommand = &cli.Command{
	Name:  "collect",
	Usage: "Collect infrastructure data",
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

		r, err := runner.New(logger.Named("runner"), job)
		if err != nil {
			return fmt.Errorf("failed to create runner: %w", err)
		}

		if err := r.Run(ctx); err != nil {
			return fmt.Errorf("failed to run job: %w", err)
		}

		return nil
	},
}
