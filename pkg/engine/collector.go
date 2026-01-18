package engine

import "context"

type Collector interface {
	Named
	Closer
	Start(context.Context) error
}
