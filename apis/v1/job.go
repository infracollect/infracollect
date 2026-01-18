package v1

type CollectJob struct {
	Kind     string         `yaml:"kind" json:"kind"`
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
	Terraform *TerraformCollector `yaml:"terraform" json:"terraform"`
}

type TerraformCollector struct {
	Provider string         `yaml:"provider" json:"provider"`
	Version  string         `yaml:"version" json:"version"`
	Args     map[string]any `yaml:"args" json:"args"`
}

type Step struct {
	ID                  string                   `yaml:"id" json:"id"`
	TerraformDataSource *TerraformDataSourceStep `yaml:"terraform_datasource" json:"terraform_datasource"`
}

type TerraformDataSourceStep struct {
	Name      string         `yaml:"name" json:"name"`
	Collector string         `yaml:"collector" json:"collector"`
	Args      map[string]any `yaml:"args" json:"args"`
}

// OutputSpec configures how results are written.
type OutputSpec struct {
	// Encoding configures the output format (default: json with compact output).
	Encoding *EncodingSpec `yaml:"encoding,omitempty" json:"encoding,omitempty"`

	// Destination configures where output is written (default: stdout).
	Destination *DestinationSpec `yaml:"destination,omitempty" json:"destination,omitempty"`
}

// EncodingSpec configures the encoder (one of the fields should be set).
type EncodingSpec struct {
	JSON *JSONEncodingSpec `yaml:"json,omitempty" json:"json,omitempty"`
	// YAML *YAMLEncodingSpec - future
}

// JSONEncodingSpec configures JSON encoding.
type JSONEncodingSpec struct {
	// Indent specifies indentation. Empty = compact, "  " = 2 spaces, "\t" = tabs.
	Indent string `yaml:"indent,omitempty" json:"indent,omitempty"`
}

// DestinationSpec configures output destination (one of the fields should be set).
type DestinationSpec struct {
	Stdout *StdoutSpec `yaml:"stdout,omitempty" json:"stdout,omitempty"`
	Folder *FolderSpec `yaml:"folder,omitempty" json:"folder,omitempty"`
	Zip    *ZipSpec    `yaml:"zip,omitempty" json:"zip,omitempty"`
}

// StdoutSpec configures stdout output (no options currently).
type StdoutSpec struct{}

// FolderSpec configures folder output with one file per step.
type FolderSpec struct {
	// Path is the directory path to write output files.
	Path string `yaml:"path" json:"path"`
}

// ZipSpec configures ZIP archive output.
type ZipSpec struct {
	// Path is the path to the ZIP file to create.
	Path string `yaml:"path" json:"path"`
}
