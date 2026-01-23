package v1

type CollectJob struct {
	Kind     string         `yaml:"kind" json:"kind" validate:"required,eq=CollectJob"`
	Metadata Metadata       `yaml:"metadata" json:"metadata"`
	Spec     CollectJobSpec `yaml:"spec" json:"spec"`
}

type CollectJobSpec struct {
	Collectors []Collector `yaml:"collectors" json:"collectors"`
	Steps      []Step      `yaml:"steps" json:"steps"`
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
	Collector           string                   `yaml:"collector" json:"collector"`
	TerraformDataSource *TerraformDataSourceStep `yaml:"terraform_datasource,omitempty" json:"terraform_datasource,omitempty" validate:"excluded_with=HTTPGet"`
	HTTPGet             *HTTPGetStep             `yaml:"http_get,omitempty" json:"http_get,omitempty" validate:"excluded_with=TerraformDataSource"`
}

// TerraformDataSourceStep is a step that uses a Terraform provider's data source.
type TerraformDataSourceStep struct {
	// Name of the provider data source to use.
	Name string         `yaml:"name" json:"name"`
	Args map[string]any `yaml:"args" json:"args"`
}

type HTTPCollector struct {
	BaseURL  string            `yaml:"base_url" json:"base_url"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Auth     *HTTPAuth         `yaml:"auth,omitempty" json:"auth,omitempty"`
	Timeout  *int              `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Insecure bool              `yaml:"insecure,omitempty" json:"insecure,omitempty"`
}

type HTTPAuth struct {
	Basic *HTTPBasicAuth `yaml:"basic,omitempty" json:"basic,omitempty"`
}

type HTTPBasicAuth struct {
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
	Encoded  string `yaml:"encoded,omitempty" json:"encoded,omitempty"`
}

type HTTPGetStep struct {
	Path         string            `yaml:"path" json:"path"`
	Headers      map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Params       map[string]string `yaml:"params,omitempty" json:"params,omitempty"`
	ResponseType string            `yaml:"response_type,omitempty" json:"response_type,omitempty" validate:"oneof=json raw"`
}

// OutputSpec configures how results are written.
// The output system has three concerns:
//   - Encoding: How to format the data (JSON, YAML, etc.)
//   - Sink: Where to write (stdout, filesystem)
//
// Defaults: JSON encoding, stdout sink.
type OutputSpec struct {
	// Encoding configures the output format (default: json with compact output).
	Encoding *EncodingSpec `yaml:"encoding,omitempty" json:"encoding,omitempty"`

	// Sink configures where output is written (default: stdout for stream mode,
	// filesystem for files mode).
	Sink *SinkSpec `yaml:"sink,omitempty" json:"sink,omitempty"`
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
	Stdout *StdoutSinkSpec `yaml:"stdout,omitempty" json:"stdout,omitempty" validate:"excluded_with=Filesystem"`

	// Filesystem writes to files on the local filesystem.
	Filesystem *FilesystemSinkSpec `yaml:"filesystem,omitempty" json:"filesystem,omitempty" validate:"excluded_with=Stdout"`
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
	//   - $JOB_DATE_RFC3339: Current UTC time in RFC3339 format
	Prefix *string `yaml:"prefix,omitempty" json:"prefix,omitempty"`
}
