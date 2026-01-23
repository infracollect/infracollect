package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type getStepTest struct {
	name               string
	config             GetConfig
	response           string
	responseStatusCode int    // defaults to 200
	contentType        string // defaults to "application/json"
	expected           any    // if set, asserts result equals this
	expectErr          string // if set, asserts error contains this
	validateReq        func(t *testing.T, req *http.Request)
}

func runGetStepTests(t *testing.T, tests []getStepTest) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults
			statusCode := tt.responseStatusCode
			if statusCode == 0 {
				statusCode = http.StatusOK
			}
			contentType := tt.contentType
			if contentType == "" {
				contentType = "application/json"
			}

			var capturedReq *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedReq = r
				w.Header().Set("Content-Type", contentType)
				w.WriteHeader(statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			collector, err := NewCollector(Config{
				BaseURL: server.URL,
			}, WithHttpClient(server.Client()))
			require.NoError(t, err)

			step, err := NewGetStep(collector.(*Collector), tt.config)
			require.NoError(t, err)

			result, err := step.Resolve(t.Context())

			if tt.validateReq != nil {
				tt.validateReq(t, capturedReq)
			}

			if tt.expectErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectErr)
				return
			}

			require.NoError(t, err)
			if tt.expected != nil {
				assert.Equal(t, tt.expected, result.Data)
			}
		})
	}
}

func TestGetStep_Resolve(t *testing.T) {
	t.Run("path handling", func(t *testing.T) {
		runGetStepTests(t, []getStepTest{
			{
				name:     "simple path",
				config:   GetConfig{Path: "/test"},
				response: `{"status": "ok"}`,
				expected: map[string]any{"status": "ok"},
			},
			{
				name:     "nested path segments",
				config:   GetConfig{Path: "/api/v1/users"},
				response: `{"users": []}`,
				expected: map[string]any{"users": []any{}},
			},
			{
				name:     "path without leading slash",
				config:   GetConfig{Path: "test"},
				response: `{"ok": true}`,
				validateReq: func(t *testing.T, req *http.Request) {
					assert.Equal(t, "/test", req.URL.Path)
				},
			},
		})
	})

	t.Run("request building", func(t *testing.T) {
		runGetStepTests(t, []getStepTest{
			{
				name: "custom headers",
				config: GetConfig{
					Path: "/test",
					Headers: map[string]string{
						"X-Custom-Header": "custom-value",
						"Authorization":   "Bearer token123",
					},
				},
				response: `{"ok": true}`,
				validateReq: func(t *testing.T, req *http.Request) {
					assert.Equal(t, "custom-value", req.Header.Get("X-Custom-Header"))
					assert.Equal(t, "Bearer token123", req.Header.Get("Authorization"))
				},
			},
			{
				name: "query params",
				config: GetConfig{
					Path: "/test",
					Params: map[string]string{
						"page":  "1",
						"limit": "10",
						"sort":  "name",
					},
				},
				response: `{"ok": true}`,
				validateReq: func(t *testing.T, req *http.Request) {
					assert.Equal(t, "1", req.URL.Query().Get("page"))
					assert.Equal(t, "10", req.URL.Query().Get("limit"))
					assert.Equal(t, "name", req.URL.Query().Get("sort"))
				},
			},
			{
				name: "headers and params combined",
				config: GetConfig{
					Path:    "/api/search",
					Headers: map[string]string{"X-API-Key": "secret-key"},
					Params:  map[string]string{"q": "test", "type": "user"},
				},
				response: `{"results": []}`,
				validateReq: func(t *testing.T, req *http.Request) {
					assert.Equal(t, "secret-key", req.Header.Get("X-API-Key"))
					assert.Equal(t, "test", req.URL.Query().Get("q"))
					assert.Equal(t, "user", req.URL.Query().Get("type"))
				},
			},
		})
	})

	t.Run("response types", func(t *testing.T) {
		runGetStepTests(t, []getStepTest{
			{
				name:     "json (default)",
				config:   GetConfig{Path: "/test"},
				response: `{"name": "test", "value": 42}`,
				expected: map[string]any{"name": "test", "value": float64(42)},
			},
			{
				name:        "raw",
				config:      GetConfig{Path: "/test", ResponseType: "raw"},
				response:    "raw response content",
				contentType: "text/plain",
				expected:    "raw response content",
			},
			{
				name:        "empty body",
				config:      GetConfig{Path: "/test", ResponseType: "raw"},
				response:    "",
				contentType: "text/plain",
				expected:    "",
			},
			{
				name:      "invalid json",
				config:    GetConfig{Path: "/test", ResponseType: "json"},
				response:  "not valid json",
				expectErr: "failed to parse JSON",
			},
		})
	})

	t.Run("error handling", func(t *testing.T) {
		runGetStepTests(t, []getStepTest{
			{
				name:               "500 internal server error",
				config:             GetConfig{Path: "/test"},
				response:           "Internal Server Error",
				responseStatusCode: http.StatusInternalServerError,
				expectErr:          "500",
			},
			{
				name:               "404 not found",
				config:             GetConfig{Path: "/nonexistent"},
				response:           "Not Found",
				responseStatusCode: http.StatusNotFound,
				expectErr:          "404",
			},
			{
				name:               "401 unauthorized",
				config:             GetConfig{Path: "/protected"},
				response:           "Unauthorized",
				responseStatusCode: http.StatusUnauthorized,
				expectErr:          "401",
			},
		})
	})
}
