package engine

import (
	"context"
	"errors"
)

var ErrCollectorNotStarted = errors.New("collector not started")

type Collector interface {
	Named
	Closer
	Start(context.Context) error
}
