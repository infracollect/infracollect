package runner

import (
	"testing"

	"github.com/infracollect/infracollect/internal/enginetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDryRun_PlainStep(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)
	src := []byte(`
step "stub_nocoll" "only" {
  greeting = "hello"
}
`)
	r := newRunner(t, src, "plain.hcl", stub.Reg)
	plan, err := r.DryRun()
	require.NoError(t, err)

	assert.Equal(t, "plain", plan.Job)
	require.Len(t, plan.Nodes, 1)
	assert.Equal(t, "step", plan.Nodes[0].Kind)
	assert.Equal(t, "stub_nocoll", plan.Nodes[0].Type)
	assert.Equal(t, "only", plan.Nodes[0].ID)
	assert.Empty(t, plan.Nodes[0].DependsOn)
	assert.Empty(t, plan.Nodes[0].Collector)
	assert.False(t, plan.Nodes[0].ForEach)
}

func TestDryRun_CollectorBinding(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)
	src := []byte(`
collector "stub" "c" {
}

step "stub_step" "s" {
  collector = collector.stub.c
  label     = "bound"
}
`)
	r := newRunner(t, src, "bind.hcl", stub.Reg)
	plan, err := r.DryRun()
	require.NoError(t, err)

	require.Len(t, plan.Nodes, 2)

	// First node should be the collector (topological order).
	assert.Equal(t, "collector", plan.Nodes[0].Kind)
	assert.Equal(t, "stub", plan.Nodes[0].Type)
	assert.Equal(t, "c", plan.Nodes[0].ID)

	// Second node should be the step bound to the collector.
	assert.Equal(t, "step", plan.Nodes[1].Kind)
	assert.Equal(t, "stub/c", plan.Nodes[1].Collector)
	assert.Contains(t, plan.Nodes[1].DependsOn, "stub/c")
}

func TestDryRun_Collection(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)
	src := []byte(`
step "stub_nocoll" "items" {
  for_each = { a = "1", b = "2" }
  val      = each.value
}
`)
	r := newRunner(t, src, "collection.hcl", stub.Reg)
	plan, err := r.DryRun()
	require.NoError(t, err)

	require.Len(t, plan.Nodes, 1)
	assert.Equal(t, "collection", plan.Nodes[0].Kind)
	assert.True(t, plan.Nodes[0].ForEach)
	assert.True(t, plan.Nodes[0].ForEachResolved, "literal for_each should resolve at plan time")
	assert.Equal(t, []string{"a", "b"}, plan.Nodes[0].ForEachKeys)
}

func TestDryRun_ForEachComputedAtRuntime(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)
	src := []byte(`
step "stub_nocoll" "source" {
  value = "1"
}

step "stub_nocoll" "items" {
  for_each = step.stub_nocoll.source.data
  val      = each.value
}
`)
	r := newRunner(t, src, "runtime-foreach.hcl", stub.Reg)
	plan, err := r.DryRun()
	require.NoError(t, err)

	byID := make(map[string]PlanNode)
	for _, n := range plan.Nodes {
		byID[n.ID] = n
	}

	items := byID["items"]
	assert.Equal(t, "collection", items.Kind)
	assert.True(t, items.ForEach)
	assert.False(t, items.ForEachResolved, "for_each over upstream step data is only known at runtime")
	assert.Empty(t, items.ForEachKeys)
}

func TestDryRun_MultiStepDependencies(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)
	src := []byte(`
step "stub_nocoll" "first" {
  value = "a"
}

step "stub_nocoll" "second" {
  value = step.stub_nocoll.first.data
}

step "stub_nocoll" "third" {
  value = step.stub_nocoll.second.data
}
`)
	r := newRunner(t, src, "chain.hcl", stub.Reg)
	plan, err := r.DryRun()
	require.NoError(t, err)

	require.Len(t, plan.Nodes, 3)

	// Verify topological order.
	ids := make([]string, len(plan.Nodes))
	for i, n := range plan.Nodes {
		ids[i] = n.ID
	}
	assert.Equal(t, []string{"first", "second", "third"}, ids)

	// Verify dependencies.
	assert.Empty(t, plan.Nodes[0].DependsOn)
	assert.Equal(t, []string{"stub_nocoll/first"}, plan.Nodes[1].DependsOn)
	assert.Equal(t, []string{"stub_nocoll/second"}, plan.Nodes[2].DependsOn)
}

func TestDryRun_OutputDefaults(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)
	src := []byte(`
step "stub_nocoll" "s" {
  v = "1"
}
`)
	r := newRunner(t, src, "defaults.hcl", stub.Reg)
	plan, err := r.DryRun()
	require.NoError(t, err)

	assert.Equal(t, "json", plan.Output.Encoding)
	assert.Equal(t, "stdout", plan.Output.Sink)
	assert.Empty(t, plan.Output.Archive)
	assert.Nil(t, plan.Output.Steps)
}

func TestDryRun_OutputConfig(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)
	src := []byte(`
step "stub_nocoll" "a" {
  v = "1"
}

step "stub_nocoll" "b" {
  v = "2"
}

output {
  encoding "json" {}
  sink "filesystem" {
    path = "/tmp/out"
  }
  steps = [step.stub_nocoll.a]
}
`)
	r := newRunner(t, src, "output.hcl", stub.Reg)
	plan, err := r.DryRun()
	require.NoError(t, err)

	assert.Equal(t, "json", plan.Output.Encoding)
	assert.Equal(t, "filesystem", plan.Output.Sink)
	assert.Equal(t, []string{"stub_nocoll/a"}, plan.Output.Steps)
}

func TestDryRun_ComplexGraph(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)
	src := []byte(`
job {
  name = "complex-demo"
}

collector "stub" "main" {
}

step "stub_nocoll" "regions" {
  value = "us-east-1"
}

step "stub_step" "fetch" {
  collector = collector.stub.main
  region    = step.stub_nocoll.regions.data
}

step "stub_nocoll" "items" {
  for_each = { a = "1", b = "2", c = "3" }
  val      = each.value
  upstream = step.stub_step.fetch.data
}

step "stub_nocoll" "summary" {
  data = step.stub_nocoll.items.data
}

output {
  encoding "json" {}
  sink "filesystem" {
    path = "/tmp/out"
  }
  archive "tar" {}
  steps = [step.stub_step.fetch, step.stub_nocoll.summary]
}
`)
	r := newRunner(t, src, "complex.hcl", stub.Reg)
	plan, err := r.DryRun()
	require.NoError(t, err)

	assert.Equal(t, "complex-demo", plan.Job)
	require.Len(t, plan.Nodes, 5)

	// Build a lookup for easier assertions.
	byID := make(map[string]PlanNode)
	for _, n := range plan.Nodes {
		byID[n.ID] = n
	}

	// Collector node.
	assert.Equal(t, "collector", byID["main"].Kind)

	// regions has no dependencies.
	assert.Equal(t, "step", byID["regions"].Kind)
	assert.Empty(t, byID["regions"].DependsOn)

	// fetch depends on collector and regions.
	assert.Equal(t, "step", byID["fetch"].Kind)
	assert.Equal(t, "stub/main", byID["fetch"].Collector)
	assert.Contains(t, byID["fetch"].DependsOn, "stub/main")
	assert.Contains(t, byID["fetch"].DependsOn, "stub_nocoll/regions")

	// items is a for_each collection depending on fetch.
	assert.Equal(t, "collection", byID["items"].Kind)
	assert.True(t, byID["items"].ForEach)
	assert.Contains(t, byID["items"].DependsOn, "stub_step/fetch")

	// summary depends on items.
	assert.Equal(t, "step", byID["summary"].Kind)
	assert.Contains(t, byID["summary"].DependsOn, "stub_nocoll/items")

	// Topological order: collector and regions before fetch, fetch before items, items before summary.
	idxOf := func(id string) int {
		for i, n := range plan.Nodes {
			if n.ID == id {
				return i
			}
		}
		return -1
	}
	assert.Less(t, idxOf("main"), idxOf("fetch"))
	assert.Less(t, idxOf("regions"), idxOf("fetch"))
	assert.Less(t, idxOf("fetch"), idxOf("items"))
	assert.Less(t, idxOf("items"), idxOf("summary"))

	// Output configuration.
	assert.Equal(t, "json", plan.Output.Encoding)
	assert.Equal(t, "filesystem", plan.Output.Sink)
	assert.Equal(t, "tar", plan.Output.Archive)
	assert.Equal(t, []string{"stub_nocoll/summary", "stub_step/fetch"}, plan.Output.Steps)
}

func TestDryRun_DoesNotExecute(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)
	src := []byte(`
collector "stub" "c" {
}

step "stub_step" "s" {
  collector = collector.stub.c
  label     = "test"
}
`)
	r := newRunner(t, src, "noexec.hcl", stub.Reg)
	_, err := r.DryRun()
	require.NoError(t, err)

	// Collectors should not have been started.
	assert.Empty(t, stub.Collectors, "DryRun must not instantiate collectors")
}
