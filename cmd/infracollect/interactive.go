package main

import (
	"context"
	"os"

	"golang.org/x/term"
)

type interactiveCtxKeyType struct{}

var interactiveCtxKey = interactiveCtxKeyType{}

func isInteractiveEnvironment() bool {
	if os.Getenv("CI") != "" {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func withInteractive(ctx context.Context, interactive bool) context.Context {
	return context.WithValue(ctx, interactiveCtxKey, interactive)
}

func isInteractive(ctx context.Context) bool {
	interactive, ok := ctx.Value(interactiveCtxKey).(bool)
	if !ok {
		return false
	}
	return interactive
}
