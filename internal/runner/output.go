package runner

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/infracollect/infracollect/internal/engine/archivers"
	"github.com/infracollect/infracollect/internal/engine/encoders"
	"github.com/infracollect/infracollect/internal/engine/sinks"
)

// buildOutputPipeline translates the parsed output {} block into an
// (encoder, sink) pair. When output is nil the pipeline defaults to a JSON
// encoder streaming to stdout, preserving the pre-output-block behaviour.
// When output is present but missing a sink child, it is a user error — an
// output block with no sink destination cannot do anything useful.
func buildOutputPipeline(
	ctx context.Context,
	output *OutputBlock,
	baseCtx *hcl.EvalContext,
	jobName string,
) (engine.Encoder, engine.Sink, error) {
	if output == nil {
		return encoders.NewJSONEncoder("  "), sinks.NewStreamSink(os.Stdout), nil
	}

	encoder, err := buildEncoder(output.Encoding, baseCtx)
	if err != nil {
		return nil, nil, err
	}

	if output.Sink == nil {
		return nil, nil, fmt.Errorf("output block requires a sink")
	}
	sink, err := buildSink(ctx, output.Sink, baseCtx)
	if err != nil {
		return nil, nil, err
	}

	if output.Archive != nil {
		archiver, archiveName, err := buildArchiver(output.Archive, baseCtx, jobName)
		if err != nil {
			return nil, nil, err
		}
		sink = sinks.NewArchiveSink(sink, archiver, archiveName)
	}

	return encoder, sink, nil
}

// decodeBlock runs gohcl.DecodeBody into dst and wraps the diagnostics into
// a single error labelled with the block's variant (`what` is the noun —
// "encoding", "archive", "sink"; kind is the first-label variant).
func decodeBlock[T any](what, kind string, body hcl.Body, ctx *hcl.EvalContext, dst *T) error {
	if diags := gohcl.DecodeBody(body, ctx, dst); diags.HasErrors() {
		return fmt.Errorf("failed to decode %s %q: %s", what, kind, diags.Error())
	}
	return nil
}

type jsonEncodingConfig struct {
	Indent string `hcl:"indent,optional"`
}

func buildEncoder(block *EncodingBlock, baseCtx *hcl.EvalContext) (engine.Encoder, error) {
	if block == nil {
		return encoders.NewJSONEncoder("  "), nil
	}
	switch block.Kind {
	case "json":
		cfg := jsonEncodingConfig{Indent: "  "}
		if err := decodeBlock("encoding", block.Kind, block.Body, baseCtx, &cfg); err != nil {
			return nil, err
		}
		return encoders.NewJSONEncoder(cfg.Indent), nil
	default:
		return nil, fmt.Errorf("unknown encoding kind %q (known: json)", block.Kind)
	}
}

type tarArchiveConfig struct {
	Compression string `hcl:"compression,optional"`
}

func buildArchiver(block *ArchiveBlock, baseCtx *hcl.EvalContext, jobName string) (engine.Archiver, string, error) {
	switch block.Kind {
	case "tar":
		var cfg tarArchiveConfig
		if err := decodeBlock("archive", block.Kind, block.Body, baseCtx, &cfg); err != nil {
			return nil, "", err
		}
		archiver, err := archivers.NewTarArchiver(cfg.Compression)
		if err != nil {
			return nil, "", fmt.Errorf("failed to build tar archiver: %w", err)
		}
		return archiver, jobName + archiver.Extension(), nil
	default:
		return nil, "", fmt.Errorf("unknown archive kind %q (known: tar)", block.Kind)
	}
}

type filesystemSinkConfig struct {
	Path string `hcl:"path"`
}

// s3SinkConfig decodes `sink "s3" { ... }` minus the nested credentials
// block, which the parser has already split off into block.Credentials.
type s3SinkConfig struct {
	Bucket         string `hcl:"bucket"`
	Region         string `hcl:"region,optional"`
	Endpoint       string `hcl:"endpoint,optional"`
	Prefix         string `hcl:"prefix,optional"`
	ForcePathStyle bool   `hcl:"force_path_style,optional"`
}

type s3CredentialsConfig struct {
	AccessKeyID     string `hcl:"access_key_id,optional"`
	SecretAccessKey string `hcl:"secret_access_key,optional"`
}

func buildSink(ctx context.Context, block *SinkBlock, baseCtx *hcl.EvalContext) (engine.Sink, error) {
	switch block.Kind {
	case "stdout":
		return sinks.NewStreamSink(os.Stdout), nil
	case "stderr":
		return sinks.NewStreamSink(os.Stderr), nil
	case "filesystem":
		var cfg filesystemSinkConfig
		if err := decodeBlock("sink", block.Kind, block.Body, baseCtx, &cfg); err != nil {
			return nil, err
		}
		sink, err := sinks.NewFilesystemSinkFromPath(cfg.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to build filesystem sink: %w", err)
		}
		return sink, nil
	case "s3":
		var cfg s3SinkConfig
		if err := decodeBlock("sink", block.Kind, block.Body, baseCtx, &cfg); err != nil {
			return nil, err
		}
		var creds s3CredentialsConfig
		if block.Credentials != nil {
			if err := decodeBlock("sink", block.Kind+" credentials", block.Credentials.Body, baseCtx, &creds); err != nil {
				return nil, err
			}
		}
		sink, err := sinks.NewS3Sink(ctx, sinks.S3Config{
			Bucket:          cfg.Bucket,
			Region:          cfg.Region,
			Endpoint:        cfg.Endpoint,
			Prefix:          cfg.Prefix,
			ForcePathStyle:  cfg.ForcePathStyle,
			AccessKeyID:     creds.AccessKeyID,
			SecretAccessKey: creds.SecretAccessKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to build s3 sink: %w", err)
		}
		return sink, nil
	default:
		return nil, fmt.Errorf("unknown sink kind %q (known: stdout, stderr, filesystem, s3)", block.Kind)
	}
}
