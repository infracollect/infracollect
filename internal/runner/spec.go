package runner

import (
	"fmt"

	v1 "github.com/infracollect/infracollect/apis/v1"
)

// ResolvedSpec holds a kind identifier and the spec for that kind.
type ResolvedSpec struct {
	Kind string
	Spec any
}

// ResolveCollectorSpec extracts the kind and spec from a v1.Collector.
// Returns an error if no collector type is specified.
func ResolveCollectorSpec(c v1.Collector) (ResolvedSpec, error) {
	switch {
	case c.Terraform != nil:
		return ResolvedSpec{Kind: "terraform", Spec: c.Terraform}, nil
	case c.HTTP != nil:
		return ResolvedSpec{Kind: "http", Spec: c.HTTP}, nil
	default:
		return ResolvedSpec{}, fmt.Errorf("collector %q has no type specified", c.ID)
	}
}

// ResolveStepSpec extracts the kind and spec from a v1.Step.
// Returns an error if no step type is specified.
func ResolveStepSpec(s v1.Step) (ResolvedSpec, error) {
	switch {
	case s.TerraformDataSource != nil:
		return ResolvedSpec{Kind: "terraform_datasource", Spec: s.TerraformDataSource}, nil
	case s.HTTPGet != nil:
		return ResolvedSpec{Kind: "http_get", Spec: s.HTTPGet}, nil
	case s.Static != nil:
		return ResolvedSpec{Kind: "static", Spec: s.Static}, nil
	default:
		return ResolvedSpec{}, fmt.Errorf("step %q has no type specified", s.ID)
	}
}
