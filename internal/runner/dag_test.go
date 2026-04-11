package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectedAcyclicGraph_TopologicalSort(t *testing.T) {
	g := NewDirectedAcyclicGraph()

	require.NoError(t, g.AddNode(Node{Kind: NodeTypeCollector, ID: "c1"}))
	require.NoError(t, g.AddNode(Node{Kind: NodeTypeCollector, ID: "c2"}))
	require.NoError(t, g.AddNode(Node{Kind: NodeTypeStep, ID: "s1"}))
	require.NoError(t, g.AddNode(Node{Kind: NodeTypeStep, ID: "s2"}))

	require.NoError(t, g.AddEdge(Node{Kind: NodeTypeCollector, ID: "c1"}, Node{Kind: NodeTypeStep, ID: "s1"}))
	require.NoError(t, g.AddEdge(Node{Kind: NodeTypeCollector, ID: "c2"}, Node{Kind: NodeTypeStep, ID: "s1"}))
	require.NoError(t, g.AddEdge(Node{Kind: NodeTypeStep, ID: "s1"}, Node{Kind: NodeTypeStep, ID: "s2"}))

	order, err := g.TopologicalSort()
	require.NoError(t, err)
	require.Len(t, order, 4)

	keys := make([]string, 0, len(order))
	for _, node := range order {
		keys = append(keys, node.Key())
	}

	assert.True(t, indexOf(keys, "collector:c1") < indexOf(keys, "step:s1"))
	assert.True(t, indexOf(keys, "collector:c2") < indexOf(keys, "step:s1"))
	assert.True(t, indexOf(keys, "step:s1") < indexOf(keys, "step:s2"))
}

func TestDirectedAcyclicGraph_AddEdge_RejectsCycle(t *testing.T) {
	g := NewDirectedAcyclicGraph()

	require.NoError(t, g.AddNode(Node{Kind: NodeTypeStep, ID: "a"}))
	require.NoError(t, g.AddNode(Node{Kind: NodeTypeStep, ID: "b"}))

	require.NoError(t, g.AddEdge(Node{Kind: NodeTypeStep, ID: "a"}, Node{Kind: NodeTypeStep, ID: "b"}))
	err := g.AddEdge(Node{Kind: NodeTypeStep, ID: "b"}, Node{Kind: NodeTypeStep, ID: "a"})
	require.Error(t, err)
	assert.EqualError(t, err, `adding edge "Node<step, b>" -> "Node<step, a>" would create a cycle`)
}

func TestDirectedAcyclicGraph_WouldCreateCycle(t *testing.T) {
	g := NewDirectedAcyclicGraph()

	require.NoError(t, g.AddNode(Node{Kind: NodeTypeStep, ID: "a"}))
	require.NoError(t, g.AddNode(Node{Kind: NodeTypeStep, ID: "b"}))
	require.NoError(t, g.AddNode(Node{Kind: NodeTypeStep, ID: "c"}))

	require.NoError(t, g.AddEdge(Node{Kind: NodeTypeStep, ID: "a"}, Node{Kind: NodeTypeStep, ID: "b"}))
	require.NoError(t, g.AddEdge(Node{Kind: NodeTypeStep, ID: "b"}, Node{Kind: NodeTypeStep, ID: "c"}))

	wouldCycle, err := g.WouldCreateCycle(Node{Kind: NodeTypeStep, ID: "c"}, Node{Kind: NodeTypeStep, ID: "a"})
	require.NoError(t, err)
	assert.True(t, wouldCycle)

	wouldCycle, err = g.WouldCreateCycle(Node{Kind: NodeTypeStep, ID: "a"}, Node{Kind: NodeTypeStep, ID: "c"})
	require.NoError(t, err)
	assert.False(t, wouldCycle)

	wouldCycle, err = g.WouldCreateCycle(Node{Kind: NodeTypeStep, ID: "a"}, Node{Kind: NodeTypeStep, ID: "a"})
	require.NoError(t, err)
	assert.True(t, wouldCycle)
}

func TestDirectedAcyclicGraph_TopologicalSort_ReturnsErrorOnInvariantViolation(t *testing.T) {
	g := NewDirectedAcyclicGraph()

	require.NoError(t, g.AddNode(Node{Kind: NodeTypeStep, ID: "a"}))
	require.NoError(t, g.AddNode(Node{Kind: NodeTypeStep, ID: "b"}))

	// Create a cycle directly to exercise cycle detection independently of AddEdge guard.
	g.edges["step:a"] = []string{"step:b"}
	g.edges["step:b"] = []string{"step:a"}

	_, err := g.TopologicalSort()
	require.Error(t, err)
	assert.EqualError(t, err, "cycle detected during topological sort (nodes=2, sorted=0)")
}

func TestDirectedAcyclicGraph_TopologicalSort_CacheInvalidation(t *testing.T) {
	g := NewDirectedAcyclicGraph()

	require.NoError(t, g.AddNode(Node{Kind: NodeTypeStep, ID: "a"}))
	require.NoError(t, g.AddNode(Node{Kind: NodeTypeStep, ID: "b"}))
	require.NoError(t, g.AddEdge(Node{Kind: NodeTypeStep, ID: "a"}, Node{Kind: NodeTypeStep, ID: "b"}))

	first, err := g.TopologicalSort()
	require.NoError(t, err)
	require.Len(t, first, 2)

	require.NoError(t, g.AddNode(Node{Kind: NodeTypeStep, ID: "c"}))
	require.NoError(t, g.AddEdge(Node{Kind: NodeTypeStep, ID: "b"}, Node{Kind: NodeTypeStep, ID: "c"}))

	second, err := g.TopologicalSort()
	require.NoError(t, err)
	require.Len(t, second, 3)

	keys := make([]string, 0, len(second))
	for _, node := range second {
		keys = append(keys, node.Key())
	}
	assert.True(t, indexOf(keys, "step:a") < indexOf(keys, "step:b"))
	assert.True(t, indexOf(keys, "step:b") < indexOf(keys, "step:c"))
}

func indexOf(values []string, target string) int {
	for idx, value := range values {
		if value == target {
			return idx
		}
	}
	return -1
}
