package engine

import (
	"context"
	"io"
)

// Sink is a destination for output
type Sink interface {
	Named
	Closer
	Write(ctx context.Context, path string, data io.Reader) error
}
