package sinks

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUploader struct {
	uploads []mockUpload
}

type mockUpload struct {
	bucket      string
	key         string
	body        []byte
	contentType string
}

func (m *mockUploader) Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
	body, _ := io.ReadAll(input.Body)
	upload := mockUpload{
		bucket: *input.Bucket,
		key:    *input.Key,
		body:   body,
	}
	if input.ContentType != nil {
		upload.contentType = *input.ContentType
	}
	m.uploads = append(m.uploads, upload)
	return &manager.UploadOutput{}, nil
}

func TestS3Sink_Name(t *testing.T) {
	tests := []struct {
		name     string
		bucket   string
		prefix   string
		expected string
	}{
		{
			name:     "bucket only",
			bucket:   "my-bucket",
			prefix:   "",
			expected: "s3(my-bucket)",
		},
		{
			name:     "bucket with prefix",
			bucket:   "my-bucket",
			prefix:   "data/exports",
			expected: "s3(my-bucket/data/exports)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := NewS3SinkWithUploader(tt.bucket, tt.prefix, &mockUploader{})
			assert.Equal(t, tt.expected, sink.Name())
		})
	}
}

func TestS3Sink_Kind(t *testing.T) {
	sink := NewS3SinkWithUploader("bucket", "", &mockUploader{})
	assert.Equal(t, "s3", sink.Kind())
}

func TestS3Sink_Write(t *testing.T) {
	tests := []struct {
		name           string
		bucket         string
		prefix         string
		path           string
		data           string
		expectedKey    string
		expectedBucket string
	}{
		{
			name:           "write without prefix",
			bucket:         "my-bucket",
			prefix:         "",
			path:           "test.json",
			data:           `{"key": "value"}`,
			expectedKey:    "test.json",
			expectedBucket: "my-bucket",
		},
		{
			name:           "write with prefix",
			bucket:         "my-bucket",
			prefix:         "exports/2024",
			path:           "test.json",
			data:           `{"key": "value"}`,
			expectedKey:    "exports/2024/test.json",
			expectedBucket: "my-bucket",
		},
		{
			name:           "write nested path with prefix",
			bucket:         "my-bucket",
			prefix:         "data",
			path:           "nested/path/file.json",
			data:           `{"nested": true}`,
			expectedKey:    "data/nested/path/file.json",
			expectedBucket: "my-bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uploader := &mockUploader{}
			sink := NewS3SinkWithUploader(tt.bucket, tt.prefix, uploader)

			err := sink.Write(t.Context(), tt.path, bytes.NewBufferString(tt.data))
			require.NoError(t, err)

			require.Len(t, uploader.uploads, 1)
			assert.Equal(t, tt.expectedBucket, uploader.uploads[0].bucket)
			assert.Equal(t, tt.expectedKey, uploader.uploads[0].key)
			assert.Equal(t, tt.data, string(uploader.uploads[0].body))
		})
	}
}

func TestS3Sink_Write_ContentType(t *testing.T) {
	tests := []struct {
		name                string
		path                string
		expectedContentType string
	}{
		{
			name:                "json file",
			path:                "data.json",
			expectedContentType: "application/json",
		},
		{
			name:                "yaml file",
			path:                "config.yaml",
			expectedContentType: "application/x-yaml",
		},
		{
			name:                "yml file",
			path:                "config.yml",
			expectedContentType: "application/x-yaml",
		},
		{
			name:                "xml file",
			path:                "data.xml",
			expectedContentType: "application/xml",
		},
		{
			name:                "txt file",
			path:                "readme.txt",
			expectedContentType: "text/plain",
		},
		{
			name:                "unknown extension",
			path:                "data.bin",
			expectedContentType: "",
		},
		{
			name:                "no extension",
			path:                "data",
			expectedContentType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uploader := &mockUploader{}
			sink := NewS3SinkWithUploader("bucket", "", uploader)

			err := sink.Write(t.Context(), tt.path, bytes.NewBufferString("content"))
			require.NoError(t, err)

			require.Len(t, uploader.uploads, 1)
			assert.Equal(t, tt.expectedContentType, uploader.uploads[0].contentType)
		})
	}
}
