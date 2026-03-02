package runner

import (
	"fmt"
	"sort"
)

type NodeType int

const (
	NodeTypeCollector NodeType = iota
	NodeTypeStep
	// NodeTypeCollection is a step that declared a for_each attribute. It
	// is not executed once like NodeTypeStep; at execution time the runner
	// evaluates the for_each expression to a cty collection, fans the step
	// body out per iteration, and aggregates per-key results.
	NodeTypeCollection
)

func (t NodeType) String() string {
	switch t {
	case NodeTypeCollector:
		return "collector"
	case NodeTypeStep:
		return "step"
	case NodeTypeCollection:
		return "collection"
	}
	return ""
}

// Node identifies a pipeline node by (Kind, Type, ID). Type is the first HCL
// label (e.g. "terraform", "http_get") and ID is the second (user-chosen).
type Node struct {
	Kind NodeType
	Type string
	ID   string
}

func (n Node) Key() string {
	return fmt.Sprintf("%s:%s:%s", n.Kind.String(), n.Type, n.ID)
}

func (n Node) String() string {
	return fmt.Sprintf("Node<%v, %v, %v>", n.Kind.String(), n.Type, n.ID)
}

type DirectedAcyclicGraph struct {
	nodes map[string]Node
	edges map[string][]string
}

func (g *DirectedAcyclicGraph) requireNode(node Node) error {
	if _, ok := g.nodes[node.Key()]; !ok {
		return fmt.Errorf("node %v not found", node.Key())
	}
	return nil
}

func insertSorted(values []string, value string) []string {
	idx := sort.SearchStrings(values, value)
	values = append(values, "")
	copy(values[idx+1:], values[idx:])
	values[idx] = value
	return values
}

func NewDirectedAcyclicGraph() *DirectedAcyclicGraph {
	return &DirectedAcyclicGraph{
		nodes: make(map[string]Node),
		edges: make(map[string][]string),
	}
}

func (g *DirectedAcyclicGraph) AddNode(node Node) error {
	if _, ok := g.nodes[node.Key()]; ok {
		return fmt.Errorf("node %v already exists", node.Key())
	}

	g.nodes[node.Key()] = node
	return nil
}

func (g *DirectedAcyclicGraph) AddEdge(from, to Node) error {
	if err := g.requireNode(from); err != nil {
		return err
	}

	if err := g.requireNode(to); err != nil {
		return err
	}

	wouldCycle, err := g.WouldCreateCycle(from, to)
	if err != nil {
		return err
	}

	if wouldCycle {
		return fmt.Errorf("adding edge %q -> %q would create a cycle", from, to)
	}

	g.edges[from.Key()] = append(g.edges[from.Key()], to.Key())
	return nil
}

// AddEdgeUnchecked appends an edge without the O(V+E) cycle check that
// AddEdge performs per call. Callers must run TopologicalSort after bulk
// inserts to catch any cycles — kahnSort surfaces them as a single error.
// Used by pipeline construction where every reference becomes an edge and
// the per-edge check would be quadratic in a dense graph.
func (g *DirectedAcyclicGraph) AddEdgeUnchecked(from, to Node) error {
	if err := g.requireNode(from); err != nil {
		return err
	}
	if err := g.requireNode(to); err != nil {
		return err
	}
	g.edges[from.Key()] = append(g.edges[from.Key()], to.Key())
	return nil
}

// WouldCreateCycle reports whether adding the edge would create a cycle.
func (g *DirectedAcyclicGraph) WouldCreateCycle(from, to Node) (bool, error) {
	if err := g.requireNode(from); err != nil {
		return false, err
	}

	if err := g.requireNode(to); err != nil {
		return false, err
	}

	if from == to {
		return true, nil
	}

	found, err := g.canReach(to, from)
	if err != nil {
		return false, err
	}

	return found, nil
}

func (g *DirectedAcyclicGraph) TopologicalSort() ([]Node, error) {
	return g.kahnSort()
}

func (g *DirectedAcyclicGraph) kahnSort() ([]Node, error) {
	inDegree := make(map[string]int, len(g.nodes))
	for key := range g.nodes {
		inDegree[key] = 0
	}

	for from, neighbors := range g.edges {
		if _, ok := g.nodes[from]; !ok {
			return nil, fmt.Errorf("edge references unknown source node %q", from)
		}

		for _, to := range neighbors {
			if _, ok := g.nodes[to]; !ok {
				return nil, fmt.Errorf("edge references unknown destination node %q", to)
			}
			inDegree[to]++
		}
	}

	ready := make([]string, 0, len(g.nodes))
	for key, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, key)
		}
	}
	sort.Strings(ready)

	order := make([]Node, 0, len(g.nodes))
	for len(ready) > 0 {
		current := ready[0]
		ready = ready[1:]
		order = append(order, g.nodes[current])

		neighbors := append([]string(nil), g.edges[current]...)
		sort.Strings(neighbors)
		for _, to := range neighbors {
			inDegree[to]--
			if inDegree[to] == 0 {
				ready = insertSorted(ready, to)
			}
		}
	}

	if len(order) != len(g.nodes) {
		return nil, fmt.Errorf("cycle detected during topological sort (nodes=%d, sorted=%d)", len(g.nodes), len(order))
	}

	return order, nil
}

func (g *DirectedAcyclicGraph) canReach(from, to Node) (bool, error) {
	queue := []string{from.Key()}
	visited := map[string]bool{from.Key(): true}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == to.Key() {
			break
		}

		for _, next := range g.edges[current] {
			if _, ok := g.nodes[next]; !ok {
				return false, fmt.Errorf("edge references unknown destination node %q", next)
			}
			if visited[next] {
				continue
			}
			visited[next] = true
			queue = append(queue, next)
		}
	}

	return visited[to.Key()], nil
}
