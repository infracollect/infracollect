package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStep_Resolve(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	collector, err := NewCollector(Config{
		BaseURL: server.URL,
	}, WithHttpClient(server.Client()))
	require.NoError(t, err)

	step, err := NewGetStep(collector.(*Collector), GetConfig{
		Path: "/test",
	})
	require.NoError(t, err)

	result, err := step.Resolve(t.Context())
	require.NoError(t, err)

	data, ok := result.Data.(map[string]any)
	require.True(t, ok, "expected map[string]any, got %T", result.Data)
	assert.Equal(t, "ok", data["status"])
}
