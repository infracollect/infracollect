package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stepNode(id string) Node {
	return Node{Kind: NodeTypeStep, Type: "t", ID: id}
}

func collectorNode(id string) Node {
	return Node{Kind: NodeTypeCollector, Type: "t", ID: id}
}

func TestDirectedAcyclicGraph_TopologicalSort(t *testing.T) {
	g := NewDirectedAcyclicGraph()

	require.NoError(t, g.AddNode(collectorNode("c1")))
	require.NoError(t, g.AddNode(collectorNode("c2")))
	require.NoError(t, g.AddNode(stepNode("s1")))
	require.NoError(t, g.AddNode(stepNode("s2")))

	require.NoError(t, g.AddEdge(collectorNode("c1"), stepNode("s1")))
	require.NoError(t, g.AddEdge(collectorNode("c2"), stepNode("s1")))
	require.NoError(t, g.AddEdge(stepNode("s1"), stepNode("s2")))

	order, err := g.TopologicalSort()
	require.NoError(t, err)
	require.Len(t, order, 4)

	keys := make([]string, 0, len(order))
	for _, node := range order {
		keys = append(keys, node.Key())
	}

	assert.True(t, indexOf(keys, "collector:t:c1") < indexOf(keys, "step:t:s1"))
	assert.True(t, indexOf(keys, "collector:t:c2") < indexOf(keys, "step:t:s1"))
	assert.True(t, indexOf(keys, "step:t:s1") < indexOf(keys, "step:t:s2"))
}

func TestDirectedAcyclicGraph_AddEdge_RejectsCycle(t *testing.T) {
	g := NewDirectedAcyclicGraph()

	require.NoError(t, g.AddNode(stepNode("a")))
	require.NoError(t, g.AddNode(stepNode("b")))

	require.NoError(t, g.AddEdge(stepNode("a"), stepNode("b")))
	err := g.AddEdge(stepNode("b"), stepNode("a"))
	require.Error(t, err)
	assert.EqualError(t, err, `adding edge "Node<step, t, b>" -> "Node<step, t, a>" would create a cycle`)
}

func TestDirectedAcyclicGraph_WouldCreateCycle(t *testing.T) {
	g := NewDirectedAcyclicGraph()

	require.NoError(t, g.AddNode(stepNode("a")))
	require.NoError(t, g.AddNode(stepNode("b")))
	require.NoError(t, g.AddNode(stepNode("c")))

	require.NoError(t, g.AddEdge(stepNode("a"), stepNode("b")))
	require.NoError(t, g.AddEdge(stepNode("b"), stepNode("c")))

	wouldCycle, err := g.WouldCreateCycle(stepNode("c"), stepNode("a"))
	require.NoError(t, err)
	assert.True(t, wouldCycle)

	wouldCycle, err = g.WouldCreateCycle(stepNode("a"), stepNode("c"))
	require.NoError(t, err)
	assert.False(t, wouldCycle)

	wouldCycle, err = g.WouldCreateCycle(stepNode("a"), stepNode("a"))
	require.NoError(t, err)
	assert.True(t, wouldCycle)
}

func TestDirectedAcyclicGraph_TopologicalSort_ReturnsErrorOnInvariantViolation(t *testing.T) {
	g := NewDirectedAcyclicGraph()

	require.NoError(t, g.AddNode(stepNode("a")))
	require.NoError(t, g.AddNode(stepNode("b")))

	// Create a cycle directly to exercise cycle detection independently of AddEdge guard.
	g.edges["step:t:a"] = []string{"step:t:b"}
	g.edges["step:t:b"] = []string{"step:t:a"}

	_, err := g.TopologicalSort()
	require.Error(t, err)
	assert.EqualError(t, err, "cycle detected during topological sort (nodes=2, sorted=0)")
}

func TestDirectedAcyclicGraph_TopologicalSort_AfterMutation(t *testing.T) {
	g := NewDirectedAcyclicGraph()

	require.NoError(t, g.AddNode(stepNode("a")))
	require.NoError(t, g.AddNode(stepNode("b")))
	require.NoError(t, g.AddEdge(stepNode("a"), stepNode("b")))

	first, err := g.TopologicalSort()
	require.NoError(t, err)
	require.Len(t, first, 2)

	require.NoError(t, g.AddNode(stepNode("c")))
	require.NoError(t, g.AddEdge(stepNode("b"), stepNode("c")))

	second, err := g.TopologicalSort()
	require.NoError(t, err)
	require.Len(t, second, 3)

	keys := make([]string, 0, len(second))
	for _, node := range second {
		keys = append(keys, node.Key())
	}
	assert.Less(t, indexOf(keys, "step:t:a"), indexOf(keys, "step:t:b"))
	assert.Less(t, indexOf(keys, "step:t:b"), indexOf(keys, "step:t:c"))
}

func TestDirectedAcyclicGraph_AddEdgeUnchecked_SkipsCycleGuard(t *testing.T) {
	g := NewDirectedAcyclicGraph()

	require.NoError(t, g.AddNode(stepNode("a")))
	require.NoError(t, g.AddNode(stepNode("b")))

	require.NoError(t, g.AddEdgeUnchecked(stepNode("a"), stepNode("b")))
	require.NoError(t, g.AddEdgeUnchecked(stepNode("b"), stepNode("a")))

	_, err := g.TopologicalSort()
	require.Error(t, err)
	assert.ErrorContains(t, err, "cycle detected")
}

func TestDirectedAcyclicGraph_AddEdgeUnchecked_UnknownNode(t *testing.T) {
	g := NewDirectedAcyclicGraph()
	require.NoError(t, g.AddNode(stepNode("a")))

	err := g.AddEdgeUnchecked(stepNode("a"), stepNode("missing"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "not found")
}

func indexOf(values []string, target string) int {
	for idx, value := range values {
		if value == target {
			return idx
		}
	}
	return -1
}
