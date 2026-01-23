package engine

import "context"

type Named interface {
	Name() string
	Kind() string
}

type Closer interface {
	Close(context.Context) error
}

const (
	// ISO8601Basic is a URL-safe timestamp format without colons.
	// This is the recommended format for S3 keys and filesystem paths.
	ISO8601Basic = "20060102T150405Z"
)
