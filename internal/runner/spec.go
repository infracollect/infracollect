package runner

import (
	"fmt"

	v1 "github.com/infracollect/infracollect/apis/v1"
)

type InvalidCollectorSpecError struct {
	Collector v1.Collector
}

func (e *InvalidCollectorSpecError) Error() string {
	return fmt.Sprintf("invalid collector spec: %s", e.Collector.ID)
}

type InvalidStepSpecError struct {
	Step v1.Step
}

func (e *InvalidStepSpecError) Error() string {
	return fmt.Sprintf("invalid step spec: %s", e.Step.ID)
}

type ResolvedSpec struct {
	Kind string
	Spec any
}

func ResolveCollectorSpec(c v1.Collector) (ResolvedSpec, error) {
	switch {
	case c.Terraform != nil:
		return ResolvedSpec{Kind: "terraform", Spec: *c.Terraform}, nil
	case c.HTTP != nil:
		return ResolvedSpec{Kind: "http", Spec: *c.HTTP}, nil
	default:
		return ResolvedSpec{}, &InvalidCollectorSpecError{Collector: c}
	}
}

func ResolveStepSpec(s v1.Step) (ResolvedSpec, error) {
	switch {
	case s.TerraformDataSource != nil:
		return ResolvedSpec{Kind: "terraform_datasource", Spec: *s.TerraformDataSource}, nil
	case s.HTTPGet != nil:
		return ResolvedSpec{Kind: "http_get", Spec: *s.HTTPGet}, nil
	case s.Static != nil:
		return ResolvedSpec{Kind: "static", Spec: *s.Static}, nil
	case s.Exec != nil:
		return ResolvedSpec{Kind: "exec", Spec: *s.Exec}, nil
	default:
		return ResolvedSpec{}, &InvalidStepSpecError{Step: s}
	}
}
