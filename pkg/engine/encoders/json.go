package encoders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/adrien-f/infracollect/pkg/engine"
)

// JSONEncoder encodes results as JSON.
type JSONEncoder struct {
	indent string
}

func NewJSONEncoder(indent string) engine.Encoder {
	return &JSONEncoder{
		indent: indent,
	}
}

func (e *JSONEncoder) EncodeResult(ctx context.Context, result engine.Result) (io.Reader, error) {
	var buff bytes.Buffer
	encoder := json.NewEncoder(&buff)
	if e.indent != "" {
		encoder.SetIndent("", e.indent)
	}

	if err := encoder.Encode(result.Data); err != nil {
		return nil, fmt.Errorf("failed to encode result as JSON: %w", err)
	}

	return &buff, nil
}

func (e *JSONEncoder) EncodeResults(ctx context.Context, results map[string]engine.Result) (io.Reader, error) {
	var buff bytes.Buffer
	encoder := json.NewEncoder(&buff)
	if e.indent != "" {
		encoder.SetIndent("", e.indent)
	}

	if err := encoder.Encode(results); err != nil {
		return nil, fmt.Errorf("failed to encode results as JSON: %w", err)
	}

	return &buff, nil
}

func (e *JSONEncoder) FileExtension() string {
	return "json"
}
