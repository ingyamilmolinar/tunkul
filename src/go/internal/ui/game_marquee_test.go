package ui

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ingyamilmolinar/beatmo/core/model"
)

// shiftDrag simulates press at (x1,y1), drag to (x2,y2), release — all with
// Shift held. Frames advance through the real Update loop.
func shiftDrag(g *Game, x1, y1, x2, y2 int) {
	pos := [2]int{x1, y1}
	down := true
	restore := SetInputForTest(
		func() (int, int) { return pos[0], pos[1] },
		func(b ebiten.MouseButton) bool { return down && b == ebiten.MouseButtonLeft },
		func(k ebiten.Key) bool { return k == ebiten.KeyShiftLeft },
		func() []rune { return nil },
		func() (float64, float64) { return 0, 0 },
		func() (int, int) { return 0, 0 },
	)
	defer restore()
	advanceFrames(g, 2) // press
	pos = [2]int{x2, y2}
	advanceFrames(g, 3) // drag
	down = false
	advanceFrames(g, 2) // release
}

func TestMarqueeCreatesGroupAndOpensMenu(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	// Compute a screen rect that encloses both nodes with margin.
	ax1, ay1, _, _ := g.nodeScreenRect(g.nodesByID[aID])
	_, _, bx2, by2 := g.nodeScreenRect(g.nodesByID[bID])
	t.Logf("node a rect start=(%v,%v) node b rect end=(%v,%v)", ax1, ay1, bx2, by2)
	shiftDrag(g, int(ax1)-20, int(ay1)-20, int(bx2)+20, int(by2)+20)

	groups := g.graph.AllGroups()
	if len(groups) != 1 {
		t.Fatalf("want 1 group after marquee, got %d", len(groups))
	}
	if len(groups[0].NodeIDs) != 2 {
		t.Fatalf("want both nodes in group, got %v", groups[0].NodeIDs)
	}
	if !g.groupMenu.IsOpen() || g.groupMenu.GroupID() != groups[0].ID {
		t.Fatalf("group menu must open for the new group")
	}
	// No node was created/selected by the release (marquee != click).
	if len(g.nodes) != 2 {
		t.Fatalf("marquee must not add nodes, have %d", len(g.nodes))
	}
}

func TestMarqueeOverEmptySpaceIsNoOp(t *testing.T) {
	g, _, _ := buildGroupLoopCircuit(t)
	// Far corner away from both nodes.
	shiftDrag(g, 600, 400, 700, 480)
	if n := len(g.graph.AllGroups()); n != 0 {
		t.Fatalf("empty marquee must create no group, got %d", n)
	}
	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must not open on empty marquee")
	}
}

func TestShiftClickWithoutDragIsNoOp(t *testing.T) {
	g, _, _ := buildGroupLoopCircuit(t)
	shiftDrag(g, 600, 400, 601, 401) // < 4px movement
	if n := len(g.graph.AllGroups()); n != 0 {
		t.Fatalf("tiny shift-click must be a no-op, got %d groups", n)
	}
}

func TestShiftDragFromNodeStillCreatesEdge(t *testing.T) {
	// Regression guard: link-drag (shift+drag STARTING ON a node) untouched.
	g, aID, bID := buildGroupLoopCircuit(t)
	g.deleteEdge(g.nodesByID[aID], g.nodesByID[bID]) // remove a->b, re-create via gesture
	ax1, ay1, ax2, ay2 := g.nodeScreenRect(g.nodesByID[aID])
	bx1, by1, bx2, by2 := g.nodeScreenRect(g.nodesByID[bID])
	shiftDrag(g, int((ax1+ax2)/2), int((ay1+ay2)/2), int((bx1+bx2)/2), int((by1+by2)/2))
	if _, ok := g.graph.Edges[[2]model.NodeID{aID, bID}]; !ok {
		t.Fatalf("link drag from node must still create the edge")
	}
	if n := len(g.graph.AllGroups()); n != 0 {
		t.Fatalf("link drag must not create a group, got %d", n)
	}
}

// gridClick simulates a plain (non-shift) press+release at (x,y) through the
// real Game.Update loop, mirroring shiftDrag but without Shift held and
// without movement (a simple click, not a drag).
func gridClick(g *Game, x, y int) {
	down := true
	restore := SetInputForTest(
		func() (int, int) { return x, y },
		func(b ebiten.MouseButton) bool { return down && b == ebiten.MouseButtonLeft },
		func(k ebiten.Key) bool { return false },
		func() []rune { return nil },
		func() (float64, float64) { return 0, 0 },
		func() (int, int) { return 0, 0 },
	)
	defer restore()
	advanceFrames(g, 2) // press
	down = false
	advanceFrames(g, 2) // release
}

// TestGridClickClosesOpenGroupMenu is the Task 7 review follow-up: the
// GroupMenu had no reachable click-outside close. A plain (non-shift) click
// on empty grid space must close an already-open GroupMenu, mirroring the
// sidebar's existing close-on-empty-click behavior.
func TestGridClickClosesOpenGroupMenu(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)
	if !g.groupMenu.IsOpen() {
		t.Fatalf("setup: group menu must be open")
	}
	// Plain click on empty grid space, far from the menu panel and any node.
	// (700,500) would land in the drum/audio pane for an 800x600 window (the
	// grid/drum split defaults to the vertical midpoint), so use a point in
	// the grid pane's top-right, away from both nodes (top-left) and the
	// menu panel (anchored at (100,100)).
	gridClick(g, 700, 50)
	if g.groupMenu.IsOpen() {
		t.Fatalf("plain grid click must close the open group menu")
	}
}

// TestNodeClickClosesOpenGroupMenu is the Finding 2 review follow-up:
// TestGridClickClosesOpenGroupMenu only covers the empty-space close site
// (the "else" branch of handleEditor's release handling). This variant
// exercises the sibling node-hit branch: a plain click ON an existing node
// must also close an already-open GroupMenu, and must open the node sidebar
// (confirming we actually hit the node branch, not the empty-space one).
func TestNodeClickClosesOpenGroupMenu(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)
	if !g.groupMenu.IsOpen() {
		t.Fatalf("setup: group menu must be open")
	}

	ax1, ay1, ax2, ay2 := g.nodeScreenRect(g.nodesByID[aID])
	cx, cy := int((ax1+ax2)/2), int((ay1+ay2)/2)
	gridClick(g, cx, cy)

	if g.groupMenu.IsOpen() {
		t.Fatalf("plain click on a node must close the open group menu")
	}
	if !g.sidebar.IsOpen() {
		t.Fatalf("plain click on a node must open the sidebar (confirms the node branch fired)")
	}
}

// shiftDragTo simulates a Shift+press+drag gesture with fine-grained control
// over the release position, distinct from the enclosing pane. Unlike
// shiftDrag (which drags from (x1,y1) straight to (x2,y2) and releases
// there), this helper moves through an explicit sequence of screen points,
// holding Shift throughout, with the button down for every point except the
// last (the release). Used by
// TestMarqueeCancelledWhenReleasedOutsideGridPane to reproduce a marquee
// drag that leaves the grid pane before release.
func shiftDragTo(g *Game, downAt bool, points ...[2]int) {
	if len(points) == 0 {
		return
	}
	pos := points[0]
	down := downAt
	restore := SetInputForTest(
		func() (int, int) { return pos[0], pos[1] },
		func(b ebiten.MouseButton) bool { return down && b == ebiten.MouseButtonLeft },
		func(k ebiten.Key) bool { return k == ebiten.KeyShiftLeft },
		func() []rune { return nil },
		func() (float64, float64) { return 0, 0 },
		func() (int, int) { return 0, 0 },
	)
	defer restore()
	advanceFrames(g, 2)
	for _, p := range points[1:] {
		pos = p
		advanceFrames(g, 2)
	}
}

// TestMarqueeCancelledWhenReleasedOutsideGridPane is the Finding 1 review
// follow-up: handleEditor's out-of-grid-pane early return (the
// `y < gridTopOffset() || !g.split.InGridPane(x, y)` branch) fired before the
// shift/marquee branch, so a marquee dragged out of the grid pane and
// released elsewhere never reached handleMarquee's release path. g.marquee
// stayed active forever (dead camera pan), and re-entering the grid pane
// with the button up replayed a stale release against frozen start/cur
// coords — potentially creating a phantom group.
func TestMarqueeCancelledWhenReleasedOutsideGridPane(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)

	// A rect (screen space) that encloses both nodes, mirroring
	// TestMarqueeCreatesGroupAndOpensMenu, so a stale/replayed release would
	// actually create a group if the bug is present.
	ax1, ay1, _, _ := g.nodeScreenRect(g.nodesByID[aID])
	_, _, bx2, by2 := g.nodeScreenRect(g.nodesByID[bID])
	startX, startY := int(ax1)-20, int(ay1)-20
	dragX, dragY := int(bx2)+20, int(by2)+20

	if !g.split.InGridPane(startX, startY) || !g.split.InGridPane(dragX, dragY) {
		t.Fatalf("setup: press/drag points must be inside the grid pane, got InGridPane(start)=%v InGridPane(drag)=%v",
			g.split.InGridPane(startX, startY), g.split.InGridPane(dragX, dragY))
	}
	outsideX, outsideY := 700, 550 // drum pane for an 800x600 default 50/50 split (split.Y=300)
	if g.split.InGridPane(outsideX, outsideY) {
		t.Fatalf("setup: (%d,%d) must be outside the grid pane", outsideX, outsideY)
	}

	// Press on empty space, drag to enclose both nodes (still inside the
	// grid pane), then move into the drum pane and release there.
	shiftDragTo(g, true,
		[2]int{startX, startY},
		[2]int{dragX, dragY},
		[2]int{outsideX, outsideY},
	)
	// Release explicitly while outside the grid pane.
	shiftDragReleaseAt(g, outsideX, outsideY)

	if g.marquee.active {
		t.Fatalf("marquee must be cancelled, not left active, when released outside the grid pane")
	}
	if n := len(g.graph.AllGroups()); n != 0 {
		t.Fatalf("no group should be created when the marquee is cancelled outside the grid pane, got %d", n)
	}
	if g.groupMenu.IsOpen() {
		t.Fatalf("group menu must not open when the marquee is cancelled outside the grid pane")
	}

	// Move the cursor back into the grid pane with the button already up —
	// must not resurrect a phantom group from stale start/cur coords.
	emptyX, emptyY := 700, 50 // empty grid space, away from both nodes
	shiftDragReleaseAt(g, emptyX, emptyY)

	if g.marquee.active {
		t.Fatalf("marquee must stay inactive after re-entering the grid pane with the button up")
	}
	if n := len(g.graph.AllGroups()); n != 0 {
		t.Fatalf("no phantom group should appear after re-entering the grid pane, got %d", n)
	}
	if g.groupMenu.IsOpen() {
		t.Fatalf("group menu must not open from a phantom stale-release replay")
	}
}

// TestMarqueeCancelledWhenReleasedOverGroupMenu is the final-review MUST-FIX
// follow-up: handleEditor only runs when
// `!inputHandled && !g.blocksAt(mx, my)` (game_update.go). If a marquee
// drag's release lands over the open GroupMenu (or sidebar/settings gear),
// the input dispatcher consumes the frame and handleEditor never sees the
// release — g.marquee.active is stranded true forever (dead camera pan via
// panOK), and the next plain grid click would replay the stale rect into a
// phantom group. This must instead cancel the marquee (never create a
// group) the moment the frame is consumed out from under it.
func TestMarqueeCancelledWhenReleasedOverGroupMenu(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 120, 80)
	advanceFrames(g, 1) // let the dispatcher register the now-open group menu
	if !g.groupMenu.IsOpen() {
		t.Fatalf("setup: group menu must be open")
	}

	bounds := g.groupMenu.InputBounds()
	startX, startY := 700, 50 // empty grid space, outside the panel
	if image.Pt(startX, startY).In(bounds) {
		t.Fatalf("setup: start point must be outside the panel, bounds=%v", bounds)
	}
	insideX, insideY := (bounds.Min.X+bounds.Max.X)/2, (bounds.Min.Y+bounds.Max.Y)/2

	// Shift+press on empty space (arms the marquee), drag to enclose a point
	// inside the panel, release there — the dispatcher consumes this release
	// frame because it lands inside the GroupMenu's InputBounds.
	shiftDrag(g, startX, startY, insideX, insideY)

	if g.marquee.active {
		t.Fatalf("marquee must be cancelled, not left dangling, when its release lands on the group menu")
	}
	if n := len(g.graph.AllGroups()); n != 1 {
		t.Fatalf("no phantom group should appear, want 1 (pre-existing) group, got %d", n)
	}

	// A subsequent plain click on empty grid space (clearly outside the
	// panel) must behave normally — add a node, not replay the stale
	// marquee rect into a second (phantom) group.
	clickX, clickY := 700, 200
	if image.Pt(clickX, clickY).In(bounds) {
		t.Fatalf("setup: post-cancel click point must be outside the panel, bounds=%v", bounds)
	}
	gridClick(g, clickX, clickY)
	if n := len(g.graph.AllGroups()); n != 1 {
		t.Fatalf("plain click after a cancelled marquee must not create a phantom group, got %d groups", n)
	}
}

// shiftDragReleaseAt advances a couple of frames with Shift held, the mouse
// button up, and the cursor at (x,y). Used after shiftDragTo to model
// "button is up, cursor moves" continuations of a gesture.
func shiftDragReleaseAt(g *Game, x, y int) {
	restore := SetInputForTest(
		func() (int, int) { return x, y },
		func(b ebiten.MouseButton) bool { return false },
		func(k ebiten.Key) bool { return k == ebiten.KeyShiftLeft },
		func() []rune { return nil },
		func() (float64, float64) { return 0, 0 },
		func() (int, int) { return 0, 0 },
	)
	defer restore()
	advanceFrames(g, 2)
}

// marqueeCreateGroup drives a real shift-drag enclosing both nodes of
// buildGroupLoopCircuit's a/b pair, returning the resulting (provisional)
// group's ID. Fails the test if the marquee didn't produce exactly one
// two-member group with the menu open.
func marqueeCreateGroup(t *testing.T, g *Game, aID, bID model.NodeID) model.GroupID {
	t.Helper()
	ax1, ay1, _, _ := g.nodeScreenRect(g.nodesByID[aID])
	_, _, bx2, by2 := g.nodeScreenRect(g.nodesByID[bID])
	shiftDrag(g, int(ax1)-20, int(ay1)-20, int(bx2)+20, int(by2)+20)

	groups := g.graph.AllGroups()
	if len(groups) != 1 || len(groups[0].NodeIDs) != 2 {
		t.Fatalf("setup: want 1 group with 2 members, got %+v", groups)
	}
	if !g.groupMenu.IsOpen() {
		t.Fatalf("setup: group menu must be open (provisional) after marquee-create")
	}
	return groups[0].ID
}

// TestMarqueeGroupDiscardedOnCloseWithoutSave is task-groupsave brief test 1:
// closing the menu (the same method the close button invokes) without ever
// pressing Save must silently discard the provisional group — no undo step,
// rings gone from the group index.
func TestMarqueeGroupDiscardedOnCloseWithoutSave(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	undoBefore := len(g.undoManager.undo)
	gid := marqueeCreateGroup(t, g, aID, bID)

	g.groupMenu.Close() // same method the close button invokes

	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must be closed")
	}
	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("provisional group must be deleted on close-without-save")
	}
	if got := len(g.undoManager.undo); got != undoBefore {
		t.Fatalf("discard must not add an undo step, before=%d after=%d", undoBefore, got)
	}
	idx := g.groupIndexSnapshot()
	if idx.Member(aID) || idx.Member(bID) {
		t.Fatalf("group rings must be gone after discard")
	}
}

// TestMarqueeGroupDiscardedOnGridClickOutside is task-groupsave brief test 2:
// a plain click on empty grid space discards the provisional group AND still
// performs its normal add-node behavior.
func TestMarqueeGroupDiscardedOnGridClickOutside(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid := marqueeCreateGroup(t, g, aID, bID)
	nodesBefore := len(g.nodes)

	// The menu anchors near the marquee release point (close to the node
	// pair), so pick a click point on the opposite side of the panel from
	// wherever it landed, rather than a fixed screen coordinate.
	panel := g.groupMenu.panel
	clickX, clickY := 50, 50
	if image.Pt(clickX, clickY).In(panel) {
		clickX = panel.Max.X + 50
	}
	gridClick(g, clickX, clickY) // empty grid space, away from nodes/menu

	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must close on click-outside")
	}
	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("provisional group must be deleted on click-outside")
	}
	if len(g.nodes) != nodesBefore+1 {
		t.Fatalf("plain click's normal add-node behavior must still happen, want %d nodes got %d", nodesBefore+1, len(g.nodes))
	}
}

// TestMarqueeGroupDiscardedOnNodeClick is task-groupsave brief test 3: a
// plain click ON a node discards the provisional group, closes the menu, and
// still opens the sidebar (confirms the node-hit branch fired).
func TestMarqueeGroupDiscardedOnNodeClick(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid := marqueeCreateGroup(t, g, aID, bID)

	ax1, ay1, ax2, ay2 := g.nodeScreenRect(g.nodesByID[aID])
	cx, cy := int((ax1+ax2)/2), int((ay1+ay2)/2)
	gridClick(g, cx, cy)

	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must close on node click")
	}
	if !g.sidebar.IsOpen() {
		t.Fatalf("node click must still open the sidebar")
	}
	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("provisional group must be deleted on node-click close")
	}
}

// TestMarqueeGroupSavedPersists is task-groupsave brief test 4: rule edits
// made while provisional, then Save, must land as exactly ONE undo step that
// carries the whole creation (membership + rule), undoable/redoable as a
// single atomic unit.
func TestMarqueeGroupSavedPersists(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	undoBefore := len(g.undoManager.undo)
	gid := marqueeCreateGroup(t, g, aID, bID)

	g.groupMenu.setRuleEnabled(true)
	g.groupMenu.stepRuleDelta(+1)
	advanceFrames(g, 2) // enqueueUI actions run inside Update

	g.groupMenu.save()
	advanceFrames(g, 2)

	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must close after save")
	}
	grp, ok := g.graph.Group(gid)
	if !ok {
		t.Fatalf("group must exist after save")
	}
	if len(grp.Rules) != 1 {
		t.Fatalf("saved group must keep its rule, got %+v", grp.Rules)
	}
	if got := len(g.undoManager.undo) - undoBefore; got != 1 {
		t.Fatalf("save must add exactly 1 undo step, got %d", got)
	}

	g.undoManager.Undo()
	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("undo must remove the saved group")
	}
	g.undoManager.Redo()
	grp, ok = g.graph.Group(gid)
	if !ok {
		t.Fatalf("redo must restore the group")
	}
	if len(grp.Rules) != 1 {
		t.Fatalf("redo must restore the group's rule, got %+v", grp.Rules)
	}
}

// TestProvisionalRuleEditsEmitNothing is task-groupsave brief test 5: rule
// edits made through the menu while the group is still provisional must not
// add undo steps (they only take effect, undo-wise, once Save is pressed).
func TestProvisionalRuleEditsEmitNothing(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	marqueeCreateGroup(t, g, aID, bID)
	undoBefore := len(g.undoManager.undo)

	g.groupMenu.setRuleEnabled(true)
	g.groupMenu.stepRuleDelta(+1)
	advanceFrames(g, 2)

	if got := len(g.undoManager.undo); got != undoBefore {
		t.Fatalf("provisional rule edits must not add undo steps, before=%d after=%d", undoBefore, got)
	}
}

// TestSecondMarqueeDiscardsPriorProvisionalGroup is the review Finding 1
// follow-up: openAt() used to overwrite groupID/provisional without
// discarding a still-open provisional group. A second Shift+drag marquee
// while a provisional menu was still open therefore leaked the first group
// into g.graph.Groups — uncommitted, menuless, exportable. openAt() must now
// silently discard the prior provisional group (same no-emit/no-undo
// contract as Close()) before adopting the new one.
func TestSecondMarqueeDiscardsPriorProvisionalGroup(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)

	// First marquee: encloses a/b, creates provisional group A with its menu
	// open (menuless orphan check below fails if this leaks).
	gidA := marqueeCreateGroup(t, g, aID, bID)

	// A third node in a distinct empty region (negative grid coords, which at
	// the default zoom maps well to the left of the panel — see below), so
	// the second marquee targets a disjoint selection from the first. Adding
	// this node is itself a real, committed undo step, so the "undo depth
	// unchanged" baseline below is taken AFTER it, isolating just the two
	// (both-provisional) marquee-creates that follow.
	nc := g.tryAddNode(-5, -5, model.NodeTypeRegular)
	undoBefore := len(g.undoManager.undo)
	cx1, cy1, cx2, cy2 := g.nodeScreenRect(nc)
	// A tight ±5px margin (vs. the usual ±20 elsewhere in this file) — at the
	// default zoom, nodes are packed closely enough on screen that a ±20
	// margin around (-5,-5) reaches back into node a's rect (~16px away),
	// which would make this marquee's selection overlap the first group
	// instead of targeting a disjoint one.
	p1 := image.Pt(int(cx1)-5, int(cy1)-5)
	p2 := image.Pt(int(cx2)+5, int(cy2)+5)

	// The first marquee's (still-open, uncommitted) menu panel is anchored
	// near its own release point and, per game_update.go's dispatcher-first
	// gate, ANY frame whose cursor lands inside an open InputHandler's bounds
	// gets consumed before handleEditor/handleMarquee ever sees it — for an
	// already-active marquee this actively cancels it outright (see
	// TestMarqueeCancelledWhenReleasedOverGroupMenu). shiftDrag only ever
	// samples the two endpoints (no interpolation), so both must sit clear of
	// the panel; picking negative grid coords for node c keeps this rect
	// entirely left of the panel's x-range regardless of exact camera state.
	panel := g.groupMenu.panel
	if p1.In(panel) || p2.In(panel) {
		t.Fatalf("setup: second marquee's press/release points must both fall outside the open menu panel=%v p1=%v p2=%v", panel, p1, p2)
	}

	shiftDrag(g, p1.X, p1.Y, p2.X, p2.Y)

	groups := g.graph.AllGroups()
	if len(groups) != 1 {
		t.Fatalf("want exactly 1 group after the second marquee discards the first, got %d: %+v", len(groups), groups)
	}
	if groups[0].ID == gidA {
		t.Fatalf("the surviving group must be the NEW (second) group, not the orphaned first one")
	}
	if _, ok := g.graph.Group(gidA); ok {
		t.Fatalf("first provisional group must be discarded, not orphaned into g.graph.Groups")
	}
	if !g.groupMenu.IsOpen() || g.groupMenu.GroupID() != groups[0].ID {
		t.Fatalf("menu must be open for the new (second) group, got open=%v id=%v want=%v",
			g.groupMenu.IsOpen(), g.groupMenu.GroupID(), groups[0].ID)
	}
	if got := len(g.undoManager.undo); got != undoBefore {
		t.Fatalf("neither marquee-create (both still provisional/discarded) should add an undo step, before=%d after=%d", undoBefore, got)
	}
}
