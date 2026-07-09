package ui

import (
	"image"
	"testing"

	"github.com/ingyamilmolinar/beatmo/core/model"
	"github.com/ingyamilmolinar/beatmo/internal/i18n"
)

// buildGroupLoopCircuitSized mirrors buildGroupLoopCircuit but takes an
// explicit window size and returns a 3-node loop (a -> b -> c -> a) so tests
// can exercise GroupMenu layout at a caller-chosen viewport (e.g. the
// 1280x720 default splitter where the fixed 9-row layout used to overflow
// the grid pane — see TestGroupMenuFitsWithinGridPane).
func buildGroupLoopCircuitSized(t *testing.T, w, h int) (*Game, model.NodeID, model.NodeID, model.NodeID) {
	t.Helper()
	assertDefaultParityState(t)
	g := New(testLogger)
	t.Cleanup(g.CloseForTest)
	g.Layout(w, h)
	g.updateBeatInfos()

	na := g.tryAddNode(0, 0, model.NodeTypeRegular)
	nb := g.tryAddNode(1, 0, model.NodeTypeRegular)
	nc := g.tryAddNode(2, 0, model.NodeTypeRegular)
	g.addEdge(na, nb)
	g.addEdge(nb, nc)
	g.addEdge(nc, na)
	g.start = na
	g.graph.StartNodeID = na.ID
	g.updateBeatInfos()
	return g, na.ID, nb.ID, nc.ID
}

// TestGroupMenuFitsWithinGridPane is the layout-fit regression: GroupMenu's
// panel (and every control inside it) must ALWAYS render entirely inside the
// grid pane, like the node sidebar does. Before the fix, layoutAt computed a
// fixed 9-row height without clamping it against gridH — at common window
// sizes (800x600, and the 1280x720 default splitter where the grid pane is
// well under half the window) the panel extended below the grid pane, where
// the drum view overpaints it, making the Delta/Every-N steppers and Delete
// button invisible and unreachable.
func TestGroupMenuFitsWithinGridPane(t *testing.T) {
	sizes := []struct{ w, h int }{{800, 600}, {1280, 720}}
	// Every control the brief calls out by id, including "save" (only present
	// while provisional — hence OpenAtProvisional below).
	controlIDs := []string{"delete", "n-", "n+", "delta-", "delta+", "save", "close", "ruleonoff", "param"}

	// Delta/Every-N combined stepper row (task-groupsave Every-N row fix): the
	// n- button used to render invisible because the pill's full-sentence
	// label ("Every %d rounds") overflowed past the pill's left edge and
	// painted over the already-drawn n- button — the rect itself was never
	// missing/zero-width, so plain containment checks above didn't catch it.
	stepPairIDs := []string{"delta-", "deltaval", "delta+", "n-", "nval", "n+"}

	for _, sz := range sizes {
		g, aID, bID, cID := buildGroupLoopCircuitSized(t, sz.w, sz.h)
		gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID, cID})
		if err != nil {
			t.Fatalf("%dx%d: CreateGroup: %v", sz.w, sz.h, err)
		}
		// Match the real post-marquee UI (sceneGroupMenuSetup): a live rule at
		// EveryN=4 so the pill-fit assertion below exercises the exact value
		// the defect screenshots showed truncating.
		if err := g.graph.SetGroupRules(gid, []model.GroupRule{{Param: model.GroupParamPitch, Delta: 2, EveryN: 4}}); err != nil {
			t.Fatalf("%dx%d: SetGroupRules: %v", sz.w, sz.h, err)
		}
		gridRect := g.split.GridRect(g.winW, g.winH)

		anchors := [][2]int{
			{gridRect.Dx() - 10, gridRect.Dy() - 10}, // near grid-pane bottom-right
			{20, 20},                                 // near grid-pane top-left
		}
		for _, a := range anchors {
			g.groupMenu.OpenAtProvisional(gid, a[0], a[1])
			advanceFrames(g, 1)

			panel := g.groupMenu.panel
			if !panel.In(gridRect) {
				t.Fatalf("%dx%d anchor %v: panel %v not contained in grid pane %v", sz.w, sz.h, a, panel, gridRect)
			}
			for _, id := range controlIDs {
				r, ok := g.groupMenu.rects[id]
				if !ok || r.Empty() {
					t.Fatalf("%dx%d anchor %v: control %q rect missing/empty (rects=%v)", sz.w, sz.h, a, id, g.groupMenu.rects)
				}
				if !r.In(panel) {
					t.Fatalf("%dx%d anchor %v: control %q rect %v not contained in panel %v", sz.w, sz.h, a, id, r, panel)
				}
			}

			// Every rect in the combined delta/everyN row must exist, be
			// non-empty, and (for the +/- buttons) be at least
			// sidebarIncBtnW wide.
			rects := make(map[string]image.Rectangle, len(stepPairIDs))
			for _, id := range stepPairIDs {
				r, ok := g.groupMenu.rects[id]
				if !ok || r.Empty() {
					t.Fatalf("%dx%d anchor %v: %q rect missing/empty", sz.w, sz.h, a, id)
				}
				rects[id] = r
			}
			for _, id := range []string{"delta-", "delta+", "n-", "n+"} {
				if w := rects[id].Dx(); w < sidebarIncBtnW {
					t.Fatalf("%dx%d anchor %v: %q width %d < sidebarIncBtnW %d", sz.w, sz.h, a, id, w, sidebarIncBtnW)
				}
			}
			// No pairwise overlap between the six controls sharing the row.
			for i := 0; i < len(stepPairIDs); i++ {
				for j := i + 1; j < len(stepPairIDs); j++ {
					idI, idJ := stepPairIDs[i], stepPairIDs[j]
					if rects[idI].Overlaps(rects[idJ]) {
						t.Fatalf("%dx%d anchor %v: %q %v overlaps %q %v", sz.w, sz.h, a, idI, rects[idI], idJ, rects[idJ])
					}
				}
			}

			// The Every-N pill's actual rendered label must fit inside its
			// own pill rect — this is the assertion that would have caught
			// the defect: the pre-fix label was the full "Every %d rounds"
			// sentence, which measures wider than the pill at every density
			// and overflowed onto the n- button.
			rule, _ := g.groupMenu.currentRule()
			label := g.groupMenu.everyNPillLabel(rule.EveryN)
			if tw, pw := StyledTextWidth(label, RoleBody), rects["nval"].Dx(); tw > pw {
				t.Fatalf("%dx%d anchor %v: everyN pill label %q width %d overflows pill width %d", sz.w, sz.h, a, label, tw, pw)
			}
		}
	}
}

// TestGroupMenuBatchValueTextUniform is the Part 1 RED case: before the fix,
// Draw hardcoded the batch pills to "-" regardless of live model state.
// batchValueText must format a uniform value exactly like the node sidebar
// does (formatNodeVolumeDb for volume, "%+d" for pitch, "%.2fx" for
// duration).
func TestGroupMenuBatchValueTextUniform(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, _ := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)

	if got, want := g.groupMenu.batchValueText(model.GroupParamPitch), "+0"; got != want {
		t.Fatalf("pitch batchValueText = %q, want %q", got, want)
	}
	if got, want := g.groupMenu.batchValueText(model.GroupParamDuration), "1.00x"; got != want {
		t.Fatalf("duration batchValueText = %q, want %q", got, want)
	}
	if got, want := g.groupMenu.batchValueText(model.GroupParamVolume), formatNodeVolumeDb(1); got != want {
		t.Fatalf("volume batchValueText = %q, want %q", got, want)
	}
}

// TestGroupMenuBatchValueTextMixed pins the mixed-values fallback: when the
// group's members disagree on a param, the pill must show the localized
// "mixed" label rather than a misleading single number.
func TestGroupMenuBatchValueTextMixed(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, _ := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	na, _ := g.graph.GetNodeByID(aID)
	pa := na.Params
	pa.Pitch = 3
	g.graph.SetNodeParams(aID, pa)
	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)

	if got, want := g.groupMenu.batchValueText(model.GroupParamPitch), i18n.T(i18n.KeyGroupMixed); got != want {
		t.Fatalf("mixed pitch batchValueText = %q, want %q", got, want)
	}
	// The untouched param (duration) must still report as uniform.
	if got, want := g.groupMenu.batchValueText(model.GroupParamDuration), "1.00x"; got != want {
		t.Fatalf("duration batchValueText = %q, want %q", got, want)
	}
}

// TestGroupMenuBatchValueTextUpdatesAfterBatchEdit pins that the pill text
// tracks live model state through a batch edit (applyBatch), not a cached
// snapshot.
func TestGroupMenuBatchValueTextUpdatesAfterBatchEdit(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, _ := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	g.groupMenu.OpenAt(gid, 100, 100)

	g.groupMenu.applyBatch("pit+")
	advanceFrames(g, 2)

	if got, want := g.groupMenu.batchValueText(model.GroupParamPitch), "+1"; got != want {
		t.Fatalf("pitch batchValueText after batch = %q, want %q", got, want)
	}
}

// TestGroupMenuBatchValueTextVanishedGroup is the defensive "-" case: a
// deleted/vanished group (or one with no members) must fall back to "-"
// rather than panicking.
func TestGroupMenuBatchValueTextVanishedGroup(t *testing.T) {
	g, aID, _ := buildGroupLoopCircuit(t)
	gid, _ := g.graph.CreateGroup("", []model.NodeID{aID})
	g.groupMenu.OpenAt(gid, 100, 100)
	g.groupMenu.deleteGroup()
	advanceFrames(g, 2)

	if got, want := g.groupMenu.batchValueText(model.GroupParamPitch), "-"; got != want {
		t.Fatalf("vanished group batchValueText = %q, want %q", got, want)
	}
}

// TestGroupMenuBatchValueTextMixedFitsPillAtEveryDensity mirrors
// TestGroupMenuFitsWithinGridPane's everyN-pill-fit assertion (the defect
// that fix guarded against): the mixed-label pill (or its "±" fallback via
// pillFitOrFallback) must always fit inside the batch pill rect, at every
// density.
func TestGroupMenuBatchValueTextMixedFitsPillAtEveryDensity(t *testing.T) {
	for _, d := range []Density{DensityCompact, DensityComfortable, DensitySpacious} {
		restore := SetDensityForTest(d)
		g, aID, bID := buildGroupLoopCircuit(t)
		na, _ := g.graph.GetNodeByID(aID)
		pa := na.Params
		pa.Pitch = 3
		g.graph.SetNodeParams(aID, pa)
		gid, _ := g.graph.CreateGroup("", []model.NodeID{aID, bID})
		g.groupMenu.OpenAt(gid, 100, 100)
		advanceFrames(g, 2)

		label := g.groupMenu.batchValueText(model.GroupParamPitch)
		r, ok := g.groupMenu.rects["pitval"]
		if !ok || r.Empty() {
			t.Fatalf("density %v: pitval rect missing/empty", d)
		}
		if tw, pw := StyledTextWidth(label, RoleBody), r.Dx(); tw > pw {
			t.Fatalf("density %v: mixed pill label %q width %d overflows pill width %d", d, label, tw, pw)
		}
		restore()
	}
}

func TestGroupMenuBatchPitchAppliesToAllMembersAsOneUndoStep(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, _ := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	g.groupMenu.OpenAt(gid, 100, 100)
	if !g.groupMenu.IsOpen() {
		t.Fatalf("menu should be open")
	}
	undoBefore := len(g.undoManager.undo)

	g.groupMenu.applyBatch("pit+")
	advanceFrames(g, 2) // enqueueUI actions run inside Update

	for _, id := range []model.NodeID{aID, bID} {
		n, _ := g.graph.GetNodeByID(id)
		if n.Params.Pitch != 1 {
			t.Fatalf("node %d pitch = %v, want 1", id, n.Params.Pitch)
		}
	}
	if got := len(g.undoManager.undo) - undoBefore; got != 1 {
		t.Fatalf("batch edit must be exactly 1 undo step, got %d", got)
	}
}

func TestGroupMenuRuleEditing(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, _ := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	g.groupMenu.OpenAt(gid, 100, 100)

	g.groupMenu.setRuleEnabled(true)
	g.groupMenu.stepRuleDelta(+1)
	g.groupMenu.stepRuleEveryN(+1) // 1 -> 2
	advanceFrames(g, 2)

	grp, _ := g.graph.Group(gid)
	if len(grp.Rules) != 1 {
		t.Fatalf("want 1 rule, got %+v", grp.Rules)
	}
	if grp.Rules[0].EveryN != 2 {
		t.Fatalf("EveryN = %d, want 2", grp.Rules[0].EveryN)
	}
	g.groupMenu.setRuleEnabled(false)
	advanceFrames(g, 2)
	grp, _ = g.graph.Group(gid)
	if len(grp.Rules) != 0 {
		t.Fatalf("rule off must clear rules, got %+v", grp.Rules)
	}
}

func TestGroupMenuDeleteGroupClosesMenu(t *testing.T) {
	g, aID, _ := buildGroupLoopCircuit(t)
	gid, _ := g.graph.CreateGroup("", []model.NodeID{aID})
	g.groupMenu.OpenAt(gid, 100, 100)
	g.groupMenu.deleteGroup()
	advanceFrames(g, 2)
	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must close when its group is deleted")
	}
	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("group must be deleted")
	}
}

// TestGroupMenuTitleShowsGroupName is the spec §4 gap follow-up: the header
// used to always draw the static i18n title regardless of the group's real
// Name. titleText() (read by Draw) must surface the live group name, falling
// back to the i18n title only when the group is unnamed or has vanished.
func TestGroupMenuTitleShowsGroupName(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	// CreateGroup("", ...) auto-names ("Group N"), so drive the empty-Name
	// fallback case explicitly via RenameGroup (which, unlike CreateGroup,
	// performs no empty-string validation) rather than relying on an
	// unreachable-in-practice CreateGroup state.
	gid, _ := g.graph.CreateGroup("temp", []model.NodeID{aID, bID})
	if err := g.graph.RenameGroup(gid, ""); err != nil {
		t.Fatalf("RenameGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)

	if got, want := g.groupMenu.titleText(), i18n.T(i18n.KeyGroupMenuTitle); got != want {
		t.Fatalf("unnamed group titleText() = %q, want fallback %q", got, want)
	}

	if err := g.graph.RenameGroup(gid, "Bassline"); err != nil {
		t.Fatalf("RenameGroup: %v", err)
	}
	if got, want := g.groupMenu.titleText(), "Bassline"; got != want {
		t.Fatalf("named group titleText() = %q, want %q", got, want)
	}

	g.groupMenu.deleteGroup()
	advanceFrames(g, 2)
	if got, want := g.groupMenu.titleText(), i18n.T(i18n.KeyGroupMenuTitle); got != want {
		t.Fatalf("vanished group titleText() = %q, want fallback %q", got, want)
	}
}

func TestGroupMenuClosesWhenGroupEmptiedByNodeDeletion(t *testing.T) {
	g, aID, _ := buildGroupLoopCircuit(t)
	gid, _ := g.graph.CreateGroup("", []model.NodeID{aID})
	g.groupMenu.OpenAt(gid, 100, 100)
	g.deleteNode(g.nodesByID[aID])
	advanceFrames(g, 2)
	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must close when its group empties/auto-deletes")
	}
}

// TestChipReopenCloseDoesNotDelete is task-groupsave brief test 6: a
// committed group (CreateGroup + OpenAt, non-provisional — the sidebar
// group-chip reopen path) must survive a plain Close(); the discard-on-close
// behavior only applies to provisional (unsaved marquee) groups.
func TestChipReopenCloseDoesNotDelete(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, _ := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	g.groupMenu.OpenAt(gid, 100, 100)

	g.groupMenu.Close()

	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must close")
	}
	if _, ok := g.graph.Group(gid); !ok {
		t.Fatalf("committed (non-provisional) group must survive a plain Close()")
	}
}

// TestProvisionalDeleteButtonSilent is task-groupsave brief test 7: pressing
// "Delete group" on a still-provisional group is a silent discard — same as
// closing without Save — never emitting a delete for a group that was never
// committed.
func TestProvisionalDeleteButtonSilent(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	undoBefore := len(g.undoManager.undo)
	gid := marqueeCreateGroup(t, g, aID, bID)

	g.groupMenu.deleteGroup()
	advanceFrames(g, 2)

	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must close")
	}
	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("group must be deleted")
	}
	if got := len(g.undoManager.undo); got != undoBefore {
		t.Fatalf("provisional delete-button must not add an undo step, before=%d after=%d", undoBefore, got)
	}
}

// TestProvisionalGroupEmptiedByNodeDeleteNoDoubleDelete is task-groupsave
// brief test 8: deleting the sole member of a provisional group triggers the
// model's auto-delete-on-empty (RemoveNodeFromGroup) which fires the vanish
// path (onGroupsChanged -> groupMenu.Close()) re-entrantly while the group is
// still marked provisional. Close()'s "group still exists" guard must make
// this a no-op rather than a double-delete/panic, and the ONLY undo step
// recorded must be the node deletion itself (the group's own creation/
// discard never emits).
func TestProvisionalGroupEmptiedByNodeDeleteNoDoubleDelete(t *testing.T) {
	g, aID, _ := buildGroupLoopCircuit(t)
	// Marquee over ONE node only: the two nodes in buildGroupLoopCircuit sit
	// only fractions of a pixel apart at the default zoom (adjacent grid
	// cells), so expanding the rect symmetrically would sweep in node b too.
	// Drag from up-and-left of node a to exactly node a's own far (bottom-
	// right) edge — big enough to clear marqueeMinDragPx, but never
	// extending toward node b.
	ax1, ay1, ax2, ay2 := g.nodeScreenRect(g.nodesByID[aID])
	shiftDrag(g, int(ax1)-10, int(ay1)-10, int(ax2), int(ay2))

	groups := g.graph.AllGroups()
	if len(groups) != 1 || len(groups[0].NodeIDs) != 1 {
		t.Fatalf("setup: want 1 group with 1 member, got %+v", groups)
	}
	gid := groups[0].ID
	if !g.groupMenu.IsOpen() {
		t.Fatalf("setup: menu must be open (provisional)")
	}
	undoBefore := len(g.undoManager.undo)

	g.deleteNode(g.nodesByID[aID])
	advanceFrames(g, 2)

	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must close")
	}
	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("group must be gone (auto-deleted by node removal)")
	}
	if got := len(g.undoManager.undo) - undoBefore; got != 1 {
		t.Fatalf("undo depth must reflect ONLY the node deletion, got delta %d", got)
	}
}

// TestImportClosesProvisionalMenuSilently is task-groupsave brief test 9:
// Import must close a still-open provisional GroupMenu without panicking and
// without leaking a phantom group into the freshly imported document.
func TestImportClosesProvisionalMenuSilently(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	// Capture a group-less export BEFORE the marquee creates a provisional
	// group (mirrors TestLegacyFileWithoutGroupsImportsClean).
	data, err := g.drum.exportBytes()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	marqueeCreateGroup(t, g, aID, bID)

	if err := g.Import(data); err != nil {
		t.Fatalf("Import: %v", err)
	}

	if g.groupMenu.IsOpen() {
		t.Fatalf("Import must close the group menu")
	}
	if n := len(g.graph.AllGroups()); n != 0 {
		t.Fatalf("imported doc's group set must be exactly what the file says (none), got %d", n)
	}
}

// TestGroupMenuSaveButtonVisibleOnlyWhileProvisional is task-groupsave brief
// test 10: the "save" control exists only while provisional, and is pruned
// (rect + button both absent) once the menu is opened non-provisionally.
func TestGroupMenuSaveButtonVisibleOnlyWhileProvisional(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, _ := g.graph.CreateGroup("", []model.NodeID{aID, bID})

	g.groupMenu.OpenAtProvisional(gid, 100, 100)
	if _, ok := g.groupMenu.rects["save"]; !ok {
		t.Fatalf("save control rect must exist while provisional")
	}
	if _, ok := g.groupMenu.btns["save"]; !ok {
		t.Fatalf("save button must be wired while provisional")
	}

	g.groupMenu.OpenAt(gid, 100, 100) // non-provisional
	if _, ok := g.groupMenu.rects["save"]; ok {
		t.Fatalf("save control rect must be absent for a non-provisional (committed) group")
	}
	if _, ok := g.groupMenu.btns["save"]; ok {
		t.Fatalf("save button must be pruned for a non-provisional group")
	}
}

// TestProvisionalBatchEditRecordsNodeStepOnly is the review Finding 2
// follow-up: pins the provisional batch-edit undo contract explicitly. A
// batch edit (applyBatch) applied while the group is still provisional must
// add exactly ONE undo step — the node batch itself (applyBatch's
// beginUndoGroup/endUndoGroup wrapping every per-node SetNodeParams), with
// no separate group-level step since emitGroupChanged is skipped while
// provisional. The node edits are real regardless of the group's fate: a
// subsequent close-without-save discards the group but must NOT touch the
// node param edits or add/remove any further undo step.
func TestProvisionalBatchEditRecordsNodeStepOnly(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	undoBefore := len(g.undoManager.undo)
	gid := marqueeCreateGroup(t, g, aID, bID)

	g.groupMenu.applyBatch("pit+")
	advanceFrames(g, 2) // enqueueUI actions run inside Update

	for _, id := range []model.NodeID{aID, bID} {
		n, _ := g.graph.GetNodeByID(id)
		if n.Params.Pitch != 1 {
			t.Fatalf("node %d pitch = %v, want 1", id, n.Params.Pitch)
		}
	}
	if got := len(g.undoManager.undo) - undoBefore; got != 1 {
		t.Fatalf("provisional batch edit must add exactly 1 undo step (the node batch), got %d", got)
	}

	g.groupMenu.Close() // close-without-save (same method the close button invokes)
	advanceFrames(g, 2)

	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("group must be discarded on close-without-save")
	}
	for _, id := range []model.NodeID{aID, bID} {
		n, _ := g.graph.GetNodeByID(id)
		if n.Params.Pitch != 1 {
			t.Fatalf("node %d pitch must survive the discard, got %v want 1", id, n.Params.Pitch)
		}
	}
	if got := len(g.undoManager.undo) - undoBefore; got != 1 {
		t.Fatalf("discard-without-save must not add/remove undo steps, want net +1 (batch only), got %d", got)
	}
}
