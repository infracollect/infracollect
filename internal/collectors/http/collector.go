package http

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/samber/lo"
)

const (
	CollectorKind  = "http"
	DefaultTimeout = 30 * time.Second
)

var (
	defaultHeaders = map[string]string{
		"User-Agent":      "infracollect/0.1.0",
		"Accept":          "application/json",
		"Accept-Encoding": "gzip",
	}
)

type Config struct {
	BaseURL  string
	Headers  map[string]string
	Auth     *AuthConfig
	Timeout  time.Duration
	Insecure bool
}

type AuthConfig struct {
	Basic *BasicAuthConfig
}

type BasicAuthConfig struct {
	Username string
	Password string
	Encoded  string
}

type Collector struct {
	baseURL    *url.URL
	httpClient *http.Client
	headers    map[string]string
}

type CollectOption func(*Collector)

func WithHttpClient(httpClient *http.Client) CollectOption {
	return func(c *Collector) {
		c.httpClient = httpClient
	}
}

func NewCollector(cfg Config, opts ...CollectOption) (engine.Collector, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("base_url is required")
	}

	parsedURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base_url '%s': %w", cfg.BaseURL, err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("base_url must use http or https scheme, got: %s", parsedURL.Scheme)
	}

	headers := lo.Assign(defaultHeaders, cfg.Headers)
	if cfg.Auth != nil && cfg.Auth.Basic != nil {
		if cfg.Auth.Basic.Encoded != "" {
			headers["Authorization"] = "Basic " + cfg.Auth.Basic.Encoded
		} else {
			headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(cfg.Auth.Basic.Username+":"+cfg.Auth.Basic.Password))
		}
	}

	collector := &Collector{
		baseURL: parsedURL,
		headers: headers,
	}

	for _, opt := range opts {
		opt(collector)
	}

	if collector.httpClient == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = DefaultTimeout
		}

		transport := cleanhttp.DefaultPooledTransport()
		if cfg.Insecure {
			if transport.TLSClientConfig == nil {
				transport.TLSClientConfig = &tls.Config{}
			}

			transport.TLSClientConfig.InsecureSkipVerify = true
		}

		collector.httpClient = &http.Client{
			Transport: transport,
			Timeout:   timeout,
		}
	}

	return collector, nil
}

func (c *Collector) Name() string {
	return fmt.Sprintf("%s(%s)", CollectorKind, c.baseURL.Host)
}

func (c *Collector) Kind() string {
	return CollectorKind
}

func (c *Collector) Start(ctx context.Context) error {
	return nil
}

func (c *Collector) Close(ctx context.Context) error {
	return nil
}

func (c *Collector) Do(req *http.Request) (*http.Response, error) {
	for k, v := range c.headers {
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}

	return c.httpClient.Do(req)
}

func (c *Collector) BaseURL() *url.URL {
	return c.baseURL
}
