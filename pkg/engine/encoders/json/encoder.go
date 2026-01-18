// Package json provides a JSON encoder for encoding results.
package json

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/adrien-f/infracollect/pkg/engine"
)

// Config holds configuration options for the JSON encoder.
type Config struct {
	// Indent specifies the indentation string. Empty = compact, "  " = 2 spaces, "\t" = tabs.
	Indent string
}

// Encoder implements engine.Encoder for JSON format.
type Encoder struct {
	indent string
}

// New creates a new JSON encoder with the given configuration.
func New(cfg Config) *Encoder {
	return &Encoder{
		indent: cfg.Indent,
	}
}

// Encode writes all results to w as a JSON object.
func (e *Encoder) Encode(ctx context.Context, w io.Writer, results map[string]engine.Result) error {
	// Extract Data from each result for cleaner output
	output := make(map[string]any, len(results))
	for k, v := range results {
		output[k] = v.Data
	}

	encoder := json.NewEncoder(w)
	if e.indent != "" {
		encoder.SetIndent("", e.indent)
	}

	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode results as JSON: %w", err)
	}

	return nil
}

// EncodeResult writes a single result's Data to w as JSON.
func (e *Encoder) EncodeResult(ctx context.Context, w io.Writer, result engine.Result) error {
	encoder := json.NewEncoder(w)
	if e.indent != "" {
		encoder.SetIndent("", e.indent)
	}

	if err := encoder.Encode(result.Data); err != nil {
		return fmt.Errorf("failed to encode result as JSON: %w", err)
	}

	return nil
}

// FileExtension returns "json".
func (e *Encoder) FileExtension() string {
	return "json"
}
