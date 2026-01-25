package v1

type CollectJob struct {
	Kind     string         `yaml:"kind" json:"kind" validate:"required,eq=CollectJob"`
	Metadata Metadata       `yaml:"metadata" json:"metadata"`
	Spec     CollectJobSpec `yaml:"spec" json:"spec"`
}

type CollectJobSpec struct {
	Collectors []Collector `yaml:"collectors" json:"collectors" validate:"dive"`
	Steps      []Step      `yaml:"steps" json:"steps" validate:"dive"`
	Output     *OutputSpec `yaml:"output,omitempty" json:"output,omitempty"`
}

type Collector struct {
	ID        string              `yaml:"id" json:"id"`
	Terraform *TerraformCollector `yaml:"terraform,omitempty" json:"terraform,omitempty"`
	HTTP      *HTTPCollector      `yaml:"http,omitempty" json:"http,omitempty"`
}

type TerraformCollector struct {
	Provider string         `yaml:"provider" json:"provider"`
	Version  string         `yaml:"version" json:"version"`
	Args     map[string]any `yaml:"args" json:"args"`
}

type Step struct {
	ID                  string                   `yaml:"id" json:"id"`
	Collector           *string                  `yaml:"collector,omitempty" json:"collector,omitempty" validate:"required_with=TerraformDataSource HTTPGet"`
	TerraformDataSource *TerraformDataSourceStep `yaml:"terraform_datasource,omitempty" json:"terraform_datasource,omitempty" validate:"excluded_with=HTTPGet"`
	HTTPGet             *HTTPGetStep             `yaml:"http_get,omitempty" json:"http_get,omitempty" validate:"excluded_with=TerraformDataSource"`
	Static              *StaticStep              `yaml:"static,omitempty" json:"static,omitempty" validate:"excluded_with=TerraformDataSource HTTPGet Collector"`
}

// TerraformDataSourceStep is a step that uses a Terraform provider's data source.
type TerraformDataSourceStep struct {
	// Name of the provider data source to use.
	Name string         `yaml:"name" json:"name"`
	Args map[string]any `yaml:"args" json:"args"`
}

type HTTPCollector struct {
	// BaseURL is the base URL for the HTTP collector, such as "https://api.example.com".
	BaseURL string `yaml:"base_url" json:"base_url"`

	// Headers to include in all requests.
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Auth    *HTTPAuth         `yaml:"auth,omitempty" json:"auth,omitempty"`

	// Timeout is the request timeout in seconds.
	Timeout *int `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Insecure skips TLS certificate verification.
	Insecure bool `yaml:"insecure,omitempty" json:"insecure,omitempty"`
}

type HTTPAuth struct {
	Basic *HTTPBasicAuth `yaml:"basic,omitempty" json:"basic,omitempty"`
}

type HTTPBasicAuth struct {
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	// Encoded is a pre-encoded Base64-encoded credentials.
	Encoded string `yaml:"encoded,omitempty" json:"encoded,omitempty"`
}

type HTTPGetStep struct {
	// Path is the request path.
	Path string `yaml:"path" json:"path"`

	// Headers to include in the request.
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// Query parameters to append to the request URL.
	Params map[string]string `yaml:"params,omitempty" json:"params,omitempty"`

	// ResponseType is the format to parse the response as.
	ResponseType string `yaml:"response_type,omitempty" json:"response_type,omitempty" validate:"oneof=json raw"`
}

type StaticStep struct {
	// Filepath is a local and relative path to a file. Symlinks and directories are not allowed.
	Filepath *string `yaml:"filepath,omitempty" json:"filepath,omitempty" validate:"omitempty,required_without=Value,excluded_with=Value"`

	// Value is an inline value to use as the static value.
	Value *string `yaml:"value,omitempty" json:"value,omitempty" validate:"omitempty,required_without=Filepath,excluded_with=Filepath"`

	// ParseAs is the format to parse the value as.
	ParseAs *string `yaml:"parse_as,omitempty" json:"parse_as,omitempty" validate:"omitempty,oneof=json raw"`
}

// OutputSpec configures how results are written.
// The output system has three concerns:
//   - Encoding: How to format the data (JSON, YAML, etc.)
//   - Archive: How to bundle the data (tar with gzip/zstd compression)
//   - Sink: Where to write (stdout, filesystem)
//
// Defaults: JSON encoding, no archive, stdout sink.
type OutputSpec struct {
	// Encoding configures the output format (default: json with compact output).
	Encoding *EncodingSpec `yaml:"encoding,omitempty" json:"encoding,omitempty"`

	// Archive configures bundling output into a single archive file.
	// When set, all step results are collected into a tar archive with the
	// specified compression before being written to the sink.
	Archive *ArchiveSpec `yaml:"archive,omitempty" json:"archive,omitempty"`

	// Sink configures where output is written (default: stdout for stream mode,
	// filesystem for files mode).
	Sink *SinkSpec `yaml:"sink,omitempty" json:"sink,omitempty"`
}

// ArchiveSpec configures bundling output into an archive.
type ArchiveSpec struct {
	// Format is the archive format. Currently only "tar" is supported.
	Format string `yaml:"format" json:"format" validate:"required,oneof=tar"`

	// Compression algorithm
	Compression string `yaml:"compression,omitempty" json:"compression,omitempty" validate:"omitempty,oneof=gzip zstd none"`

	// Name is the archive base name. Supports template variables:
	//   - $JOB_NAME: The job's metadata.name
	//   - $JOB_DATE_ISO8601: Current UTC time in ISO8601 basic format (20060102T150405Z)
	//   - $JOB_DATE_RFC3339: Current UTC time in RFC3339 format (2006-01-02T15:04:05Z)
	// The appropriate file extension (e.g., ".tar.gz") is automatically appended.
	// Default: "$JOB_NAME".
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
}

// EncodingSpec configures the encoder. Exactly one field should be set.
// If none is set, defaults to compact JSON.
type EncodingSpec struct {
	// JSON configures JSON encoding.
	JSON *JSONEncodingSpec `yaml:"json,omitempty" json:"json,omitempty"`
	// YAML *YAMLEncodingSpec `yaml:"yaml,omitempty" json:"yaml,omitempty"` - future
}

// JSONEncodingSpec configures JSON encoding.
type JSONEncodingSpec struct {
	// Indent specifies indentation. Empty = compact, "  " = 2 spaces, "\t" = tabs.
	Indent string `yaml:"indent,omitempty" json:"indent,omitempty"`
}

// SinkSpec configures where output is written. Exactly one field should be set.
// If none is set, defaults based on mode: stdout for stream, filesystem for files.
type SinkSpec struct {
	// Stdout writes to standard output.
	Stdout *StdoutSinkSpec `yaml:"stdout,omitempty" json:"stdout,omitempty" validate:"excluded_with=Filesystem S3"`

	// Filesystem writes to files on the local filesystem.
	Filesystem *FilesystemSinkSpec `yaml:"filesystem,omitempty" json:"filesystem,omitempty" validate:"excluded_with=Stdout S3"`

	// S3 writes to S3-compatible object storage (AWS S3, Cloudflare R2, MinIO).
	S3 *S3SinkSpec `yaml:"s3,omitempty" json:"s3,omitempty" validate:"excluded_with=Stdout Filesystem"`
}

// StdoutSinkSpec configures stdout output.
type StdoutSinkSpec struct {
	// No configuration needed for stdout.
}

// FilesystemSinkSpec configures filesystem output.
type FilesystemSinkSpec struct {
	// Path is the directory to write files to (default: current directory).
	Path *string `yaml:"path,omitempty" json:"path,omitempty"`

	// Prefix is prepended to filenames. Supports variables:
	//   - $JOB_NAME: The job's metadata.name
	//   - $JOB_DATE_ISO8601: Current UTC time in ISO8601 basic format (20060102T150405Z) - recommended
	//   - $JOB_DATE_RFC3339: Current UTC time in RFC3339 format (2006-01-02T15:04:05Z)
	Prefix *string `yaml:"prefix,omitempty" json:"prefix,omitempty"`
}

// S3SinkSpec configures S3-compatible object storage output.
// Supports AWS S3, Cloudflare R2, MinIO, and other S3-compatible services.
type S3SinkSpec struct {
	// Bucket is the S3 bucket name.
	Bucket string `yaml:"bucket" json:"bucket" validate:"required"`

	// Region is the AWS region (optional, uses SDK defaults if not specified).
	Region *string `yaml:"region,omitempty" json:"region,omitempty"`

	// Endpoint is a custom endpoint URL for S3-compatible services (e.g., R2, MinIO).
	Endpoint *string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`

	// Prefix is prepended to object keys. Supports variables:
	//   - $JOB_NAME: The job's metadata.name
	//   - $JOB_DATE_ISO8601: Current UTC time in ISO8601 basic format (20060102T150405Z) - recommended
	//   - $JOB_DATE_RFC3339: Current UTC time in RFC3339 format (2006-01-02T15:04:05Z)
	//
	// Note: $JOB_DATE_ISO8601 is recommended for S3 keys as RFC3339 contains colons
	// which require URL encoding.
	Prefix *string `yaml:"prefix,omitempty" json:"prefix,omitempty"`

	// Credentials provides explicit credentials (optional, uses SDK credential chain if not specified).
	Credentials *S3Credentials `yaml:"credentials,omitempty" json:"credentials,omitempty"`

	// ForcePathStyle forces path-style addressing (required for MinIO and some S3-compatible services).
	ForcePathStyle bool `yaml:"force_path_style,omitempty" json:"force_path_style,omitempty"`
}

// S3Credentials provides explicit S3 credentials.
type S3Credentials struct {
	// AccessKeyID is the AWS access key ID.
	AccessKeyID string `yaml:"access_key_id" json:"access_key_id" validate:"required"`

	// SecretAccessKey is the AWS secret access key.
	SecretAccessKey string `yaml:"secret_access_key" json:"secret_access_key" validate:"required"`
}
