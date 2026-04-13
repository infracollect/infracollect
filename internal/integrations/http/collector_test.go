package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCollector_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "empty base_url",
			cfg:     Config{},
			wantErr: "base_url is required",
		},
		{
			name:    "invalid URL",
			cfg:     Config{BaseURL: "://bad"},
			wantErr: "failed to parse base_url",
		},
		{
			name:    "non-http scheme",
			cfg:     Config{BaseURL: "ftp://example.com"},
			wantErr: "base_url must use http or https",
		},
		{
			name: "valid http",
			cfg:  Config{BaseURL: "http://example.com"},
		},
		{
			name: "valid https",
			cfg:  Config{BaseURL: "https://example.com"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewCollector(tt.cfg)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, c)
		})
	}
}

func TestCollector_NameAndKind(t *testing.T) {
	c, err := NewCollector(Config{BaseURL: "https://api.example.com"})
	require.NoError(t, err)

	assert.Equal(t, "http(api.example.com)", c.Name())
	assert.Equal(t, "http", c.Kind())
}

func TestCollector_StartAndClose(t *testing.T) {
	c, err := NewCollector(Config{BaseURL: "https://example.com"})
	require.NoError(t, err)

	assert.NoError(t, c.Start(t.Context()))
	assert.NoError(t, c.Close(t.Context()))
}

// newCaptureServer returns an httptest server that captures the incoming
// request and a pointer to retrieve it after Do().
func newCaptureServer(t *testing.T) (*httptest.Server, **http.Request) {
	t.Helper()
	var captured *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server, &captured
}

// doGet creates a GET request and executes it through the collector.
func doGet(t *testing.T, collector *Collector, serverURL string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, serverURL+"/test", nil)
	require.NoError(t, err)
	_, err = collector.Do(req)
	require.NoError(t, err)
}

func TestCollector_DefaultHeaders(t *testing.T) {
	server, captured := newCaptureServer(t)

	c, err := NewCollector(Config{BaseURL: server.URL}, WithHttpClient(server.Client()))
	require.NoError(t, err)

	doGet(t, c.(*Collector), server.URL)

	assert.Equal(t, "infracollect/0.1.0", (*captured).Header.Get("User-Agent"))
	assert.Equal(t, "application/json", (*captured).Header.Get("Accept"))
}

func TestCollector_CustomHeaders(t *testing.T) {
	server, captured := newCaptureServer(t)

	c, err := NewCollector(Config{
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "value"},
	}, WithHttpClient(server.Client()))
	require.NoError(t, err)

	doGet(t, c.(*Collector), server.URL)

	assert.Equal(t, "value", (*captured).Header.Get("X-Custom"))
}

func TestCollector_RequestHeaderOverridesDefault(t *testing.T) {
	server, captured := newCaptureServer(t)

	c, err := NewCollector(Config{BaseURL: server.URL}, WithHttpClient(server.Client()))
	require.NoError(t, err)

	collector := c.(*Collector)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/test", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "custom-agent")

	_, err = collector.Do(req)
	require.NoError(t, err)

	assert.Equal(t, "custom-agent", (*captured).Header.Get("User-Agent"),
		"request-level header should take precedence over default")
}

func TestCollector_BasicAuth(t *testing.T) {
	tests := []struct {
		name       string
		auth       *AuthConfig
		wantHeader string
	}{
		{
			name: "username and password",
			auth: &AuthConfig{Basic: &BasicAuthConfig{
				Username: "user",
				Password: "pass",
			}},
			wantHeader: "Basic dXNlcjpwYXNz", // base64("user:pass")
		},
		{
			name: "pre-encoded",
			auth: &AuthConfig{Basic: &BasicAuthConfig{
				Encoded: "cHJlZW5jb2RlZA==",
			}},
			wantHeader: "Basic cHJlZW5jb2RlZA==",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, captured := newCaptureServer(t)

			c, err := NewCollector(Config{
				BaseURL: server.URL,
				Auth:    tt.auth,
			}, WithHttpClient(server.Client()))
			require.NoError(t, err)

			doGet(t, c.(*Collector), server.URL)

			assert.Equal(t, tt.wantHeader, (*captured).Header.Get("Authorization"))
		})
	}
}

func TestCollector_BaseURL(t *testing.T) {
	c, err := NewCollector(Config{BaseURL: "https://api.example.com/v1"})
	require.NoError(t, err)

	collector := c.(*Collector)
	assert.Equal(t, "api.example.com", collector.BaseURL().Host)
	assert.Equal(t, "/v1", collector.BaseURL().Path)
}
