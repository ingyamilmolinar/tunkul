package ui

import (
	"image"
	"math"
	"testing"

	"github.com/ingyamilmolinar/beatmo/core/model"
	"github.com/ingyamilmolinar/beatmo/internal/i18n"
)

// ─── Shared helpers ─────────────────────────────────────────────────────────

// almostEqual compares floats with a small epsilon, needed because the
// batch steppers accumulate via float64 arithmetic (e.g. 1.0-0.1) which
// isn't guaranteed bit-identical to the literal constant.
func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// clickMenuControl fails the test if the named GroupMenu control's rect is
// missing/empty, else clicks its center through the REAL Game.Update input
// path (gridClick). Per the house rule (functional tests drive Game.Update,
// not isolated method calls), this is the only way the E2E suite below
// exercises GroupMenu's buttons — mirroring the already-established fact
// (zz_repro_groupmenu_click_test.go, superseded by this file) that rects are
// live and clickable without any Draw() call in this harness.
func clickMenuControl(t *testing.T, g *Game, id string) {
	t.Helper()
	r, ok := g.groupMenu.rects[id]
	if !ok || r.Empty() {
		t.Fatalf("clickMenuControl(%q): rect missing/empty (rects=%v)", id, g.groupMenu.rects)
	}
	cx, cy := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
	gridClick(g, cx, cy)
}

// ─── A. Every control through real clicks (positive) ───────────────────────

// TestGroupMenuE2E_BatchButtonsApplyToAllMembersOneUndoStep drives all six
// batch steppers (vol-/vol+/pit-/pit+/dur-/dur+) through real clicks: each
// must apply to every member with the exact expected value, cost exactly one
// undo step, and the corresponding pill must reflect the new uniform value
// via batchValueText (the Part 1 fix).
func TestGroupMenuE2E_BatchButtonsApplyToAllMembersOneUndoStep(t *testing.T) {
	cases := []struct {
		id    string
		param model.GroupParam
		get   func(model.NodeParams) float64
		want  float64
	}{
		{"vol-", model.GroupParamVolume, func(p model.NodeParams) float64 { return p.Volume }, 0.9},
		{"vol+", model.GroupParamVolume, func(p model.NodeParams) float64 { return p.Volume }, 1.1},
		{"pit-", model.GroupParamPitch, func(p model.NodeParams) float64 { return p.Pitch }, -1},
		{"pit+", model.GroupParamPitch, func(p model.NodeParams) float64 { return p.Pitch }, 1},
		{"dur-", model.GroupParamDuration, func(p model.NodeParams) float64 { return p.Duration }, 0.9},
		{"dur+", model.GroupParamDuration, func(p model.NodeParams) float64 { return p.Duration }, 1.1},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			g, aID, bID := buildGroupLoopCircuit(t)
			gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
			if err != nil {
				t.Fatalf("CreateGroup: %v", err)
			}
			g.groupMenu.OpenAt(gid, 100, 100)
			advanceFrames(g, 2)
			undoBefore := len(g.undoManager.undo)

			clickMenuControl(t, g, c.id)
			advanceFrames(g, 2)

			for _, id := range []model.NodeID{aID, bID} {
				n, _ := g.graph.GetNodeByID(id)
				if got := c.get(n.Params); !almostEqual(got, c.want) {
					t.Fatalf("node %d %s = %v, want %v", id, c.id, got, c.want)
				}
			}
			if got := len(g.undoManager.undo) - undoBefore; got != 1 {
				t.Fatalf("%s must be exactly 1 undo step, got %d", c.id, got)
			}
			na, _ := g.graph.GetNodeByID(aID)
			wantText := formatGroupParamValue(c.param, c.get(na.Params))
			if got := g.groupMenu.batchValueText(c.param); got != wantText {
				t.Fatalf("%s pill text = %q, want %q (formatted uniform value)", c.id, got, wantText)
			}
		})
	}
}

// TestGroupMenuE2E_BatchButtonsClampAtBoundary is the clamp-edge case: both
// members already sit at the pitch ceiling (+24); a real pit+ click must
// leave them unchanged rather than exceeding the model's legal range.
func TestGroupMenuE2E_BatchButtonsClampAtBoundary(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	for _, id := range []model.NodeID{aID, bID} {
		n, _ := g.graph.GetNodeByID(id)
		p := n.Params
		p.Pitch = 24
		g.graph.SetNodeParams(id, p)
	}
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)

	clickMenuControl(t, g, "pit+")
	advanceFrames(g, 2)

	for _, id := range []model.NodeID{aID, bID} {
		n, _ := g.graph.GetNodeByID(id)
		if n.Params.Pitch != 24 {
			t.Fatalf("node %d pitch = %v, want clamped 24", id, n.Params.Pitch)
		}
	}
}

// TestGroupMenuE2E_MixedValuesBatchValueTextAndClickShiftsEach: members start
// at different pitches (2 and 5) — batchValueText must report the mixed
// label, and a real pit+ click must shift EACH member by +1 independently
// (staying mixed, but each individually advanced), not force them together.
func TestGroupMenuE2E_MixedValuesBatchValueTextAndClickShiftsEach(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	na, _ := g.graph.GetNodeByID(aID)
	pa := na.Params
	pa.Pitch = 2
	g.graph.SetNodeParams(aID, pa)
	nb, _ := g.graph.GetNodeByID(bID)
	pb := nb.Params
	pb.Pitch = 5
	g.graph.SetNodeParams(bID, pb)

	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)

	if got, want := g.groupMenu.batchValueText(model.GroupParamPitch), i18n.T(i18n.KeyGroupMixed); got != want {
		t.Fatalf("mixed pitch pill = %q, want %q", got, want)
	}

	clickMenuControl(t, g, "pit+")
	advanceFrames(g, 2)

	na, _ = g.graph.GetNodeByID(aID)
	nb, _ = g.graph.GetNodeByID(bID)
	if na.Params.Pitch != 3 {
		t.Fatalf("node a pitch = %v, want 3", na.Params.Pitch)
	}
	if nb.Params.Pitch != 6 {
		t.Fatalf("node b pitch = %v, want 6", nb.Params.Pitch)
	}
	if got, want := g.groupMenu.batchValueText(model.GroupParamPitch), i18n.T(i18n.KeyGroupMixed); got != want {
		t.Fatalf("still-mixed pitch pill after click = %q, want %q", got, want)
	}
}

// TestGroupMenuE2E_RuleOnOffClickTogglesRule drives the "ruleonoff" button
// through real clicks: first click enables (Rules 0->1), second disables
// (Rules 1->0).
func TestGroupMenuE2E_RuleOnOffClickTogglesRule(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)

	clickMenuControl(t, g, "ruleonoff")
	advanceFrames(g, 2)
	grp, _ := g.graph.Group(gid)
	if len(grp.Rules) != 1 {
		t.Fatalf("ruleonoff click must enable rule, got %+v", grp.Rules)
	}

	clickMenuControl(t, g, "ruleonoff")
	advanceFrames(g, 2)
	grp, _ = g.graph.Group(gid)
	if len(grp.Rules) != 0 {
		t.Fatalf("second ruleonoff click must disable rule, got %+v", grp.Rules)
	}
}

// TestGroupMenuE2E_ParamCycleClickCyclesRuleParam drives the "param" button
// through real clicks: pitch -> volume -> duration -> pitch, asserting
// Rules[0].Param and the re-derived per-param clamps at every step.
func TestGroupMenuE2E_ParamCycleClickCyclesRuleParam(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)

	clickMenuControl(t, g, "ruleonoff") // enable, default param pitch
	advanceFrames(g, 2)

	wantSeq := []struct {
		param    model.GroupParam
		min, max float64
	}{
		{model.GroupParamVolume, 0, 4},
		{model.GroupParamDuration, 0.1, 4},
		{model.GroupParamPitch, -24, 24},
	}
	for _, w := range wantSeq {
		clickMenuControl(t, g, "param")
		advanceFrames(g, 2)
		grp, _ := g.graph.Group(gid)
		if len(grp.Rules) != 1 {
			t.Fatalf("param click must keep exactly 1 rule, got %+v", grp.Rules)
		}
		r := grp.Rules[0]
		if r.Param != w.param {
			t.Fatalf("param = %v, want %v", r.Param, w.param)
		}
		if r.Min != w.min || r.Max != w.max {
			t.Fatalf("clamps for %v = [%v,%v], want [%v,%v]", w.param, r.Min, r.Max, w.min, w.max)
		}
	}
}

// TestGroupMenuE2E_DeltaStepClickAppliesPitchThenVolumeStep verifies the
// delta+/delta- steppers: pitch steps by 1.0, and after cycling the rule to
// volume the step becomes 0.05 (stepRuleDelta's per-param step size).
func TestGroupMenuE2E_DeltaStepClickAppliesPitchThenVolumeStep(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)

	clickMenuControl(t, g, "ruleonoff") // enable, pitch, delta 1
	advanceFrames(g, 2)
	clickMenuControl(t, g, "delta+")
	advanceFrames(g, 2)
	grp, _ := g.graph.Group(gid)
	if !almostEqual(grp.Rules[0].Delta, 2) {
		t.Fatalf("pitch delta+ = %v, want 2", grp.Rules[0].Delta)
	}
	clickMenuControl(t, g, "delta-")
	advanceFrames(g, 2)
	clickMenuControl(t, g, "delta-")
	advanceFrames(g, 2)
	grp, _ = g.graph.Group(gid)
	if !almostEqual(grp.Rules[0].Delta, 0) {
		t.Fatalf("pitch delta after two delta- clicks = %v, want 0", grp.Rules[0].Delta)
	}

	clickMenuControl(t, g, "param") // cycle pitch -> volume
	advanceFrames(g, 2)
	grp, _ = g.graph.Group(gid)
	if grp.Rules[0].Param != model.GroupParamVolume {
		t.Fatalf("setup: expected volume param, got %v", grp.Rules[0].Param)
	}
	deltaBefore := grp.Rules[0].Delta

	clickMenuControl(t, g, "delta+")
	advanceFrames(g, 2)
	grp, _ = g.graph.Group(gid)
	if !almostEqual(grp.Rules[0].Delta, deltaBefore+0.05) {
		t.Fatalf("volume delta+ = %v, want %v", grp.Rules[0].Delta, deltaBefore+0.05)
	}
}

// TestGroupMenuE2E_EveryNStepClickFloorsAtOne verifies n+/n- steppers,
// including the EveryN floor at 1.
func TestGroupMenuE2E_EveryNStepClickFloorsAtOne(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)

	clickMenuControl(t, g, "ruleonoff") // enable, EveryN default 1
	advanceFrames(g, 2)

	clickMenuControl(t, g, "n-") // already at floor
	advanceFrames(g, 2)
	grp, _ := g.graph.Group(gid)
	if grp.Rules[0].EveryN != 1 {
		t.Fatalf("n- at floor = %d, want 1", grp.Rules[0].EveryN)
	}

	clickMenuControl(t, g, "n+")
	advanceFrames(g, 2)
	grp, _ = g.graph.Group(gid)
	if grp.Rules[0].EveryN != 2 {
		t.Fatalf("n+ = %d, want 2", grp.Rules[0].EveryN)
	}

	clickMenuControl(t, g, "n-")
	advanceFrames(g, 2)
	grp, _ = g.graph.Group(gid)
	if grp.Rules[0].EveryN != 1 {
		t.Fatalf("n- back down = %d, want 1", grp.Rules[0].EveryN)
	}
}

// TestGroupMenuE2E_SaveClickCommitsProvisionalGroup: a real marquee creates a
// provisional group; a real "save" click must commit it as exactly 1 undo
// step and close the menu.
func TestGroupMenuE2E_SaveClickCommitsProvisionalGroup(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	undoBefore := len(g.undoManager.undo)
	gid := marqueeCreateGroup(t, g, aID, bID)
	advanceFrames(g, 2)

	clickMenuControl(t, g, "save")
	advanceFrames(g, 2)

	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must close after save")
	}
	if _, ok := g.graph.Group(gid); !ok {
		t.Fatalf("saved group must persist")
	}
	if got := len(g.undoManager.undo) - undoBefore; got != 1 {
		t.Fatalf("save must emit exactly 1 undo step, got %d", got)
	}
}

// TestGroupMenuE2E_CloseClickDiscardsProvisionalGroup: a real "close" click
// on a still-provisional (never-saved) group must silently discard it — no
// undo step.
func TestGroupMenuE2E_CloseClickDiscardsProvisionalGroup(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	undoBefore := len(g.undoManager.undo)
	gid := marqueeCreateGroup(t, g, aID, bID)
	advanceFrames(g, 2)

	clickMenuControl(t, g, "close")
	advanceFrames(g, 2)

	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must be closed")
	}
	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("provisional group must be discarded on close click")
	}
	if got := len(g.undoManager.undo); got != undoBefore {
		t.Fatalf("close-without-save must not add an undo step, before=%d after=%d", undoBefore, got)
	}
}

// TestGroupMenuE2E_DeleteClickRemovesCommittedGroup: a real "delete" click on
// an already-committed (non-provisional) group deletes it and emits exactly
// 1 undo step.
func TestGroupMenuE2E_DeleteClickRemovesCommittedGroup(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	// Baseline the group's creation into the undo snapshot journal the same
	// way a real Save would (emitGroupCreated) — a direct graph.CreateGroup
	// call is untracked, so without this the journal's "committed" baseline
	// never included the group in the first place, and deleting it would
	// land back on that same untracked baseline (zero diff, no undo entry
	// pushed by design — see UndoManager.commitNow). Baselining first isolates
	// the delete-click's own step, matching how a real committed group
	// reached this state.
	grp, _ := g.graph.Group(gid)
	emitGroupCreated(gid, grp.Name, grp.NodeIDs)
	advanceFrames(g, 2)

	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)
	undoBefore := len(g.undoManager.undo)

	clickMenuControl(t, g, "delete")
	advanceFrames(g, 2)

	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must close after delete")
	}
	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("group must be deleted")
	}
	if got := len(g.undoManager.undo) - undoBefore; got != 1 {
		t.Fatalf("delete on committed group must emit exactly 1 undo step, got %d", got)
	}
}

// TestGroupMenuE2E_ClickInsidePanelNoControlIsConsumed is the negative
// click-through guard: a real click inside the panel bounds but on no
// control (the "batchlabel" half-row, which has no button) must be consumed
// — the menu stays open, no model change, no undo step, and (crucially) no
// node gets created at that grid position, which would happen if the click
// fell through to the grid beneath (see HandleInput's "opaque to z"
// contract).
func TestGroupMenuE2E_ClickInsidePanelNoControlIsConsumed(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)

	r, ok := g.groupMenu.rects["batchlabel"]
	if !ok || r.Empty() {
		t.Fatalf("setup: batchlabel rect missing/empty")
	}
	cx, cy := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
	pt := image.Pt(cx, cy)
	for id, cr := range g.groupMenu.rects {
		if id == "panel" || id == "batchlabel" || id == "title" || id == "rulelabel" {
			continue
		}
		if pt.In(cr) {
			t.Fatalf("setup: chosen point (%d,%d) unexpectedly inside control %q %v", cx, cy, id, cr)
		}
	}

	nodesBefore := len(g.nodes)
	undoBefore := len(g.undoManager.undo)

	gridClick(g, cx, cy)
	advanceFrames(g, 2)

	if !g.groupMenu.IsOpen() {
		t.Fatalf("click on panel whitespace must not close the menu")
	}
	if len(g.nodes) != nodesBefore {
		t.Fatalf("click-through created a node at (%d,%d); want none (panel must be opaque to z)", cx, cy)
	}
	if _, ok := g.graph.Group(gid); !ok {
		t.Fatalf("group must be unaffected by a whitespace click")
	}
	if got := len(g.undoManager.undo); got != undoBefore {
		t.Fatalf("whitespace click must not add an undo step, before=%d after=%d", undoBefore, got)
	}
}

// ─── B. Playback-time edits ─────────────────────────────────────────────────

// TestGroupMenuE2E_PlaybackBatchPitchEditAppliesLive: a real pit+ click
// during playback applies to all members, and evalNodeParamsOnly at a future
// index reflects the new base pitch. Surviving advanceFrames afterward (with
// PARITY_FATAL left enabled by stopPlaybackForTest's setup) is the no-
// parity-mismatch signal.
func TestGroupMenuE2E_PlaybackBatchPitchEditAppliesLive(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	for i := range g.drum.Rows[0].Steps {
		g.drum.Rows[0].Steps[i] = true
	}
	g.refreshDrumRow()

	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)

	g.SetPlaying(true)
	t.Cleanup(func() { stopPlaybackForTest(g) })
	advanceFrames(g, 5)

	clickMenuControl(t, g, "pit+")
	advanceFrames(g, 3)

	for _, id := range []model.NodeID{aID, bID} {
		n, _ := g.graph.GetNodeByID(id)
		if n.Params.Pitch != 1 {
			t.Fatalf("node %d pitch = %v, want 1", id, n.Params.Pitch)
		}
	}

	loopStart := g.loopStartByRow[0]
	loopLen := len(g.beatInfosByRow[0]) - loopStart
	if !g.isLoopByRow[0] || loopLen <= 0 {
		t.Fatalf("expected loop row, isLoop=%v loopLen=%d", g.isLoopByRow[0], loopLen)
	}
	idx := loopStart + loopLen // a future round
	info := g.beatInfoAtRow(0, idx)
	_, pitch, _ := g.evalNodeParamsOnly(0, idx, info)
	if pitch != 1 {
		t.Fatalf("future-index pitch = %v, want 1", pitch)
	}

	// The test surviving further ticking without a parity panic is the
	// no-mismatch signal (parityFatalEnabled stays true; see testutil header).
	advanceFrames(g, 10)
}

// TestGroupMenuE2E_PlaybackRuleEditPreservesTriggerParity: enabling a group
// rule via real clicks (ruleonoff + delta+) on a COMMITTED group during
// playback must never alter trigger/audible decisions — mirrors
// parity_group_rule_test.go's TestParityGroupRuleNeverAltersTriggerParity,
// driven through the real click path instead of the model API directly.
func TestGroupMenuE2E_PlaybackRuleEditPreservesTriggerParity(t *testing.T) {
	assertDefaultParityState(t)
	g, aID, bID := buildGroupLoopCircuit(t)
	for i := range g.drum.Rows[0].Steps {
		g.drum.Rows[0].Steps[i] = true
	}
	g.refreshDrumRow()
	baseline := append([]bool(nil), g.drum.Rows[0].Steps...)

	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100) // committed (non-provisional)
	advanceFrames(g, 2)

	g.SetPlaying(true)
	t.Cleanup(func() { stopPlaybackForTest(g) })
	advanceFrames(g, 5)

	clickMenuControl(t, g, "ruleonoff") // enable rule (pitch, delta 1, everyN 1)
	advanceFrames(g, 2)
	clickMenuControl(t, g, "delta+") // delta 1 -> 2
	advanceFrames(g, 3)

	grp, ok := g.graph.Group(gid)
	if !ok || len(grp.Rules) != 1 || grp.Rules[0].Param != model.GroupParamPitch || !almostEqual(grp.Rules[0].Delta, 2) {
		t.Fatalf("want pitch rule delta=2 after ruleonoff+delta+ clicks, got %+v ok=%v", grp, ok)
	}

	// Trigger parity must be UNCHANGED by the rule.
	for i, on := range baseline {
		abs := g.drum.Offset + i
		if got := g.drum.Rows[0].Steps[i]; got != on {
			t.Fatalf("rule click changed rendered Steps at i=%d: got=%v want=%v", i, got, on)
		}
		if got := g.engine.Predictor.VisibleAt(0, abs); got != on {
			t.Fatalf("rule click changed predictor VisibleAt at abs=%d: got=%v want=%v", abs, got, on)
		}
	}

	loopStart := g.loopStartByRow[0]
	loopLen := len(g.beatInfosByRow[0]) - loopStart
	if !g.isLoopByRow[0] || loopLen <= 0 {
		t.Fatalf("expected loop row, isLoop=%v loopLen=%d", g.isLoopByRow[0], loopLen)
	}
	infoAt := func(idx int) model.BeatInfo { return g.beatInfoAtRow(0, idx) }
	for round := 0; round < 3; round++ {
		idx := loopStart + round*loopLen
		info := infoAt(idx)
		_, pitch, _ := g.evalNodeParamsOnly(0, idx, info)
		want := float64(round) * 2
		if pitch != want {
			t.Fatalf("round %d pitch = %v, want %v", round, pitch, want)
		}
		if !g.nodeTriggered(0, idx, info) {
			t.Fatalf("round %d: node must still trigger despite the pitch rule", round)
		}
		if want := g.engine.Predictor.VisibleAt(0, idx); !want {
			t.Fatalf("round %d: predictor must still show visible/firing despite the rule", round)
		}
	}
}

// TestGroupMenuE2E_PlaybackMembershipRemovalRevertsBaseParams: removing a
// node from a ruled group during playback (via the model API, per the brief)
// must revert that node's effective pitch to base at future indices, while
// the remaining member keeps the rule's effect.
func TestGroupMenuE2E_PlaybackMembershipRemovalRevertsBaseParams(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := g.graph.SetGroupRules(gid, []model.GroupRule{
		{Param: model.GroupParamPitch, Delta: 2, EveryN: 1},
	}); err != nil {
		t.Fatalf("SetGroupRules: %v", err)
	}
	g.refreshDrumRow()

	g.SetPlaying(true)
	t.Cleanup(func() { stopPlaybackForTest(g) })
	advanceFrames(g, 5)

	loopStart := g.loopStartByRow[0]
	loopLen := len(g.beatInfosByRow[0]) - loopStart
	if !g.isLoopByRow[0] || loopLen <= 0 {
		t.Fatalf("expected loop row, isLoop=%v loopLen=%d", g.isLoopByRow[0], loopLen)
	}
	infoAt := func(idx int) model.BeatInfo { return g.beatInfoAtRow(0, idx) }
	findIdx := func(id model.NodeID) int {
		for i := 0; i < loopLen; i++ {
			if infoAt(loopStart + i).NodeID == id {
				return loopStart + i
			}
		}
		t.Fatalf("node %d not present in loop", id)
		return -1
	}
	idxA, idxB := findIdx(aID), findIdx(bID)
	const round = 2
	idxA2, idxB2 := idxA+round*loopLen, idxB+round*loopLen

	_, pitchA, _ := g.evalNodeParamsOnly(0, idxA2, infoAt(idxA2))
	_, pitchB, _ := g.evalNodeParamsOnly(0, idxB2, infoAt(idxB2))
	if pitchA != 4 || pitchB != 4 {
		t.Fatalf("setup: round-2 pitch A=%v B=%v, want both 4", pitchA, pitchB)
	}

	if err := g.graph.RemoveNodeFromGroup(gid, aID); err != nil {
		t.Fatalf("RemoveNodeFromGroup: %v", err)
	}
	advanceFrames(g, 2)

	_, pitchA, _ = g.evalNodeParamsOnly(0, idxA2, infoAt(idxA2))
	_, pitchB, _ = g.evalNodeParamsOnly(0, idxB2, infoAt(idxB2))
	if pitchA != 0 {
		t.Fatalf("removed member A pitch = %v, want back to base 0", pitchA)
	}
	if pitchB != 4 {
		t.Fatalf("remaining member B pitch = %v, want unchanged 4", pitchB)
	}
}

// TestGroupMenuE2E_PlaybackDeleteClickRevertsAllMembersToBase: a real
// "delete" click on a ruled, committed group during playback must revert
// every member's effective pitch to base at future indices.
func TestGroupMenuE2E_PlaybackDeleteClickRevertsAllMembersToBase(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := g.graph.SetGroupRules(gid, []model.GroupRule{
		{Param: model.GroupParamPitch, Delta: 2, EveryN: 1},
	}); err != nil {
		t.Fatalf("SetGroupRules: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)

	g.SetPlaying(true)
	t.Cleanup(func() { stopPlaybackForTest(g) })
	advanceFrames(g, 5)

	loopStart := g.loopStartByRow[0]
	loopLen := len(g.beatInfosByRow[0]) - loopStart
	if !g.isLoopByRow[0] || loopLen <= 0 {
		t.Fatalf("expected loop row, isLoop=%v loopLen=%d", g.isLoopByRow[0], loopLen)
	}
	infoAt := func(idx int) model.BeatInfo { return g.beatInfoAtRow(0, idx) }
	findIdx := func(id model.NodeID) int {
		for i := 0; i < loopLen; i++ {
			if infoAt(loopStart + i).NodeID == id {
				return loopStart + i
			}
		}
		t.Fatalf("node %d not present in loop", id)
		return -1
	}
	idxA, idxB := findIdx(aID), findIdx(bID)
	const round = 2
	idxA2, idxB2 := idxA+round*loopLen, idxB+round*loopLen

	_, pitchA, _ := g.evalNodeParamsOnly(0, idxA2, infoAt(idxA2))
	_, pitchB, _ := g.evalNodeParamsOnly(0, idxB2, infoAt(idxB2))
	if pitchA != 4 || pitchB != 4 {
		t.Fatalf("setup: round-2 pitch A=%v B=%v, want both 4", pitchA, pitchB)
	}

	clickMenuControl(t, g, "delete")
	advanceFrames(g, 3)

	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must close after delete")
	}
	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("group must be gone")
	}

	_, pitchA, _ = g.evalNodeParamsOnly(0, idxA2, infoAt(idxA2))
	_, pitchB, _ = g.evalNodeParamsOnly(0, idxB2, infoAt(idxB2))
	if pitchA != 0 || pitchB != 0 {
		t.Fatalf("post-delete pitch A=%v B=%v, want both back to base 0", pitchA, pitchB)
	}
}

// TestGroupMenuE2E_PlaybackSaveViaRealMarquee: a real marquee-created group
// saved via a real click while playback is running must not panic, must emit
// exactly 1 undo step, and playback must still be running afterward.
func TestGroupMenuE2E_PlaybackSaveViaRealMarquee(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	g.SetPlaying(true)
	t.Cleanup(func() { stopPlaybackForTest(g) })
	advanceFrames(g, 5)

	undoBefore := len(g.undoManager.undo)
	gid := marqueeCreateGroup(t, g, aID, bID)
	if !g.Playing() {
		t.Fatalf("marquee-created group must not stop playback")
	}
	advanceFrames(g, 2)

	clickMenuControl(t, g, "save")
	advanceFrames(g, 3)

	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must close after save")
	}
	if _, ok := g.graph.Group(gid); !ok {
		t.Fatalf("saved group must persist")
	}
	if got := len(g.undoManager.undo) - undoBefore; got != 1 {
		t.Fatalf("save must emit exactly 1 undo step, got %d", got)
	}
	if !g.Playing() {
		t.Fatalf("playback must continue through save")
	}
}

// ─── C. Negative / API scenarios ────────────────────────────────────────────

// TestGroupMenuE2E_ClickAfterAllMembersDeletedNoPanic: once a group's sole
// member is deleted, the model auto-deletes the (now-empty) group and the
// menu closes. A subsequent plain click at the panel's former screen
// location must not panic, must not reopen the menu, and must not
// resurrect the group.
func TestGroupMenuE2E_ClickAfterAllMembersDeletedNoPanic(t *testing.T) {
	g, aID, _ := buildGroupLoopCircuit(t)
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)
	if !g.groupMenu.IsOpen() {
		t.Fatalf("setup: menu must be open")
	}
	panel := g.groupMenu.panel
	px, py := (panel.Min.X+panel.Max.X)/2, (panel.Min.Y+panel.Max.Y)/2

	g.deleteNode(g.nodesByID[aID])
	advanceFrames(g, 2)

	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must close once the sole member is deleted")
	}
	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("group must be gone")
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("click at former panel location panicked: %v", r)
			}
		}()
		gridClick(g, px, py)
	}()
	advanceFrames(g, 2)

	if g.groupMenu.IsOpen() {
		t.Fatalf("group menu must not reopen from a plain click")
	}
	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("deleted group must not resurrect")
	}
}

// TestGroupMenuE2E_SetRuleEnabledOnVanishedGroupIsNoop: deleting the group
// out from under an open menu (bypassing the menu's own delete path) closes
// the menu synchronously via the model's group-changed hook, but leaves
// m.groupID pointing at the vanished id. Calling setRuleEnabled(true) after
// that must be a no-op — no panic, no rule resurrection, no group
// recreation — never duplicate core/model's own negative-path coverage.
func TestGroupMenuE2E_SetRuleEnabledOnVanishedGroupIsNoop(t *testing.T) {
	g, aID, bID := buildGroupLoopCircuit(t)
	gid, err := g.graph.CreateGroup("", []model.NodeID{aID, bID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	g.groupMenu.OpenAt(gid, 100, 100)
	advanceFrames(g, 2)

	if err := g.graph.DeleteGroup(gid); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if g.groupMenu.IsOpen() {
		t.Fatalf("setup: menu must have closed synchronously via the group-changed hook")
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("setRuleEnabled on a vanished group panicked: %v", r)
			}
		}()
		g.groupMenu.setRuleEnabled(true)
	}()
	advanceFrames(g, 2)

	if _, ok := g.graph.Group(gid); ok {
		t.Fatalf("vanished group must not be resurrected by setRuleEnabled")
	}
	if g.groupMenu.IsOpen() {
		t.Fatalf("menu must not reopen")
	}
}
