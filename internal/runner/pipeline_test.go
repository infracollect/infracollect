package runner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/infracollect/infracollect/apis/v1"
)

func TestBuildVariables(t *testing.T) {
	job := v1.CollectJob{
		Metadata: v1.Metadata{
			Name: "test-job",
		},
	}

	t.Run("built-in variables are set", func(t *testing.T) {
		variables, err := BuildVariables(job, nil)
		require.NoError(t, err)

		assert.Equal(t, "test-job", variables["JOB_NAME"])
		assert.NotEmpty(t, variables["JOB_DATE_ISO8601"])
		assert.NotEmpty(t, variables["JOB_DATE_RFC3339"])

		// Verify date formats
		_, err = time.Parse("20060102T150405Z", variables["JOB_DATE_ISO8601"])
		require.NoError(t, err, "JOB_DATE_ISO8601 should be valid ISO8601 basic format")

		_, err = time.Parse(time.RFC3339, variables["JOB_DATE_RFC3339"])
		require.NoError(t, err, "JOB_DATE_RFC3339 should be valid RFC3339 format")
	})

	t.Run("allowed env variables are included", func(t *testing.T) {
		t.Setenv("TEST_VAR", "test-value")

		variables, err := BuildVariables(job, []string{"TEST_VAR"})
		require.NoError(t, err)

		assert.Equal(t, "test-value", variables["TEST_VAR"])
	})

	t.Run("multiple allowed env variables", func(t *testing.T) {
		t.Setenv("VAR1", "value1")
		t.Setenv("VAR2", "value2")

		variables, err := BuildVariables(job, []string{"VAR1", "VAR2"})
		require.NoError(t, err)

		assert.Equal(t, "value1", variables["VAR1"])
		assert.Equal(t, "value2", variables["VAR2"])
	})

	t.Run("error when allowed env variable is not set", func(t *testing.T) {
		_, err := BuildVariables(job, []string{"UNSET_VAR"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "UNSET_VAR")
		assert.Contains(t, err.Error(), "is not set")
	})

	t.Run("error accumulates for multiple missing env variables", func(t *testing.T) {
		_, err := BuildVariables(job, []string{"MISSING1", "MISSING2"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MISSING1")
		assert.Contains(t, err.Error(), "MISSING2")
	})

	t.Run("empty allowed env list", func(t *testing.T) {
		variables, err := BuildVariables(job, []string{})
		require.NoError(t, err)

		// Should only have built-in variables
		assert.Len(t, variables, 3)
	})
}

func TestExpandTemplates_CollectJob(t *testing.T) {
	t.Run("expands HTTP collector fields", func(t *testing.T) {
		job := v1.CollectJob{
			Spec: v1.CollectJobSpec{
				Collectors: []v1.Collector{
					{
						ID: "api",
						HTTP: &v1.HTTPCollector{
							BaseURL: "https://${API_HOST}",
							Headers: map[string]string{
								"Authorization": "Bearer ${API_TOKEN}",
							},
							Auth: &v1.HTTPAuth{
								Basic: &v1.HTTPBasicAuth{
									Username: "${USERNAME}",
									Password: "${PASSWORD}",
								},
							},
						},
					},
				},
			},
		}

		variables := map[string]string{
			"API_HOST":  "api.example.com",
			"API_TOKEN": "secret123",
			"USERNAME":  "user",
			"PASSWORD":  "pass",
		}

		err := ExpandTemplates(&job, variables)
		require.NoError(t, err)

		assert.Equal(t, "https://api.example.com", job.Spec.Collectors[0].HTTP.BaseURL)
		assert.Equal(t, "Bearer secret123", job.Spec.Collectors[0].HTTP.Headers["Authorization"])
		assert.Equal(t, "user", job.Spec.Collectors[0].HTTP.Auth.Basic.Username)
		assert.Equal(t, "pass", job.Spec.Collectors[0].HTTP.Auth.Basic.Password)
	})

	t.Run("error on missing variable", func(t *testing.T) {
		job := v1.CollectJob{
			Spec: v1.CollectJobSpec{
				Collectors: []v1.Collector{
					{
						ID: "api",
						HTTP: &v1.HTTPCollector{
							BaseURL: "https://${MISSING_HOST}",
						},
					},
				},
			},
		}

		err := ExpandTemplates(&job, map[string]string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MISSING_HOST")
	})
}
