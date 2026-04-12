package gendocs

import (
	"fmt"
	"slices"
	"strings"
)

// ExtractSchema builds a DocSchema for a single registry target.
// The idx is a pre-built block header index for fast block-ref lookups.
func ExtractSchema(lp *LoadedPackages, reg *Registry, idx BlockHeaderIndex, target Target) (*DocSchema, error) {
	switch target.Kind {
	case KindPage:
		return extractPage(lp, target)
	case KindUnion:
		return extractUnion(lp, reg, target)
	default:
		return extractBlock(lp, idx, target)
	}
}

// extractBlock handles rootBlock, stepBlock, variant, and freeform targets.
func extractBlock(lp *LoadedPackages, idx BlockHeaderIndex, target Target) (*DocSchema, error) {
	info, err := lp.FindStruct(target.Package, target.Type)
	if err != nil {
		return nil, fmt.Errorf("target %s: %w", target.ID, err)
	}

	schema := &DocSchema{
		SchemaVersion: 2,
		ID:            target.ID,
		Name:          info.Name,
		BlockHeader:   target.BlockHeader,
		Description:   info.Doc,
	}

	for _, f := range info.Fields {
		if f.HCLTag == nil {
			continue // untagged field (e.g. DefRange, Filename)
		}

		tag := f.HCLTag

		// Skip label fields — they're part of the block header, not the body.
		if tag.Label {
			continue
		}

		// ",remain" body
		if tag.Remain {
			schema.Remain = &DocRemain{
				Description: f.Doc,
			}
			continue
		}

		// Block reference.
		// In gohcl, blocks are optional when the Go field is a pointer or
		// slice — the ,optional modifier is for attributes only.
		if tag.Block {
			ref := findBlockRef(idx, tag.Name)
			required := !strings.HasPrefix(f.GoType, "*") && !strings.HasPrefix(f.GoType, "[]")
			schema.Blocks = append(schema.Blocks, DocBlockRef{
				Name:        tag.Name,
				Ref:         ref,
				Required:    required,
				Description: f.Doc,
			})
			continue
		}

		// Regular attribute
		goType := cleanGoType(f.GoType)
		attr := DocAttribute{
			Name:        tag.Name,
			Type:        goType,
			Required:    !tag.Optional,
			Description: f.Doc,
			Default:     ExtractDefault(f.Doc),
			Enum:        ExtractEnum(f.Doc),
		}
		schema.Attributes = append(schema.Attributes, attr)
	}

	return schema, nil
}

// extractUnion builds a union schema from its variants.
func extractUnion(lp *LoadedPackages, reg *Registry, target Target) (*DocSchema, error) {
	info, err := lp.FindStruct(target.Package, target.Type)
	if err != nil {
		return nil, fmt.Errorf("target %s: %w", target.ID, err)
	}

	schema := &DocSchema{
		SchemaVersion: 2,
		ID:            target.ID,
		Name:          info.Name,
		BlockHeader:   target.BlockHeader,
		Description:   info.Doc,
		Kind:          SchemaKindUnion,
	}

	// Find the label field name.
	for _, f := range info.Fields {
		if f.HCLTag != nil && f.HCLTag.Label {
			schema.LabelName = f.HCLTag.Name
			break
		}
	}

	// If no variants are declared, this is a freeform/open-label union.
	if len(target.Variants) == 0 {
		schema.Kind = SchemaKindFreeform
		return schema, nil
	}

	// Build variant list in deterministic order (sorted by label).
	labels := make([]string, 0, len(target.Variants))
	for k := range target.Variants {
		labels = append(labels, k)
	}
	slices.Sort(labels)
	for _, label := range labels {
		ref := target.Variants[label]
		v := DocVariant{Label: label}
		if ref != nil {
			v.Ref = *ref
		}
		schema.Variants = append(schema.Variants, v)
	}

	return schema, nil
}

// extractPage builds schemas for a multi-type page target.
func extractPage(lp *LoadedPackages, target Target) (*DocSchema, error) {
	schema := &DocSchema{
		SchemaVersion: 2,
		ID:            target.ID,
		Name:          target.ID,
		Kind:          SchemaKindPage,
	}

	// For page targets, we embed sub-schemas as blocks.
	for _, pt := range target.Types {
		info, err := lp.FindStruct(pt.Package, pt.Type)
		if err != nil {
			return nil, fmt.Errorf("target %s type %s: %w", target.ID, pt.Type, err)
		}

		schema.Blocks = append(schema.Blocks, DocBlockRef{
			Name:        pt.BlockHeader,
			Description: info.Doc,
		})
	}

	return schema, nil
}

// BlockHeaderIndex maps block header base names (e.g. "auth", "sink") to
// target IDs for O(1) lookups. Build once with BuildBlockHeaderIndex.
type BlockHeaderIndex map[string]string

// BuildBlockHeaderIndex builds an index from block header base names to
// target IDs for fast lookups in findBlockRef.
func BuildBlockHeaderIndex(reg *Registry) BlockHeaderIndex {
	idx := make(BlockHeaderIndex, len(reg.Targets))
	for _, t := range reg.Targets {
		if t.BlockHeader == "" {
			continue
		}
		base := strings.SplitN(t.BlockHeader, " ", 2)[0]
		idx[base] = t.ID
	}
	return idx
}

// findBlockRef looks up the registry target ID for a nested block name.
func findBlockRef(idx BlockHeaderIndex, blockName string) string {
	return idx[blockName]
}

// cleanGoType simplifies a Go type for display in docs.
// Removes pointer prefix since optionality is already tracked.
func cleanGoType(goType string) string {
	goType = strings.TrimPrefix(goType, "*")

	// Map well-known types to friendlier names.
	switch goType {
	case "int", "int64", "int32":
		return "number"
	case "bool":
		return "bool"
	case "string":
		return "string"
	case "[]string":
		return "list(string)"
	case "map[string]string":
		return "map(string)"
	case "map[string]any", "map[string]interface{}":
		return "map(any)"
	}

	return goType
}

