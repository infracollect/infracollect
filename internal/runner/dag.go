package runner

import (
	"fmt"
	"sort"
)

type NodeType int

const (
	NodeTypeCollector NodeType = iota
	NodeTypeStep
)

func (t NodeType) String() string {
	switch t {
	case NodeTypeCollector:
		return "collector"
	case NodeTypeStep:
		return "step"
	}
	return ""
}

type Node struct {
	Kind NodeType
	ID   string
}

func (n Node) String() string {
	return fmt.Sprintf("Node<%v, %v>", n.Kind.String(), n.ID)
}

type DirectedAcyclicGraph struct {
	nodes map[string]*Node

	// cached topological sort
	order []*Node
}

func NewDirectedAcyclicGraph() *DirectedAcyclicGraph {
	return &DirectedAcyclicGraph{
		nodes: make(map[string]*Node),
	}
}

func (g *DirectedAcyclicGraph) AddNode(kind NodeType, id string) error {
	if _, ok := g.nodes[fmt.Sprintf("%s:%s", kind.String(), id)]; ok {
		return fmt.Errorf("node %v already exists", id)
	}
	g.nodes[fmt.Sprintf("%s:%s", kind.String(), id)] = &Node{Kind: kind, ID: id}
	g.order = nil // invalidate cached topological sort
	return nil
}

func (g *DirectedAcyclicGraph) TopologicalSort() ([]*Node, error) {
	if g.order != nil {
		return g.order, nil
	}

	nodes := make([]*Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	g.order = nodes
	return g.order, nil
}
