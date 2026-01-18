package engine

import "context"

type Named interface {
	Name() string
	Kind() string
}

type Closer interface {
	Close(context.Context) error
}
