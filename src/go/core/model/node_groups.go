package model

import (
	"errors"
	"fmt"
	"sort"
)

// GroupID identifies a node group within a Graph.
type GroupID int

// GroupParam names the node parameter a GroupRule mutates.
type GroupParam string

const (
	GroupParamPitch    GroupParam = "pitch"
	GroupParamVolume   GroupParam = "volume"
	GroupParamDuration GroupParam = "duration"
)

// GroupRule: every EveryN rounds, accumulate Delta on Param. The accumulated
// effect (Delta × steps) clamps to [Min, Max]. Zero Min+Max on input means
// "use per-param defaults" (set at store time by SetGroupRules).
type GroupRule struct {
	Param  GroupParam
	Delta  float64
	EveryN int
	Min    float64
	Max    float64
}

// NodeGroup is a persistent set of nodes sharing batch edits and rules.
// A node may belong to several groups; rule effects stack (pitch deltas sum,
// volume/duration multipliers multiply).
type NodeGroup struct {
	ID      GroupID
	Name    string
	NodeIDs []NodeID
	Rules   []GroupRule
}

var (
	ErrGroupNotFound    = errors.New("group not found")
	ErrGroupEmptyNodes  = errors.New("group needs at least one node")
	ErrGroupUnknownNode = errors.New("group references unknown node")
	ErrGroupBadRule     = errors.New("invalid group rule")
)

// groupRuleDefaultClamp returns the default accumulated-effect clamp per param.
func groupRuleDefaultClamp(p GroupParam) (float64, float64) {
	switch p {
	case GroupParamPitch:
		return -24, 24
	case GroupParamVolume:
		return 0, 4
	default: // duration
		return 0.1, 4
	}
}

func (g *Graph) fireGroupChanged(id GroupID) {
	if g.onGroupChanged != nil {
		g.onGroupChanged(id)
	}
}

// SetGroupChangedHook registers a callback invoked after every group mutation
// (create, delete, rename, membership, rules). Sibling of SetNodeChangedHook.
func (g *Graph) SetGroupChangedHook(fn func(GroupID)) { g.onGroupChanged = fn }

// dedupValidateNodeIDs returns nodeIDs deduped in first-seen order, erroring
// on empty sets and IDs not present in the graph.
func (g *Graph) dedupValidateNodeIDs(nodeIDs []NodeID) ([]NodeID, error) {
	seen := map[NodeID]bool{}
	out := make([]NodeID, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		if seen[id] {
			continue
		}
		if _, ok := g.Nodes[id]; !ok {
			return nil, fmt.Errorf("%w: %d", ErrGroupUnknownNode, id)
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, ErrGroupEmptyNodes
	}
	return out, nil
}

// ResetGroups clears every group AND resets the GroupID counter, unlike
// assigning an empty map to Groups directly. This must be used (not a bare
// `g.Groups = map[...]{}`) anywhere groups are wiped as part of a full
// document replace (e.g. Import's replace-not-merge reset) — otherwise
// nextGroupID keeps climbing across repeated import/undo cycles, so
// re-importing the SAME document twice mints different GroupIDs each time
// and the exported bytes are no longer deterministic/byte-stable (breaks
// undo/redo's byte-for-byte round-trip, which reimports snapshots).
func (g *Graph) ResetGroups() {
	g.Groups = map[GroupID]*NodeGroup{}
	g.nextGroupID = 0
}

// CreateGroup creates a group over nodeIDs (deduped). Empty name gets
// "Group N". Returns the new GroupID.
func (g *Graph) CreateGroup(name string, nodeIDs []NodeID) (GroupID, error) {
	ids, err := g.dedupValidateNodeIDs(nodeIDs)
	if err != nil {
		return 0, err
	}
	if g.Groups == nil {
		g.Groups = map[GroupID]*NodeGroup{}
	}
	g.nextGroupID++
	id := g.nextGroupID
	if name == "" {
		name = fmt.Sprintf("Group %d", id)
	}
	g.Groups[id] = &NodeGroup{ID: id, Name: name, NodeIDs: ids}
	g.fireGroupChanged(id)
	return id, nil
}

func (g *Graph) DeleteGroup(id GroupID) error {
	if _, ok := g.Groups[id]; !ok {
		return ErrGroupNotFound
	}
	delete(g.Groups, id)
	g.fireGroupChanged(id)
	return nil
}

func (g *Graph) RenameGroup(id GroupID, name string) error {
	grp, ok := g.Groups[id]
	if !ok {
		return ErrGroupNotFound
	}
	grp.Name = name
	g.fireGroupChanged(id)
	return nil
}

// SetGroupNodes replaces membership. Empty set is an error (use DeleteGroup).
func (g *Graph) SetGroupNodes(id GroupID, nodeIDs []NodeID) error {
	grp, ok := g.Groups[id]
	if !ok {
		return ErrGroupNotFound
	}
	ids, err := g.dedupValidateNodeIDs(nodeIDs)
	if err != nil {
		return err
	}
	grp.NodeIDs = ids
	g.fireGroupChanged(id)
	return nil
}

// AddNodeToGroup is idempotent for existing members.
func (g *Graph) AddNodeToGroup(id GroupID, n NodeID) error {
	grp, ok := g.Groups[id]
	if !ok {
		return ErrGroupNotFound
	}
	if _, nok := g.Nodes[n]; !nok {
		return fmt.Errorf("%w: %d", ErrGroupUnknownNode, n)
	}
	for _, m := range grp.NodeIDs {
		if m == n {
			return nil
		}
	}
	grp.NodeIDs = append(grp.NodeIDs, n)
	g.fireGroupChanged(id)
	return nil
}

// RemoveNodeFromGroup removes n; a group that empties is auto-deleted.
func (g *Graph) RemoveNodeFromGroup(id GroupID, n NodeID) error {
	grp, ok := g.Groups[id]
	if !ok {
		return ErrGroupNotFound
	}
	kept := grp.NodeIDs[:0]
	for _, m := range grp.NodeIDs {
		if m != n {
			kept = append(kept, m)
		}
	}
	grp.NodeIDs = kept
	if len(grp.NodeIDs) == 0 {
		delete(g.Groups, id)
	}
	g.fireGroupChanged(id)
	return nil
}

// normalizeGroupRules validates and applies default clamps in place.
func normalizeGroupRules(rules []GroupRule) ([]GroupRule, error) {
	out := make([]GroupRule, 0, len(rules))
	for _, r := range rules {
		switch r.Param {
		case GroupParamPitch, GroupParamVolume, GroupParamDuration:
		default:
			return nil, fmt.Errorf("%w: unknown param %q", ErrGroupBadRule, r.Param)
		}
		if r.EveryN < 1 {
			return nil, fmt.Errorf("%w: every_n %d < 1", ErrGroupBadRule, r.EveryN)
		}
		if r.Min == 0 && r.Max == 0 {
			r.Min, r.Max = groupRuleDefaultClamp(r.Param)
		}
		if r.Min > r.Max {
			return nil, fmt.Errorf("%w: min %v > max %v", ErrGroupBadRule, r.Min, r.Max)
		}
		out = append(out, r)
	}
	return out, nil
}

// SetGroupRules replaces the group's rules (v1 UI passes 0 or 1).
func (g *Graph) SetGroupRules(id GroupID, rules []GroupRule) error {
	grp, ok := g.Groups[id]
	if !ok {
		return ErrGroupNotFound
	}
	norm, err := normalizeGroupRules(rules)
	if err != nil {
		return err
	}
	grp.Rules = norm
	g.fireGroupChanged(id)
	return nil
}

func (g *Graph) ClearGroupRules(id GroupID) error { return g.SetGroupRules(id, nil) }

// Group returns a deep copy (callers must never alias internal state).
func (g *Graph) Group(id GroupID) (NodeGroup, bool) {
	grp, ok := g.Groups[id]
	if !ok {
		return NodeGroup{}, false
	}
	return copyGroup(grp), true
}

func copyGroup(grp *NodeGroup) NodeGroup {
	return NodeGroup{
		ID:      grp.ID,
		Name:    grp.Name,
		NodeIDs: append([]NodeID(nil), grp.NodeIDs...),
		Rules:   append([]GroupRule(nil), grp.Rules...),
	}
}

// AllGroups returns deep copies sorted by ID.
func (g *Graph) AllGroups() []NodeGroup {
	out := make([]NodeGroup, 0, len(g.Groups))
	for _, grp := range g.Groups {
		out = append(out, copyGroup(grp))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// GroupsForNode returns the sorted IDs of every group containing n.
func (g *Graph) GroupsForNode(n NodeID) []GroupID {
	var out []GroupID
	for id, grp := range g.Groups {
		for _, m := range grp.NodeIDs {
			if m == n {
				out = append(out, id)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// removeNodeFromAllGroups strips n from every group (auto-deleting emptied
// groups). Called by Graph.RemoveNode so the "no dangling NodeIDs" invariant
// holds regardless of which code path deletes a node.
func (g *Graph) removeNodeFromAllGroups(n NodeID) {
	for id, grp := range g.Groups {
		for _, m := range grp.NodeIDs {
			if m == n {
				_ = g.RemoveNodeFromGroup(id, n)
				break
			}
		}
	}
}
