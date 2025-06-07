package model

import "testing"

func TestAddNodeTogglesRow(t *testing.T) {
	g := NewGraph()
	g.AddNode(2, 0)
	if !g.Row[2] {
		t.Fatal("step 2 should be ON after adding node at (2,0)")
	}
}

func TestToggleStepAddsOrRemovesNode(t *testing.T) {
	g := NewGraph()
	g.ToggleStep(1)
	if _, ok := findNode(g, 1, 0); !ok {
		t.Fatal("expected node at (1,0)")
	}
	g.ToggleStep(1)
	if _, ok := findNode(g, 1, 0); ok {
		t.Fatal("node should be gone after toggling off")
	}
}

func findNode(g *Graph, i, j int) (NodeID, bool) {
	for id, n := range g.Nodes {
		if n.I == i && n.J == j {
			return id, true
		}
	}
	return 0, false
}
