package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/urfave/cli/v3"
)

var loggerDeferFunc func() error

func main() {
	app := &cli.Command{
		Name:  "infracollect",
		Usage: "Infracollect is a tool to collect infrastructure data",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Usage:   "Enable debug logging",
			},
			&cli.StringFlag{
				Name:    "log-level",
				Aliases: []string{"l"},
				Value:   "info",
				Usage:   "Log Level (debug, info, warn, error, fatal)",
				Action: func(ctx context.Context, command *cli.Command, s string) error {
					_, err := zapcore.ParseLevel(s)
					if err != nil {
						return fmt.Errorf("invalid log level %s: %w", s, err)
					}
					return nil
				},
			},
		},
		Commands: []*cli.Command{
			collectCommand,
		},
		Before: func(ctx context.Context, command *cli.Command) (context.Context, error) {
			logger, _, err := createLogger(command.Bool("debug"), command.String("log-level"))
			if err != nil {
				return nil, err
			}

			logger.Info("logger created", zap.String("log_level", command.String("log-level")))

			loggerDeferFunc = func() error {
				return logger.Sync()
			}

			return withLogger(ctx, logger), nil
		},
		ExitErrHandler: func(ctx context.Context, command *cli.Command, err error) {
			if err == nil {
				return
			}

			if logger := tryLogger(ctx); logger != nil {
				logger.Fatal("failed to run application", zap.Error(err))
			} else {
				log.Fatal(fmt.Errorf("failed to run application: %w", err))
			}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	defer func() {
		if loggerDeferFunc != nil {
			loggerDeferFunc()
		}
	}()

	app.Run(ctx, os.Args)
}
