package core

import "errors"

type Edge struct {
	Source NodeID   `json:"src"`
	SrcOut PortName `json:"srcPort"`
	Target NodeID   `json:"dst"`
	TgtIn  PortName `json:"dstPort"`
	Delay  int      `json:"delay"`
}

type Grid struct {
	nodes map[NodeID]Node
	edges []Edge
}

func NewGrid() *Grid { return &Grid{nodes: map[NodeID]Node{}} }

func (g *Grid) Add(n Node)          { g.nodes[n.ID()] = n }
func (g *Grid) Nodes() map[NodeID]Node { return g.nodes }
func (g *Grid) Edges() []Edge          { return g.edges }

func (g *Grid) Connect(src NodeID, sp PortName, dst NodeID, dp PortName, d int) error {
	if _, ok := g.nodes[src]; !ok {
		return errors.New("src missing")
	}
	if _, ok := g.nodes[dst]; !ok {
		return errors.New("dst missing")
	}
	g.edges = append(g.edges, Edge{src, sp, dst, dp, d})
	return nil
}

