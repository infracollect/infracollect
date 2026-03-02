package runner

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// testRegistry returns a registry populated with no-op factories for the
// collector/step kinds used by the pipeline tests. BuildPipeline consults
// the descriptors for the known-kinds gate and the collector-binding rules,
// so each step's RequiresCollector / AllowedCollectorKinds must match the
// real integration's contract.
func testRegistry() *engine.Registry {
	reg := engine.NewRegistry(zap.NewNop())
	stubCollector := engine.CollectorFactory(func(*engine.RegistryHelper, hcl.Body, *hcl.EvalContext) (engine.Collector, hcl.Diagnostics) {
		return nil, nil
	})
	stubStep := engine.StepFactory(func(*engine.RegistryHelper, string, engine.Collector, hcl.Body, *hcl.EvalContext) (engine.Step, hcl.Diagnostics) {
		return nil, nil
	})
	for _, k := range []string{"terraform", "http"} {
		if err := reg.RegisterCollector(k, stubCollector); err != nil {
			panic(err)
		}
	}
	if err := reg.RegisterSteps(
		engine.StepDescriptor{
			Kind:                  "terraform_datasource",
			Factory:               stubStep,
			RequiresCollector:     true,
			AllowedCollectorKinds: []string{"terraform"},
		},
		engine.StepDescriptor{
			Kind:                  "http_get",
			Factory:               stubStep,
			RequiresCollector:     true,
			AllowedCollectorKinds: []string{"http"},
		},
		engine.StepDescriptor{Kind: "static", Factory: stubStep},
		engine.StepDescriptor{Kind: "exec", Factory: stubStep},
	); err != nil {
		panic(err)
	}
	return reg
}

func TestBuildPipeline_GoalDAG(t *testing.T) {
	src := []byte(`
job {
  name = "k8s-deployments-by-namespace"
}

collector "terraform" "k8s" {
  provider = "hashicorp/kubernetes"
}

step "terraform_datasource" "namespaces" {
  collector = collector.terraform.k8s

  datasource "kubernetes_resources" {
    api_version = "v1"
    kind        = "Namespace"
  }
}

step "terraform_datasource" "deployments" {
  collector = collector.terraform.k8s
  for_each  = step.terraform_datasource.namespaces.data.objects

  datasource "kubernetes_resources" {
    api_version = "apps/v1"
    kind        = "Deployment"
    namespace   = each.value.metadata.name
  }
}
`)

	tmpl, diags := ParseJobTemplate(src, "goal.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	p, diags := BuildPipeline(zap.NewNop(), tmpl, testRegistry())
	require.False(t, diags.HasErrors(), "build diags: %s", diags.Error())
	require.NotNil(t, p)

	collector := Node{Kind: NodeTypeCollector, Type: "terraform", ID: "k8s"}
	namespaces := Node{Kind: NodeTypeStep, Type: "terraform_datasource", ID: "namespaces"}
	deployments := Node{Kind: NodeTypeCollection, Type: "terraform_datasource", ID: "deployments"}

	_, ok := p.Meta(collector)
	assert.True(t, ok, "collector node missing")
	_, ok = p.Meta(namespaces)
	assert.True(t, ok, "namespaces node missing")
	meta, ok := p.Meta(deployments)
	assert.True(t, ok, "deployments node missing")
	require.NotNil(t, meta)
	assert.NotNil(t, meta.ForEach, "deployments should carry for_each expression")

	order, err := p.Dag().TopologicalSort()
	require.NoError(t, err)

	keys := make([]string, 0, len(order))
	for _, n := range order {
		keys = append(keys, n.Key())
	}

	assert.Less(t, indexOf(keys, collector.Key()), indexOf(keys, namespaces.Key()))
	assert.Less(t, indexOf(keys, collector.Key()), indexOf(keys, deployments.Key()))
	assert.Less(t, indexOf(keys, namespaces.Key()), indexOf(keys, deployments.Key()))
}

func TestBuildPipeline_SingleStep(t *testing.T) {
	src := []byte(`
step "static" "only" {
  value = "hello"
}
`)

	tmpl, diags := ParseJobTemplate(src, "one.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	p, diags := BuildPipeline(zap.NewNop(), tmpl, testRegistry())
	require.False(t, diags.HasErrors(), "build diags: %s", diags.Error())

	order, err := p.Dag().TopologicalSort()
	require.NoError(t, err)
	require.Len(t, order, 1)
	assert.Equal(t, "step:static:only", order[0].Key())
}

func TestBuildPipeline_Errors(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantMsg string
	}{
		{
			name: "dangling step reference",
			src: `
step "static" "consumer" {
  value = step.static.missing.output
}`,
			wantMsg: "Reference to unknown step",
		},
		{
			name: "dangling collector reference",
			src: `
step "terraform_datasource" "orphan" {
  collector = collector.terraform.missing
}`,
			wantMsg: "Reference to unknown collector",
		},
		{
			name: "each.* outside for_each",
			src: `
step "static" "bad" {
  value = each.value
}`,
			wantMsg: "each.* used outside a for_each step",
		},
		{
			name: "unknown collector type",
			src: `
collector "mystery" "x" {
}`,
			wantMsg: "Unknown collector type",
		},
		{
			name: "unknown step type",
			src: `
step "unknown_kind" "x" {
}`,
			wantMsg: "Unknown step type",
		},
		{
			name: "cycle via mutual step refs",
			src: `
step "static" "a" {
  value = step.static.b.data
}
step "static" "b" {
  value = step.static.a.data
}`,
			wantMsg: "Cycle in collect job DAG",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, diags := ParseJobTemplate([]byte(tc.src), "case.hcl")
			require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

			_, diags = BuildPipeline(zap.NewNop(), tmpl, testRegistry())
			require.True(t, diags.HasErrors())
			assert.Contains(t, diags.Error(), tc.wantMsg)
		})
	}
}

func TestBuildPipeline_NestedBlockEdge(t *testing.T) {
	// `step.static.first.data.items` appears only inside the nested
	// `datasource {}` block. The dependency walk must still create the edge.
	src := []byte(`
step "static" "first" {
  value = "hello"
}

step "terraform_datasource" "second" {
  collector = collector.terraform.k8s

  datasource "kubernetes_resources" {
    items = step.static.first.data.items
  }
}

collector "terraform" "k8s" {
  provider = "hashicorp/kubernetes"
}
`)

	tmpl, diags := ParseJobTemplate(src, "nested.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	p, diags := BuildPipeline(zap.NewNop(), tmpl, testRegistry())
	require.False(t, diags.HasErrors(), "build diags: %s", diags.Error())

	order, err := p.Dag().TopologicalSort()
	require.NoError(t, err)

	keys := make([]string, 0, len(order))
	for _, n := range order {
		keys = append(keys, n.Key())
	}
	assert.Less(t,
		indexOf(keys, "step:static:first"),
		indexOf(keys, "step:terraform_datasource:second"),
		"nested-block reference must still produce a DAG edge")
}

func TestBuildPipeline_NestedBlockCycle(t *testing.T) {
	// Two steps refer to each other only from inside their nested blocks.
	// A non-recursive walk would miss both edges and fail to detect the cycle.
	src := []byte(`
step "terraform_datasource" "a" {
  collector = collector.terraform.k8s

  datasource "k" {
    v = step.terraform_datasource.b.data.v
  }
}

step "terraform_datasource" "b" {
  collector = collector.terraform.k8s

  datasource "k" {
    v = step.terraform_datasource.a.data.v
  }
}

collector "terraform" "k8s" {
  provider = "hashicorp/kubernetes"
}
`)

	tmpl, diags := ParseJobTemplate(src, "cycle.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	_, diags = BuildPipeline(zap.NewNop(), tmpl, testRegistry())
	require.True(t, diags.HasErrors(), "expected cycle diagnostic")
	assert.Contains(t, diags.Error(), "Cycle in collect job DAG")
}

func TestBuildPipeline_NestedEachOutsideForEach(t *testing.T) {
	// each.* appears only inside a nested block and the step has no for_each.
	// Build should reject it, not wait for execution.
	src := []byte(`
step "terraform_datasource" "bad" {
  collector = collector.terraform.k8s

  datasource "k" {
    v = each.value
  }
}

collector "terraform" "k8s" {
  provider = "hashicorp/kubernetes"
}
`)

	tmpl, diags := ParseJobTemplate(src, "each.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	_, diags = BuildPipeline(zap.NewNop(), tmpl, testRegistry())
	require.True(t, diags.HasErrors())
	assert.Contains(t, diags.Error(), "each.* used outside a for_each step")
}

func TestBuildPipeline_StepCollectorPolicy(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantMsg string
	}{
		{
			name: "required collector missing",
			src: `
step "terraform_datasource" "orphan" {
  datasource "k" {}
}`,
			wantMsg: `requires a collector`,
		},
		{
			name: "collector-less step declares collector",
			src: `
collector "terraform" "k8s" { provider = "hashicorp/kubernetes" }
step "static" "bad" {
  collector = collector.terraform.k8s
  value     = "hi"
}`,
			wantMsg: `must not declare a collector`,
		},
		{
			name: "incompatible collector kind",
			src: `
collector "http" "api" { base_url = "https://example.com" }
step "terraform_datasource" "s" {
  collector = collector.http.api
  datasource "k" {}
}`,
			wantMsg: `Incompatible collector for step "terraform_datasource"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, diags := ParseJobTemplate([]byte(tc.src), "policy.hcl")
			require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

			_, diags = BuildPipeline(zap.NewNop(), tmpl, testRegistry())
			require.True(t, diags.HasErrors())
			assert.Contains(t, diags.Error(), tc.wantMsg)
		})
	}
}

func TestBuildPipeline_CollectorBindingInvalid(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantMsg string
	}{
		{
			name: "conditional",
			src: `
collector "terraform" "a" { provider = "hashicorp/kubernetes" }
collector "terraform" "b" { provider = "hashicorp/kubernetes" }
step "terraform_datasource" "s" {
  collector = true ? collector.terraform.a : collector.terraform.b
  datasource "k" {}
}`,
			wantMsg: "Invalid collector binding",
		},
		{
			name: "function call",
			src: `
collector "terraform" "a" { provider = "hashicorp/kubernetes" }
step "terraform_datasource" "s" {
  collector = coalesce(collector.terraform.a)
  datasource "k" {}
}`,
			wantMsg: "Invalid collector binding",
		},
		{
			name: "string interpolation",
			src: `
collector "terraform" "a" { provider = "hashicorp/kubernetes" }
step "terraform_datasource" "s" {
  collector = "${collector.terraform.a}"
  datasource "k" {}
}`,
			wantMsg: "Invalid collector binding",
		},
		{
			name: "too few segments",
			src: `
step "terraform_datasource" "s" {
  collector = collector.terraform
  datasource "k" {}
}`,
			wantMsg: "Invalid collector binding",
		},
		{
			name: "wrong root",
			src: `
step "terraform_datasource" "s" {
  collector = step.terraform_datasource.x
  datasource "k" {}
}`,
			wantMsg: "Invalid collector binding",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, diags := ParseJobTemplate([]byte(tc.src), "binding.hcl")
			require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

			_, diags = BuildPipeline(zap.NewNop(), tmpl, testRegistry())
			require.True(t, diags.HasErrors())
			assert.Contains(t, diags.Error(), tc.wantMsg)
		})
	}
}

func TestBuildPipeline_CollectorAddressMismatch(t *testing.T) {
	// A collector "http" "api" is declared, but the step references
	// collector.terraform.api. With (Kind, Type, ID) identity, that is
	// simply an unknown collector address — not a silent wire-up.
	src := []byte(`
collector "http" "api" {
  base_url = "https://example.com"
}

step "terraform_datasource" "s" {
  collector = collector.terraform.api
  datasource "k" {}
}
`)

	tmpl, diags := ParseJobTemplate(src, "type-mismatch.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	_, diags = BuildPipeline(zap.NewNop(), tmpl, testRegistry())
	require.True(t, diags.HasErrors())
	assert.Contains(t, diags.Error(), "Reference to unknown collector")
}

func TestBuildPipeline_StepAddressMismatch(t *testing.T) {
	// A step "static" "first" is declared, but another step references
	// step.http_get.first.data. With (Kind, Type, ID) identity, that is
	// an unknown step address, not a silent match on id.
	src := []byte(`
step "static" "first" {
  value = "hello"
}

step "static" "second" {
  value = step.http_get.first.data
}
`)

	tmpl, diags := ParseJobTemplate(src, "step-type-mismatch.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	_, diags = BuildPipeline(zap.NewNop(), tmpl, testRegistry())
	require.True(t, diags.HasErrors())
	assert.Contains(t, diags.Error(), "Reference to unknown step")
}

func TestBuildPipeline_CollectorAddrStored(t *testing.T) {
	src := []byte(`
collector "terraform" "k8s" {
  provider = "hashicorp/kubernetes"
}

step "terraform_datasource" "s" {
  collector = collector.terraform.k8s
  datasource "k" {}
}
`)

	tmpl, diags := ParseJobTemplate(src, "addr.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	p, diags := BuildPipeline(zap.NewNop(), tmpl, testRegistry())
	require.False(t, diags.HasErrors(), "build diags: %s", diags.Error())

	meta, ok := p.Meta(Node{Kind: NodeTypeStep, Type: "terraform_datasource", ID: "s"})
	require.True(t, ok)
	require.NotNil(t, meta.CollectorAddr)
	assert.Equal(t, "terraform", meta.CollectorAddr.Type)
	assert.Equal(t, "k8s", meta.CollectorAddr.Name)
}

func TestBuildPipeline_StepToStepEdge(t *testing.T) {
	src := []byte(`
step "static" "first" {
  value = "hello"
}

step "static" "second" {
  value = step.static.first.data
}
`)

	tmpl, diags := ParseJobTemplate(src, "chain.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	p, diags := BuildPipeline(zap.NewNop(), tmpl, testRegistry())
	require.False(t, diags.HasErrors(), "build diags: %s", diags.Error())

	order, err := p.Dag().TopologicalSort()
	require.NoError(t, err)

	keys := make([]string, 0, len(order))
	for _, n := range order {
		keys = append(keys, n.Key())
	}
	assert.Less(t, indexOf(keys, "step:static:first"), indexOf(keys, "step:static:second"))
}

func TestBuildPipeline_SameIdDifferentTypes(t *testing.T) {
	// Two collectors share the id "api" but differ in type. Two steps
	// share the id "fetch" and also differ in type. Each step binds to a
	// distinct collector. The pipeline must treat the nodes as independent
	// and wire each reference to the matching (type, id) address.
	src := []byte(`
collector "terraform" "api" {
  provider = "hashicorp/kubernetes"
}

collector "http" "api" {
  base_url = "https://example.com"
}

step "terraform_datasource" "fetch" {
  collector = collector.terraform.api
  datasource "k" {}
}

step "http_get" "fetch" {
  collector = collector.http.api
  url       = "https://example.com/x"
}
`)

	tmpl, diags := ParseJobTemplate(src, "same-id.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	p, diags := BuildPipeline(zap.NewNop(), tmpl, testRegistry())
	require.False(t, diags.HasErrors(), "build diags: %s", diags.Error())

	tfColl := Node{Kind: NodeTypeCollector, Type: "terraform", ID: "api"}
	httpColl := Node{Kind: NodeTypeCollector, Type: "http", ID: "api"}
	tfStep := Node{Kind: NodeTypeStep, Type: "terraform_datasource", ID: "fetch"}
	httpStep := Node{Kind: NodeTypeStep, Type: "http_get", ID: "fetch"}

	for _, n := range []Node{tfColl, httpColl, tfStep, httpStep} {
		_, ok := p.Meta(n)
		assert.True(t, ok, "%s should be a distinct node", n.Key())
	}

	tfMeta, _ := p.Meta(tfStep)
	require.NotNil(t, tfMeta.CollectorAddr)
	assert.Equal(t, "terraform", tfMeta.CollectorAddr.Type)
	assert.Equal(t, "api", tfMeta.CollectorAddr.Name)

	httpMeta, _ := p.Meta(httpStep)
	require.NotNil(t, httpMeta.CollectorAddr)
	assert.Equal(t, "http", httpMeta.CollectorAddr.Type)
	assert.Equal(t, "api", httpMeta.CollectorAddr.Name)

	order, err := p.Dag().TopologicalSort()
	require.NoError(t, err)
	require.Len(t, order, 4)

	keys := make([]string, 0, len(order))
	for _, n := range order {
		keys = append(keys, n.Key())
	}
	assert.Less(t, indexOf(keys, tfColl.Key()), indexOf(keys, tfStep.Key()))
	assert.Less(t, indexOf(keys, httpColl.Key()), indexOf(keys, httpStep.Key()))
}
