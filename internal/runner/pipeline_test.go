package runner

import (
	"testing"

	"github.com/infracollect/infracollect/internal/engine"
	"github.com/infracollect/infracollect/internal/enginetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// mustBuildPipeline parses HCL and builds a pipeline, failing the test on error.
func mustBuildPipeline(t *testing.T, src []byte, filename string, reg *engine.Registry) *Pipeline {
	t.Helper()
	tmpl, diags := ParseJobTemplate(src, filename)
	require.False(t, diags.HasErrors(), "parse: %s", diags.Error())

	p, diags := BuildPipeline(zaptest.NewLogger(t), tmpl, reg)
	require.False(t, diags.HasErrors(), "build: %s", diags.Error())
	return p
}

// dagKeys returns the topologically sorted node keys from a pipeline.
func dagKeys(t *testing.T, p *Pipeline) []string {
	t.Helper()
	order, err := p.Dag().TopologicalSort()
	require.NoError(t, err)

	keys := make([]string, 0, len(order))
	for _, n := range order {
		keys = append(keys, n.Key())
	}
	return keys
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

	reg := enginetest.PipelineRegistry(t)
	p, diags := BuildPipeline(zaptest.NewLogger(t), tmpl, reg)
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

	keys := dagKeys(t, p)

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

	reg := enginetest.PipelineRegistry(t)
	p := mustBuildPipeline(t, src, "one.hcl", reg)

	keys := dagKeys(t, p)
	require.Len(t, keys, 1)
	assert.Equal(t, "step:static:only", keys[0])
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
	reg := enginetest.PipelineRegistry(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, diags := ParseJobTemplate([]byte(tc.src), "case.hcl")
			require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

			_, diags = BuildPipeline(zaptest.NewLogger(t), tmpl, reg)
			require.True(t, diags.HasErrors())
			assert.Contains(t, diags.Error(), tc.wantMsg)
		})
	}
}

func TestBuildPipeline_NestedBlockEdge(t *testing.T) {
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

	reg := enginetest.PipelineRegistry(t)
	p := mustBuildPipeline(t, src, "nested.hcl", reg)

	keys := dagKeys(t, p)
	assert.Less(t,
		indexOf(keys, "step:static:first"),
		indexOf(keys, "step:terraform_datasource:second"),
		"nested-block reference must still produce a DAG edge")
}

func TestBuildPipeline_NestedBlockCycle(t *testing.T) {
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

	reg := enginetest.PipelineRegistry(t)
	tmpl, diags := ParseJobTemplate(src, "cycle.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	_, diags = BuildPipeline(zaptest.NewLogger(t), tmpl, reg)
	require.True(t, diags.HasErrors(), "expected cycle diagnostic")
	assert.Contains(t, diags.Error(), "Cycle in collect job DAG")
}

func TestBuildPipeline_NestedEachOutsideForEach(t *testing.T) {
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

	reg := enginetest.PipelineRegistry(t)
	tmpl, diags := ParseJobTemplate(src, "each.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	_, diags = BuildPipeline(zaptest.NewLogger(t), tmpl, reg)
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
	reg := enginetest.PipelineRegistry(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, diags := ParseJobTemplate([]byte(tc.src), "policy.hcl")
			require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

			_, diags = BuildPipeline(zaptest.NewLogger(t), tmpl, reg)
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
	reg := enginetest.PipelineRegistry(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, diags := ParseJobTemplate([]byte(tc.src), "binding.hcl")
			require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

			_, diags = BuildPipeline(zaptest.NewLogger(t), tmpl, reg)
			require.True(t, diags.HasErrors())
			assert.Contains(t, diags.Error(), tc.wantMsg)
		})
	}
}

func TestBuildPipeline_CollectorAddressMismatch(t *testing.T) {
	src := []byte(`
collector "http" "api" {
  base_url = "https://example.com"
}

step "terraform_datasource" "s" {
  collector = collector.terraform.api
  datasource "k" {}
}
`)

	reg := enginetest.PipelineRegistry(t)
	tmpl, diags := ParseJobTemplate(src, "type-mismatch.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	_, diags = BuildPipeline(zaptest.NewLogger(t), tmpl, reg)
	require.True(t, diags.HasErrors())
	assert.Contains(t, diags.Error(), "Reference to unknown collector")
}

func TestBuildPipeline_StepAddressMismatch(t *testing.T) {
	src := []byte(`
step "static" "first" {
  value = "hello"
}

step "static" "second" {
  value = step.http_get.first.data
}
`)

	reg := enginetest.PipelineRegistry(t)
	tmpl, diags := ParseJobTemplate(src, "step-type-mismatch.hcl")
	require.False(t, diags.HasErrors(), "parse diags: %s", diags.Error())

	_, diags = BuildPipeline(zaptest.NewLogger(t), tmpl, reg)
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

	reg := enginetest.PipelineRegistry(t)
	p := mustBuildPipeline(t, src, "addr.hcl", reg)

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

	reg := enginetest.PipelineRegistry(t)
	p := mustBuildPipeline(t, src, "chain.hcl", reg)

	keys := dagKeys(t, p)
	assert.Less(t, indexOf(keys, "step:static:first"), indexOf(keys, "step:static:second"))
}

func TestBuildPipeline_SameIdDifferentTypes(t *testing.T) {
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

	reg := enginetest.PipelineRegistry(t)
	p := mustBuildPipeline(t, src, "same-id.hcl", reg)

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

	keys := dagKeys(t, p)
	require.Len(t, keys, 4)

	assert.Less(t, indexOf(keys, tfColl.Key()), indexOf(keys, tfStep.Key()))
	assert.Less(t, indexOf(keys, httpColl.Key()), indexOf(keys, httpStep.Key()))
}
