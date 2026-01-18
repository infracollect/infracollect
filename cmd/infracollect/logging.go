package main

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

var (
	loggerCtxKey = struct{}{}
)

func createLogger(debug bool, logLevel string) (logger *zap.Logger, level zap.AtomicLevel, err error) {
	level, err = zap.ParseAtomicLevel(logLevel)
	if err != nil {
		return nil, zap.NewAtomicLevel(), fmt.Errorf("invalid log level %s: %w", logLevel, err)
	}

	var loggerCfg zap.Config
	if debug {
		loggerCfg = zap.NewDevelopmentConfig()
		loggerCfg.Level = level
	} else {
		loggerCfg = zap.NewProductionConfig()
		loggerCfg.DisableStacktrace = false
		loggerCfg.Level = level
	}

	logger, err = loggerCfg.Build()
	if err != nil {
		return nil, zap.NewAtomicLevel(), fmt.Errorf("failed to build logger: %w", err)
	}

	logger = logger.Named("infracollect")

	return logger, level, nil
}

func withLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey, logger)
}

func tryLogger(ctx context.Context) *zap.Logger {
	logger, ok := ctx.Value(loggerCtxKey).(*zap.Logger)
	if !ok {
		return nil
	}
	return logger
}

func getLogger(ctx context.Context) *zap.Logger {
	logger, ok := ctx.Value(loggerCtxKey).(*zap.Logger)
	if !ok {
		panic("logger not found in context")
	}
	return logger
}
