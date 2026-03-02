package runner

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/infracollect/infracollect/internal/runner/hclfuncs"
	"github.com/zclconf/go-cty/cty"
)

// BuildBaseEvalContext produces the root hcl.EvalContext for a job. It
// populates:
//
//   - env.<VAR>: one entry per variable in allowedEnv, looked up from the
//     process environment. A missing entry is a hard error — callers must
//     pass an explicit --pass-env list.
//   - job.name: the effective job name from the optional job block.
//   - functions: timestamp, timeadd, formatdate (see hclfuncs/datetime.go).
//
// It does NOT populate step.* or collector.* — those are layered in per-node
// at execution time once predecessors have completed. It also does not
// populate each.* — that lives only inside a for_each iteration scope.
func BuildBaseEvalContext(tmpl *JobTemplate, allowedEnv []string) (*hcl.EvalContext, error) {
	envMap := map[string]cty.Value{}
	for _, name := range allowedEnv {
		val, ok := os.LookupEnv(name)
		if !ok {
			return nil, fmt.Errorf("environment variable %q is not set", name)
		}
		envMap[name] = cty.StringVal(val)
	}

	// cty.ObjectVal panics on an empty attribute map; substitute the empty
	// object sentinel when no env vars are in the allow-list.
	envVal := cty.EmptyObjectVal
	if len(envMap) > 0 {
		envVal = cty.ObjectVal(envMap)
	}

	jobVal := cty.ObjectVal(map[string]cty.Value{
		"name": cty.StringVal(tmpl.JobName()),
	})

	return &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"env": envVal,
			"job": jobVal,
		},
		Functions: hclfuncs.Datetime(),
	}, nil
}
