package model

import (
	"errors"
	"testing"
)

func newGroupTestGraph(t *testing.T) (*Graph, NodeID, NodeID, NodeID) {
	t.Helper()
	g := NewGraph(testLogger)
	a := g.AddNode(0, 0, NodeTypeRegular)
	b := g.AddNode(1, 0, NodeTypeRegular)
	c := g.AddNode(2, 0, NodeTypeRegular)
	return g, a, b, c
}

func TestCreateGroupAndRead(t *testing.T) {
	g, a, b, _ := newGroupTestGraph(t)
	id, err := g.CreateGroup("", []NodeID{a, b, a}) // dup a: deduped
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	grp, ok := g.Group(id)
	if !ok {
		t.Fatalf("Group(%d) not found", id)
	}
	if grp.Name == "" {
		t.Fatalf("expected default name, got empty")
	}
	if len(grp.NodeIDs) != 2 {
		t.Fatalf("want 2 members (deduped), got %v", grp.NodeIDs)
	}
	// Read is a copy: mutating it must not affect the graph.
	grp.NodeIDs[0] = 999
	grp2, _ := g.Group(id)
	if grp2.NodeIDs[0] == 999 {
		t.Fatalf("Group() must return a deep copy")
	}
}

func TestCreateGroupValidation(t *testing.T) {
	g, a, _, _ := newGroupTestGraph(t)
	if _, err := g.CreateGroup("x", nil); !errors.Is(err, ErrGroupEmptyNodes) {
		t.Fatalf("want ErrGroupEmptyNodes, got %v", err)
	}
	if _, err := g.CreateGroup("x", []NodeID{a, 12345}); !errors.Is(err, ErrGroupUnknownNode) {
		t.Fatalf("want ErrGroupUnknownNode, got %v", err)
	}
}

func TestGroupsForNodeAndMultiMembership(t *testing.T) {
	g, a, b, c := newGroupTestGraph(t)
	id1, _ := g.CreateGroup("", []NodeID{a, b})
	id2, _ := g.CreateGroup("", []NodeID{a, c})
	got := g.GroupsForNode(a)
	if len(got) != 2 || got[0] != id1 || got[1] != id2 {
		t.Fatalf("GroupsForNode(a) = %v, want [%d %d]", got, id1, id2)
	}
	if got := g.GroupsForNode(c); len(got) != 1 || got[0] != id2 {
		t.Fatalf("GroupsForNode(c) = %v", got)
	}
}

func TestSetGroupRulesValidationAndDefaults(t *testing.T) {
	g, a, _, _ := newGroupTestGraph(t)
	id, _ := g.CreateGroup("", []NodeID{a})
	if err := g.SetGroupRules(id, []GroupRule{{Param: "nope", Delta: 1, EveryN: 1}}); !errors.Is(err, ErrGroupBadRule) {
		t.Fatalf("bad param: want ErrGroupBadRule, got %v", err)
	}
	if err := g.SetGroupRules(id, []GroupRule{{Param: GroupParamPitch, Delta: 1, EveryN: 0}}); !errors.Is(err, ErrGroupBadRule) {
		t.Fatalf("EveryN<1: want ErrGroupBadRule, got %v", err)
	}
	if err := g.SetGroupRules(id, []GroupRule{{Param: GroupParamPitch, Delta: 1, EveryN: 1, Min: 5, Max: 2}}); !errors.Is(err, ErrGroupBadRule) {
		t.Fatalf("Min>Max: want ErrGroupBadRule, got %v", err)
	}
	// Zero Min/Max → per-param defaults applied on store.
	if err := g.SetGroupRules(id, []GroupRule{{Param: GroupParamPitch, Delta: 2, EveryN: 4}}); err != nil {
		t.Fatalf("SetGroupRules: %v", err)
	}
	grp, _ := g.Group(id)
	if grp.Rules[0].Min != -24 || grp.Rules[0].Max != 24 {
		t.Fatalf("pitch defaults: got Min=%v Max=%v, want -24/24", grp.Rules[0].Min, grp.Rules[0].Max)
	}
	if err := g.SetGroupRules(id, []GroupRule{{Param: GroupParamVolume, Delta: -0.1, EveryN: 2}}); err != nil {
		t.Fatalf("SetGroupRules vol: %v", err)
	}
	grp, _ = g.Group(id)
	if grp.Rules[0].Min != 0 || grp.Rules[0].Max != 4 {
		t.Fatalf("vol defaults: got %v/%v, want 0/4", grp.Rules[0].Min, grp.Rules[0].Max)
	}
}

func TestMembershipMutations(t *testing.T) {
	g, a, b, c := newGroupTestGraph(t)
	id, _ := g.CreateGroup("", []NodeID{a})
	if err := g.AddNodeToGroup(id, b); err != nil {
		t.Fatalf("AddNodeToGroup: %v", err)
	}
	if err := g.AddNodeToGroup(id, b); err != nil { // idempotent
		t.Fatalf("AddNodeToGroup dup: %v", err)
	}
	if err := g.SetGroupNodes(id, []NodeID{b, c}); err != nil {
		t.Fatalf("SetGroupNodes: %v", err)
	}
	if err := g.SetGroupNodes(id, nil); !errors.Is(err, ErrGroupEmptyNodes) {
		t.Fatalf("empty SetGroupNodes: want ErrGroupEmptyNodes, got %v", err)
	}
	if got := g.GroupsForNode(a); len(got) != 0 {
		t.Fatalf("a should have left the group, got %v", got)
	}
	// Removing the last member auto-deletes the group.
	if err := g.RemoveNodeFromGroup(id, b); err != nil {
		t.Fatalf("RemoveNodeFromGroup: %v", err)
	}
	if err := g.RemoveNodeFromGroup(id, c); err != nil {
		t.Fatalf("RemoveNodeFromGroup last: %v", err)
	}
	if _, ok := g.Group(id); ok {
		t.Fatalf("group should be auto-deleted when emptied")
	}
}

func TestRemoveNodeCleansGroups(t *testing.T) {
	g, a, b, _ := newGroupTestGraph(t)
	id, _ := g.CreateGroup("", []NodeID{a, b})
	g.RemoveNode(a)
	grp, ok := g.Group(id)
	if !ok || len(grp.NodeIDs) != 1 || grp.NodeIDs[0] != b {
		t.Fatalf("after RemoveNode(a): %v ok=%v", grp.NodeIDs, ok)
	}
	g.RemoveNode(b)
	if _, ok := g.Group(id); ok {
		t.Fatalf("group must auto-delete when last member node is removed")
	}
}

func TestMoveNodePreservesMembership(t *testing.T) {
	g, a, _, _ := newGroupTestGraph(t)
	id, _ := g.CreateGroup("", []NodeID{a})
	if !g.MoveNode(a, 5, 7) {
		t.Fatalf("MoveNode failed")
	}
	if got := g.GroupsForNode(a); len(got) != 1 || got[0] != id {
		t.Fatalf("membership must survive MoveNode, got %v", got)
	}
}

func TestGroupChangedHookFires(t *testing.T) {
	g, a, b, _ := newGroupTestGraph(t)
	var fired []GroupID
	g.SetGroupChangedHook(func(id GroupID) { fired = append(fired, id) })
	id, _ := g.CreateGroup("", []NodeID{a, b})
	_ = g.SetGroupRules(id, []GroupRule{{Param: GroupParamPitch, Delta: 1, EveryN: 1}})
	_ = g.RenameGroup(id, "melody")
	_ = g.DeleteGroup(id)
	if len(fired) != 4 {
		t.Fatalf("hook should fire once per mutation (create/rules/rename/delete), got %d: %v", len(fired), fired)
	}
	if err := g.DeleteGroup(id); !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("double delete: want ErrGroupNotFound, got %v", err)
	}
}
