package gendocs

import (
	"strings"
)

// HCLTag represents a parsed hcl:"..." struct tag.
type HCLTag struct {
	Name     string // the HCL attribute/block/label name (empty for ",remain")
	Optional bool   // has ",optional" modifier
	Label    bool   // has ",label" modifier — field captures a block label
	Block    bool   // has ",block" modifier — field is a nested block
	Remain   bool   // tag is ",remain" — captures remaining body
}

// ParseHCLTag parses an hcl:"..." struct tag value (the part inside quotes).
// Returns nil if the tag is empty or not an hcl tag.
func ParseHCLTag(tagValue string) *HCLTag {
	if tagValue == "" {
		return nil
	}

	parts := strings.Split(tagValue, ",")
	tag := &HCLTag{
		Name: parts[0],
	}

	for _, mod := range parts[1:] {
		switch mod {
		case "optional":
			tag.Optional = true
		case "label":
			tag.Label = true
		case "block":
			tag.Block = true
		case "remain":
			tag.Remain = true
		}
	}

	return tag
}
