package sinks

import (
	"context"
	"fmt"
	"io"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/infracollect/infracollect/internal/engine"
)

// S3Uploader is an interface for uploading objects to S3.
// This allows for easy mocking in tests.
type S3Uploader interface {
	Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

// S3Config contains configuration for the S3 sink.
type S3Config struct {
	Bucket          string
	Region          string
	Endpoint        string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
}

// S3Sink writes output to S3-compatible object storage.
type S3Sink struct {
	bucket   string
	prefix   string
	uploader S3Uploader
}

// NewS3Sink creates a new S3 sink with the given configuration.
func NewS3Sink(ctx context.Context, cfg S3Config) (engine.Sink, error) {
	var opts []func(*config.LoadOptions) error

	// Set region if provided
	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}

	// Set explicit credentials if provided
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Build S3 client options
	var s3Opts []func(*s3.Options)

	// Set custom endpoint for S3-compatible services (R2, MinIO, etc.)
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	// Force path-style addressing for MinIO and some S3-compatible services
	if cfg.ForcePathStyle {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)
	uploader := manager.NewUploader(client)

	return &S3Sink{
		bucket:   cfg.Bucket,
		prefix:   cfg.Prefix,
		uploader: uploader,
	}, nil
}

// NewS3SinkWithUploader creates a new S3 sink with a custom uploader.
// This is useful for testing.
func NewS3SinkWithUploader(bucket, prefix string, uploader S3Uploader) engine.Sink {
	return &S3Sink{
		bucket:   bucket,
		prefix:   prefix,
		uploader: uploader,
	}
}

func (s *S3Sink) Name() string {
	if s.prefix != "" {
		return fmt.Sprintf("s3(%s/%s)", s.bucket, s.prefix)
	}
	return fmt.Sprintf("s3(%s)", s.bucket)
}

func (s *S3Sink) Kind() string {
	return "s3"
}

func (s *S3Sink) Write(ctx context.Context, objectPath string, data io.Reader) error {
	key := objectPath
	if s.prefix != "" {
		key = path.Join(s.prefix, objectPath)
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   data,
	}

	// Set Content-Type based on file extension
	if contentType := contentTypeFromPath(objectPath); contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	_, err := s.uploader.Upload(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload to s3://%s/%s: %w", s.bucket, key, err)
	}

	return nil
}

// contentTypeFromPath returns the Content-Type based on the file extension.
func contentTypeFromPath(p string) string {
	ext := path.Ext(p)
	switch ext {
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/x-yaml"
	case ".xml":
		return "application/xml"
	case ".txt":
		return "text/plain"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	case ".zst":
		return "application/zstd"
	default:
		return ""
	}
}

func (s *S3Sink) Close(ctx context.Context) error {
	return nil
}
