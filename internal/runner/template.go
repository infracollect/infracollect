// Package runner provides HCL parsing and pipeline orchestration for
// infracollect collect jobs.
//
// The parser produces a JobTemplate: the authorable, pre-evaluation shape of
// a collect job. It deliberately does not touch the integration-specific
// schemas — each integration owns its own gohcl-tagged config struct and
// decodes the variant body at execution time, once the eval context has been
// layered with step results.
package runner

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// JobTemplate is the parse-time shape of a collect job. It describes a
// template that can fan out into many resolved invocations at execution
// time; it is not the shape of an already-resolved job.
type JobTemplate struct {
	Job        *JobBlock         `hcl:"job,block"`
	Collectors []*CollectorBlock `hcl:"collector,block"`
	Steps      []*StepBlock      `hcl:"step,block"`
	Output     *OutputBlock      `hcl:"output,block"`

	// Source file name used for diagnostic rendering. Populated by
	// ParseJobTemplate. Untagged so gohcl ignores it.
	Filename string
}

// JobBlock carries top-level job metadata. It is optional: when omitted, the
// pipeline generates a default name.
type JobBlock struct {
	Name string `hcl:"name,optional"`
}

// CollectorBlock is the outer shape of a collector. The inner body stays as
// hcl.Body for integration-specific decoding during Phase 2.
type CollectorBlock struct {
	Type string   `hcl:"type,label"`
	Name string   `hcl:"name,label"`
	Body hcl.Body `hcl:",remain"`

	// DefRange is the source range of the block definition itself, used for
	// diagnostic messages after parsing. Untagged so gohcl ignores it.
	DefRange hcl.Range
}

// StepBlock is the outer shape of a step. ForEach and Collector are
// unevaluated expressions; the runner evaluates them at execution time —
// ForEach to a cty collection, Collector to a traversal whose first three
// segments identify a collector node.
//
// Neither ForEach nor Collector is a gohcl-tagged field: gohcl's handling of
// optional hcl.Expression synthesizes a null staticExpr when the attribute is
// absent, making "declared vs. not declared" unobservable. We extract them
// manually after gohcl decode (see splitStepMeta), matching Terraform's
// configs/resource.go pattern.
type StepBlock struct {
	Type string   `hcl:"type,label"`
	Name string   `hcl:"name,label"`
	Body hcl.Body `hcl:",remain"`

	// Populated by splitStepMeta when the step's body contained these
	// attributes. Nil otherwise.
	ForEach   hcl.Expression
	Collector hcl.Expression

	// Untagged so gohcl ignores it.
	DefRange hcl.Range
}

// OutputBlock wraps the output configuration. Its children are labeled
// sub-blocks whose first label selects the variant (json encoding, tar
// archive, s3 sink, ...). The inner bodies stay unevaluated for the
// respective integration factories to decode; runner execution does not
// consume them yet — the runner returns collected results to the caller
// and the CLI is responsible for writing output until per-integration
// output factories land.
type OutputBlock struct {
	Encoding *EncodingBlock `hcl:"encoding,block"`
	Archive  *ArchiveBlock  `hcl:"archive,block"`
	Sink     *SinkBlock     `hcl:"sink,block"`
}

// EncodingBlock is `encoding "<kind>" { ... }`.
type EncodingBlock struct {
	Kind string   `hcl:"kind,label"`
	Body hcl.Body `hcl:",remain"`
}

// ArchiveBlock is `archive "<kind>" { ... }`.
type ArchiveBlock struct {
	Kind string   `hcl:"kind,label"`
	Body hcl.Body `hcl:",remain"`
}

// SinkBlock is `sink "<kind>" { ... }`. The nested `credentials {}` block
// is unlabeled because credentials are not a discriminated union — every
// sink that takes credentials takes the same shape.
type SinkBlock struct {
	Kind        string            `hcl:"kind,label"`
	Credentials *CredentialsBlock `hcl:"credentials,block"`
	Body        hcl.Body          `hcl:",remain"`
}

// CredentialsBlock is the shared `credentials { ... }` sub-block used by
// sinks that need authenticated access. Its free-form body is evaluated by
// the sink factory at execution time.
type CredentialsBlock struct {
	Body hcl.Body `hcl:",remain"`
}

// JobName returns the effective job name, generating a default when the
// optional job block is absent or the name is empty.
func (t *JobTemplate) JobName() string {
	if t.Job != nil && t.Job.Name != "" {
		return t.Job.Name
	}
	if t.Filename != "" {
		base := strings.TrimSuffix(filepath.Base(t.Filename), filepath.Ext(t.Filename))
		if base != "" && base != "." {
			return base
		}
	}
	return "infracollect-job"
}

// ParseJobTemplate parses raw HCL bytes into a JobTemplate and runs the
// semantic checks HCL cannot do on its own (unknown block types, duplicate
// second-labels). Every error is an hcl.Diagnostic with a source range
// pointing at the offending bytes.
func ParseJobTemplate(data []byte, filename string) (*JobTemplate, hcl.Diagnostics) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(data, filename)
	if diags.HasErrors() || file == nil {
		return nil, diags
	}

	var tmpl JobTemplate
	diags = append(diags, gohcl.DecodeBody(file.Body, nil, &tmpl)...)
	if diags.HasErrors() {
		return nil, diags
	}
	tmpl.Filename = filename

	// gohcl does not populate DefRange on ,remain-bearing structs; pull it
	// from the raw body content so diagnostics can point at the block header.
	diags = append(diags, populateDefRanges(file.Body, &tmpl)...)

	// for_each and collector are extracted manually because gohcl cannot
	// distinguish an absent optional hcl.Expression from a present null
	// expression.
	diags = append(diags, splitStepMeta(&tmpl)...)

	diags = append(diags, validateUniqueLabels(&tmpl)...)

	if diags.HasErrors() {
		return nil, diags
	}
	return &tmpl, diags
}

// populateDefRanges walks the raw file body and copies block def-ranges onto
// the decoded CollectorBlock / StepBlock values, in declaration order. This
// is a shortcut around gohcl not exposing the source range for a ,remain
// block directly.
func populateDefRanges(body hcl.Body, tmpl *JobTemplate) hcl.Diagnostics {
	content, _, diags := body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "job"},
			{Type: "collector", LabelNames: []string{"type", "name"}},
			{Type: "step", LabelNames: []string{"type", "name"}},
			{Type: "output"},
		},
	})

	var collIdx, stepIdx int
	for _, block := range content.Blocks {
		switch block.Type {
		case "collector":
			if collIdx < len(tmpl.Collectors) {
				tmpl.Collectors[collIdx].DefRange = block.DefRange
				collIdx++
			}
		case "step":
			if stepIdx < len(tmpl.Steps) {
				tmpl.Steps[stepIdx].DefRange = block.DefRange
				stepIdx++
			}
		}
	}

	return diags
}

// splitStepMeta walks the decoded steps and extracts the `for_each` and
// `collector` attributes from each step's Body into dedicated fields. The
// remaining body (everything other than those two attributes) replaces
// step.Body so integration-local gohcl decode never sees runner-owned
// attributes, and so downstream reference extraction does not double-count
// dependencies.
func splitStepMeta(tmpl *JobTemplate) hcl.Diagnostics {
	var diags hcl.Diagnostics
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{Name: "for_each", Required: false},
			{Name: "collector", Required: false},
		},
	}
	for _, s := range tmpl.Steps {
		if s.Body == nil {
			continue
		}
		content, remain, d := s.Body.PartialContent(schema)
		diags = append(diags, d...)
		if attr, ok := content.Attributes["for_each"]; ok {
			s.ForEach = attr.Expr
		}
		if attr, ok := content.Attributes["collector"]; ok {
			s.Collector = attr.Expr
		}
		s.Body = remain
	}
	return diags
}

func validateUniqueLabels(tmpl *JobTemplate) hcl.Diagnostics {
	var diags hcl.Diagnostics

	type key struct{ typ, name string }

	seenColl := map[key]hcl.Range{}
	for _, c := range tmpl.Collectors {
		if c.Name == "" {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Empty collector name",
				Detail:   "Collector second label (the ID) must not be empty.",
				Subject:  c.DefRange.Ptr(),
			})
			continue
		}
		k := key{c.Type, c.Name}
		if prev, ok := seenColl[k]; ok {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Duplicate collector",
				Detail: fmt.Sprintf(
					"A collector %q %q is already declared at %s.",
					c.Type, c.Name, prev.String(),
				),
				Subject: c.DefRange.Ptr(),
			})
			continue
		}
		seenColl[k] = c.DefRange
	}

	seenStep := map[key]hcl.Range{}
	for _, s := range tmpl.Steps {
		if s.Name == "" {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Empty step name",
				Detail:   "Step second label (the ID) must not be empty.",
				Subject:  s.DefRange.Ptr(),
			})
			continue
		}
		k := key{s.Type, s.Name}
		if prev, ok := seenStep[k]; ok {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Duplicate step",
				Detail: fmt.Sprintf(
					"A step %q %q is already declared at %s.",
					s.Type, s.Name, prev.String(),
				),
				Subject: s.DefRange.Ptr(),
			})
			continue
		}
		seenStep[k] = s.DefRange
	}

	return diags
}

