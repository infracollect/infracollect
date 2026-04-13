package steps

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/infracollect/infracollect/internal/engine"
)

// StaticHCLConfig is the HCL-level shape of a `step "static" "<id>" { ... }` block.
// Filepath and Value are mutually exclusive; exactly one must be set. The
// choice is validated at construction time rather than at the HCL schema
// level because HCL labeled-block discrimination would cost more ergonomics
// than it buys for a two-arm union.
type StaticHCLConfig struct {
	Filepath *string `hcl:"filepath,optional"`
	Value    *string `hcl:"value,optional"`
	ParseAs  *string `hcl:"parse_as,optional"`
}

// ExecHCLConfig is the HCL-level shape of a `step "exec" "<id>" { ... }` block.
type ExecHCLConfig struct {
	Program    []string          `hcl:"program"`
	Input      *execInputBlock   `hcl:"input,block"`
	WorkingDir *string           `hcl:"working_dir,optional"`
	Timeout    *string           `hcl:"timeout,optional"`
	Format     *string           `hcl:"format,optional"`
	Env        map[string]string `hcl:"env,optional"`
}

// execInputBlock lets users supply a free-form attribute set as stdin for
// the child process. We use a nested block with `,remain` so the integration
// can evaluate the attributes against the runner's eval context (the values
// may reference env / collector / step outputs).
type execInputBlock struct {
	Body hcl.Body `hcl:",remain"`
}

func Register(registry *engine.Registry) error {
	return registry.RegisterSteps(
		engine.NewTypedStepDescriptorWithoutCollector(StaticStepKind, newStaticStep),
		engine.NewTypedStepDescriptorWithoutCollector(ExecStepKind, newExecStep),
	)
}

func newStaticStep(
	_ *engine.RegistryHelper,
	id string,
	_ *hcl.EvalContext,
	cfg StaticHCLConfig,
) (engine.Step, error) {
	return NewStaticStep(id, StaticStepConfig(cfg))
}

func newExecStep(
	helper *engine.RegistryHelper,
	id string,
	ctx *hcl.EvalContext,
	cfg ExecHCLConfig,
) (engine.Step, error) {
	allowedEnv := engine.MustGetRegistryDependency[[]string](helper, engine.AllowedEnvVarsDepKey)

	var input map[string]any
	if cfg.Input != nil {
		m, err := engine.EvalBodyToMap(cfg.Input.Body, ctx, "exec step input block")
		if err != nil {
			return nil, err
		}
		input = m
	}

	return NewExecStep(id, helper.Logger(), ExecStepConfig{
		Program:    cfg.Program,
		Input:      input,
		WorkingDir: cfg.WorkingDir,
		Timeout:    cfg.Timeout,
		Format:     cfg.Format,
		Env:        cfg.Env,
		AllowedEnv: allowedEnv,
	})
}
