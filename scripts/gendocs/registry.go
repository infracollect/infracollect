package gendocs

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Registry is the top-level structure of docs/registry.yaml.
type Registry struct {
	Targets []Target `yaml:"targets"`
}

// Target describes one schema target for the doc generator.
type Target struct {
	ID          string            `yaml:"id"`
	Package     string            `yaml:"package,omitempty"`
	Type        string            `yaml:"type,omitempty"`
	Kind        TargetKind        `yaml:"kind"`
	BlockHeader string            `yaml:"blockHeader,omitempty"`
	Variants    map[string]*string `yaml:"variants,omitempty"` // label -> target id (nil = label-only, no body)
	Types       []PageType        `yaml:"types,omitempty"`     // only for kind: page
}

// PageType is a type entry within a "page" target.
type PageType struct {
	Package     string `yaml:"package"`
	Type        string `yaml:"type"`
	BlockHeader string `yaml:"blockHeader,omitempty"`
}

// TargetKind classifies what a target represents.
type TargetKind string

const (
	KindRootBlock TargetKind = "rootBlock"
	KindStepBlock TargetKind = "stepBlock"
	KindUnion     TargetKind = "union"
	KindVariant   TargetKind = "variant"
	KindFreeform  TargetKind = "freeform"
	KindPage      TargetKind = "page"
)

// LoadRegistry reads and parses a registry YAML file.
func LoadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading registry: %w", err)
	}

	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parsing registry: %w", err)
	}

	if err := reg.validate(); err != nil {
		return nil, err
	}

	return &reg, nil
}

func (r *Registry) validate() error {
	ids := make(map[string]bool, len(r.Targets))
	for _, t := range r.Targets {
		if t.ID == "" {
			return fmt.Errorf("target missing id")
		}
		if ids[t.ID] {
			return fmt.Errorf("duplicate target id: %s", t.ID)
		}
		ids[t.ID] = true

		switch t.Kind {
		case KindPage:
			if len(t.Types) == 0 {
				return fmt.Errorf("target %s: page kind requires types", t.ID)
			}
		case KindRootBlock, KindStepBlock, KindUnion, KindVariant, KindFreeform:
			if t.Package == "" || t.Type == "" {
				return fmt.Errorf("target %s: requires package and type", t.ID)
			}
		default:
			return fmt.Errorf("target %s: unknown kind %q", t.ID, t.Kind)
		}
	}
	return nil
}
