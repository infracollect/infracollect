package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/infracollect/infracollect/internal/runner"
	"github.com/urfave/cli/v3"
	"go.uber.org/zap"
)

var collectCommand = &cli.Command{
	Name:  "collect",
	Usage: "Collect infrastructure data",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "allowed-env",
			Usage: "Environment variables allowed in job configuration (can be repeated)",
		},
		&cli.BoolFlag{
			Name:  "trust-remote",
			Usage: "Trust remote job file",
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

		jobFile, isRemote, err := readJobFile(ctx, jobFilename)
		if err != nil {
			return fmt.Errorf("failed to read job file '%s': %w", jobFilename, err)
		}

		if isRemote && !command.Bool("trust-remote") {
			if !isInteractive(ctx) {
				return fmt.Errorf("remote job file requires --trust-remote flag in non-interactive mode")
			}

			logger.Warn("remote job file is not trusted", zap.String("job_filename", jobFilename))
			fmt.Println(string(jobFile))

			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Are you sure you want to trust this remote job file? (y/n): ")
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}
			if strings.TrimSpace(response) != "y" {
				return fmt.Errorf("remote job file is not trusted")
			}
		}

		logger = logger.With(zap.String("job_filename", jobFilename))
		logger.Info("parsing job file")

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

func readJobFile(ctx context.Context, jobFilename string) ([]byte, bool, error) {
	if strings.HasPrefix(jobFilename, "http://") || strings.HasPrefix(jobFilename, "https://") {
		parsedURL, err := url.Parse(jobFilename)
		if err != nil {
			return nil, false, fmt.Errorf("failed to parse URL '%s': %w", jobFilename, err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
		if err != nil {
			return nil, false, fmt.Errorf("failed to create request to remote job file '%s': %w", jobFilename, err)
		}

		resp, err := cleanhttp.DefaultClient().Do(req)
		if err != nil {
			return nil, false, fmt.Errorf("failed to execute request to remote job file '%s': %w", jobFilename, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, false, fmt.Errorf("request to remote job file '%s' failed with status %d", jobFilename, resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, false, fmt.Errorf("failed to read response body from remote job file '%s': %w", jobFilename, err)
		}

		return body, true, nil
	}

	jobFile, err := os.ReadFile(jobFilename)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read local job file '%s': %w", jobFilename, err)
	}

	return jobFile, false, nil
}
