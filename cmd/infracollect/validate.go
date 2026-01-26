package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/infracollect/infracollect/internal/runner"
	"github.com/urfave/cli/v3"
	"go.uber.org/zap"
)

var validateCommand = &cli.Command{
	Name:  "validate",
	Usage: "Validate a job file",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "allowed-env",
			Usage: "Environment variables allowed in job configuration (can be repeated)",
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

		jobFile, _, err := readJobFile(ctx, jobFilename)
		if err != nil {
			return fmt.Errorf("failed to read job file '%s': %w", jobFilename, err)
		}

		logger = logger.With(zap.String("job_filename", jobFilename))
		logger.Debug("validating job file")

		job, err := runner.ParseCollectJob(jobFile)
		if err != nil {
			fmt.Println(formatValidationError(err))
			return fmt.Errorf("job file '%s' is invalid", jobFilename)
		}

		allowedEnv := command.StringSlice("allowed-env")

		variables, err := runner.BuildVariables(job, allowedEnv)
		if err != nil {
			return fmt.Errorf("failed to build variables: %w", err)
		}

		if err := runner.ExpandTemplates(&job, variables); err != nil {
			return fmt.Errorf("failed to expand templates: %w", err)
		}

		fmt.Printf("✓ Job file '%s' is valid\n", jobFilename)
		return nil
	},
}

func formatValidationError(err error) error {
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("job file has %d validation error(s):", len(validationErrs)))
		for _, fe := range validationErrs {
			sb.WriteString(fmt.Sprintf("\n  • %s: failed '%s' validation", fe.Namespace(), fe.Tag()))
			if fe.Param() != "" {
				sb.WriteString(fmt.Sprintf(" (param: %s)", fe.Param()))
			}
		}
		return errors.New(sb.String())
	}
	return err
}
