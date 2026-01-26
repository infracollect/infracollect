package runner

import (
	"os"
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
		os.Unsetenv("UNSET_VAR")

		_, err := BuildVariables(job, []string{"UNSET_VAR"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "UNSET_VAR")
		assert.Contains(t, err.Error(), "is not set")
	})

	t.Run("error accumulates for multiple missing env variables", func(t *testing.T) {
		os.Unsetenv("MISSING1")
		os.Unsetenv("MISSING2")

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

	t.Run("expands S3 sink fields", func(t *testing.T) {
		prefix := "${JOB_NAME}/${JOB_DATE_ISO8601}/"
		region := "${AWS_REGION}"
		accessKey := "${AWS_ACCESS_KEY_ID}"
		secretKey := "${AWS_SECRET_ACCESS_KEY}"

		job := v1.CollectJob{
			Spec: v1.CollectJobSpec{
				Output: &v1.OutputSpec{
					Sink: &v1.SinkSpec{
						S3: &v1.S3SinkSpec{
							Bucket:   "${S3_BUCKET}",
							Prefix:   &prefix,
							Region:   &region,
							Credentials: &v1.S3Credentials{
								AccessKeyID:     accessKey,
								SecretAccessKey: secretKey,
							},
						},
					},
				},
			},
		}

		variables := map[string]string{
			"JOB_NAME":              "test-job",
			"JOB_DATE_ISO8601":      "20260126T120000Z",
			"S3_BUCKET":             "my-bucket",
			"AWS_REGION":            "us-east-1",
			"AWS_ACCESS_KEY_ID":     "AKIAIOSFODNN7EXAMPLE",
			"AWS_SECRET_ACCESS_KEY": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}

		err := ExpandTemplates(&job, variables)
		require.NoError(t, err)

		assert.Equal(t, "my-bucket", job.Spec.Output.Sink.S3.Bucket)
		assert.Equal(t, "test-job/20260126T120000Z/", *job.Spec.Output.Sink.S3.Prefix)
		assert.Equal(t, "us-east-1", *job.Spec.Output.Sink.S3.Region)
		assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", job.Spec.Output.Sink.S3.Credentials.AccessKeyID)
		assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", job.Spec.Output.Sink.S3.Credentials.SecretAccessKey)
	})

	t.Run("expands archive name field", func(t *testing.T) {
		job := v1.CollectJob{
			Spec: v1.CollectJobSpec{
				Output: &v1.OutputSpec{
					Archive: &v1.ArchiveSpec{
						Format: "tar",
						Name:   "${JOB_NAME}-${JOB_DATE_ISO8601}",
					},
				},
			},
		}

		variables := map[string]string{
			"JOB_NAME":         "daily-backup",
			"JOB_DATE_ISO8601": "20260126T120000Z",
		}

		err := ExpandTemplates(&job, variables)
		require.NoError(t, err)

		assert.Equal(t, "daily-backup-20260126T120000Z", job.Spec.Output.Archive.Name)
	})

	t.Run("expands static step value", func(t *testing.T) {
		value := `{"job": "${JOB_NAME}"}`
		job := v1.CollectJob{
			Spec: v1.CollectJobSpec{
				Steps: []v1.Step{
					{
						ID: "config",
						Static: &v1.StaticStep{
							Value: &value,
						},
					},
				},
			},
		}

		variables := map[string]string{
			"JOB_NAME": "my-job",
		}

		err := ExpandTemplates(&job, variables)
		require.NoError(t, err)

		assert.Equal(t, `{"job": "my-job"}`, *job.Spec.Steps[0].Static.Value)
	})

	t.Run("expands filesystem sink prefix", func(t *testing.T) {
		prefix := "${JOB_NAME}/${JOB_DATE_RFC3339}"
		job := v1.CollectJob{
			Spec: v1.CollectJobSpec{
				Output: &v1.OutputSpec{
					Sink: &v1.SinkSpec{
						Filesystem: &v1.FilesystemSinkSpec{
							Prefix: &prefix,
						},
					},
				},
			},
		}

		variables := map[string]string{
			"JOB_NAME":         "test",
			"JOB_DATE_RFC3339": "2026-01-26T12:00:00Z",
		}

		err := ExpandTemplates(&job, variables)
		require.NoError(t, err)

		assert.Equal(t, "test/2026-01-26T12:00:00Z", *job.Spec.Output.Sink.Filesystem.Prefix)
	})

	t.Run("expands HTTP get step headers and params", func(t *testing.T) {
		job := v1.CollectJob{
			Spec: v1.CollectJobSpec{
				Steps: []v1.Step{
					{
						ID: "data",
						HTTPGet: &v1.HTTPGetStep{
							Path: "/api/data",
							Headers: map[string]string{
								"X-API-Key": "${API_KEY}",
							},
							Params: map[string]string{
								"date": "${JOB_DATE_ISO8601}",
							},
						},
					},
				},
			},
		}

		variables := map[string]string{
			"API_KEY":          "secret-key",
			"JOB_DATE_ISO8601": "20260126T120000Z",
		}

		err := ExpandTemplates(&job, variables)
		require.NoError(t, err)

		assert.Equal(t, "secret-key", job.Spec.Steps[0].HTTPGet.Headers["X-API-Key"])
		assert.Equal(t, "20260126T120000Z", job.Spec.Steps[0].HTTPGet.Params["date"])
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
