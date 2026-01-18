package engine

import (
	"context"
)

type Step interface {
	Named
	Resolve(ctx context.Context) (Result, error)
}

type StepFunc func(ctx context.Context) (Result, error)

type stepFunction struct {
	name string
	kind string
	fn   StepFunc
}

func (s *stepFunction) Name() string {
	return s.name
}

func (s *stepFunction) Kind() string {
	return s.kind
}

func (s *stepFunction) Resolve(ctx context.Context) (Result, error) {
	return s.fn(ctx)
}

func StepFunction(name string, kind string, fn StepFunc) Step {
	return &stepFunction{name: name, kind: kind, fn: fn}
}

type WrappingStepFunc func(ctx context.Context, step Step) (Result, error)

type wrappingStepFunction struct {
	fn    WrappingStepFunc
	inner Step
}

func (s *wrappingStepFunction) Name() string {
	return s.inner.Name()
}

func (s *wrappingStepFunction) Kind() string {
	return s.inner.Kind()
}

func (s *wrappingStepFunction) Resolve(ctx context.Context) (Result, error) {
	return s.fn(ctx, s.inner)
}

func WrappingStepFunction(name string, fn WrappingStepFunc) func(step Step) Step {
	return func(step Step) Step {
		return &wrappingStepFunction{fn: fn, inner: step}
	}
}
