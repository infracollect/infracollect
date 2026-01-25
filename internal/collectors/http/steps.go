package http

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/infracollect/infracollect/internal/engine"
)

const (
	GetStepKind = "http_get"
)

type GetConfig struct {
	Path         string
	Headers      map[string]string
	Params       map[string]string
	ResponseType string
}

type getStep struct {
	collector *Collector
	config    GetConfig
}

func NewGetStep(collector *Collector, cfg GetConfig) (engine.Step, error) {
	return &getStep{
		collector: collector,
		config:    cfg,
	}, nil
}

func (s *getStep) Name() string {
	return fmt.Sprintf("%s(%s)", GetStepKind, s.config.Path)
}

func (s *getStep) Kind() string {
	return GetStepKind
}

func (s *getStep) Resolve(ctx context.Context) (engine.Result, error) {
	reqURL, err := s.buildURL()
	if err != nil {
		return engine.Result{}, fmt.Errorf("failed to build request URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return engine.Result{}, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range s.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := s.collector.Do(req)
	if err != nil {
		return engine.Result{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return engine.Result{}, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	data, err := s.processResponse(resp.Header.Get("Content-Encoding"), resp.Body)
	if err != nil {
		return engine.Result{}, fmt.Errorf("failed to process response: %w", err)
	}

	return engine.Result{Data: data}, nil
}

func (s *getStep) buildURL() (*url.URL, error) {
	base := s.collector.BaseURL()

	pathURL, err := url.Parse(s.config.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path '%s': %w", s.config.Path, err)
	}

	fullURL := base.ResolveReference(pathURL)

	if len(s.config.Params) > 0 {
		query := fullURL.Query()
		for k, v := range s.config.Params {
			query.Set(k, v)
		}
		fullURL.RawQuery = query.Encode()
	}

	return fullURL, nil
}

func (s *getStep) processResponse(contentEncoding string, body io.ReadCloser) (any, error) {
	responseType := s.config.ResponseType
	if responseType == "" {
		responseType = "json"
	}

	if contentEncoding == "gzip" {
		gzipReader, err := gzip.NewReader(body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer func() { _ = gzipReader.Close() }()
		body = gzipReader
	}

	switch responseType {
	case "json":
		var data any
		if err := json.NewDecoder(body).Decode(&data); err != nil {
			return nil, fmt.Errorf("failed to parse JSON response: %w", err)
		}
		return data, nil
	case "raw":
		raw, err := io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return string(raw), nil
	default:
		return nil, fmt.Errorf("unknown response_type: %s", responseType)
	}
}
