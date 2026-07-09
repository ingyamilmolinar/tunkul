package ui

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ingyamilmolinar/beatmo/core/model"
	"github.com/ingyamilmolinar/beatmo/internal/i18n"
)

// groupMenuW is the fixed panel width. Unlike NodeSidebar (left-docked, full
// grid height, scrollable), GroupMenu is a compact anchored popup — no
// scroll, no collapse — so a single constant suffices.
const groupMenuW = 300

// GroupMenu is the anchored panel opened after a marquee selection (or from
// a sidebar group chip). It edits ONE group: batch edits fan out over all
// members via SetNodeParams inside one undo group; the rule editor writes
// Rules[0] through the model API.
type GroupMenu struct {
	game    *Game
	open    bool
	groupID model.GroupID
	anchorX int
	anchorY int
	panel   image.Rectangle
	rects   map[string]image.Rectangle
	btns    map[string]*Button
	// rule scratch state mirrors Rules[0] (param + delta + everyN), used as
	// the seed when the group currently has no rule.
	ruleParam model.GroupParam
	// provisional marks a group that was just marquee-created and not yet
	// committed via Save. While true: rule/param edits made through this menu
	// mutate the model (so rings/rules work live) but do NOT emit
	// emitGroupChanged (no undo step for uncommitted settings — see writeRule
	// and applyBatch). Any Close() (close button, click-outside, node click,
	// sidebar chip, vanish, Import) while still provisional silently deletes
	// the group with no emit/undo step, since the creation itself was never
	// committed. Save() emits the single emitGroupCreated undo step for the
	// whole creation, then clears provisional before closing.
	provisional bool
}

func NewGroupMenu(g *Game) *GroupMenu {
	return &GroupMenu{
		game:      g,
		rects:     map[string]image.Rectangle{},
		btns:      map[string]*Button{},
		ruleParam: model.GroupParamPitch,
	}
}

// OpenAt opens the menu for group id anchored near (x, y), clamped on-screen.
// Non-provisional: the group is treated as already committed (existing
// callers — sidebar chip reopen, JS export, etc.).
func (m *GroupMenu) OpenAt(id model.GroupID, x, y int) {
	m.openAt(id, x, y, false)
}

// OpenAtProvisional opens the menu for a freshly marquee-created group that
// has not yet been committed. The marquee release path calls this instead of
// OpenAt. See the provisional field's doc comment for the full lifecycle.
func (m *GroupMenu) OpenAtProvisional(id model.GroupID, x, y int) {
	m.openAt(id, x, y, true)
}

// openAt is the shared body for OpenAt/OpenAtProvisional. provisional must be
// set BEFORE layoutAt runs so the initial layout already includes (or
// omits) the "save" control — a caller reading m.rects/m.btns right after
// OpenAt(Provisional) returns must see the correct state without waiting for
// a subsequent Draw()/HandleInput() to re-run layoutAt.
func (m *GroupMenu) openAt(id model.GroupID, x, y int, provisional bool) {
	if _, ok := m.game.graph.Group(id); !ok {
		return
	}
	// A second marquee/open request arriving while a still-open provisional
	// group's menu hasn't been Saved would otherwise overwrite groupID
	// without ever discarding the first group: it leaks into g.graph.Groups
	// uncommitted, menuless, and exportable. Reuse the same silent-discard
	// Close() uses (delete group, ignore ErrGroupNotFound, no emits) so the
	// two paths can't drift.
	if m.open && m.provisional && m.groupID != id {
		m.discardProvisional()
	}
	m.open = true
	m.provisional = provisional
	m.groupID = id
	if grp, ok := m.game.graph.Group(id); ok && len(grp.Rules) > 0 {
		m.ruleParam = grp.Rules[0].Param
	}
	m.anchorX, m.anchorY = x, y
	m.layoutAt(x, y)
	m.game.dispatcherDirty = true
}

// discardProvisional silently deletes the currently-referenced provisional
// group, if any: DeleteGroup with no emit and no undo step, since the
// creation was never committed. Idempotent and safe to call re-entrantly
// (the model's group-changed hook can call back into Close() while
// DeleteGroup is still unwinding — see onGroupsChanged): the "group still
// exists" check makes a nested call a no-op. Shared by Close() (menu closing
// on an uncommitted group) and openAt() (a second marquee/open arriving
// while a still-open provisional menu would otherwise orphan the first
// group) so the two discard paths can't drift apart.
func (m *GroupMenu) discardProvisional() {
	if !m.provisional {
		return
	}
	if _, ok := m.game.graph.Group(m.groupID); ok {
		_ = m.game.graph.DeleteGroup(m.groupID) // silent: ignore ErrGroupNotFound races
	}
	m.provisional = false
}

// Close closes the menu. If the group is still provisional (never saved),
// this silently discards it via discardProvisional. Idempotent and safe to
// call re-entrantly (see discardProvisional's doc comment).
func (m *GroupMenu) Close() {
	m.discardProvisional()
	m.open = false
	m.btns = map[string]*Button{}
	m.rects = map[string]image.Rectangle{}
	m.game.dispatcherDirty = true
}

func (m *GroupMenu) IsOpen() bool           { return m.open }
func (m *GroupMenu) GroupID() model.GroupID { return m.groupID }
func (m *GroupMenu) Hit(x, y int) bool      { return m.open && image.Pt(x, y).In(m.panel) }

// ─── InputHandler interface ────────────────────────────────────────────────

func (m *GroupMenu) InputBounds() image.Rectangle { return m.panel }
func (m *GroupMenu) ZIndex() int                  { return 210 }

// HandleWheel: the menu carries no scrollable content, so wheel events pass
// through untouched.
func (m *GroupMenu) HandleWheel(_, _, _ int) InputResult { return InputIgnored }

// Capturing: no drag-capture state (unlike the sidebar's scroll thumb).
func (m *GroupMenu) Capturing() bool { return false }

// HandleInput processes mouse/touch input within the menu panel.
func (m *GroupMenu) HandleInput(x, y int, pressed bool) InputResult {
	if !m.open {
		return InputIgnored
	}
	// Refresh rects/buttons against the live model state (rule on/off, param,
	// member count) before hit-testing, mirroring NodeSidebar.layout() being
	// re-run at the top of its HandleInput.
	m.layoutAt(m.anchorX, m.anchorY)

	pt := image.Pt(x, y)
	if !pt.In(m.panel) {
		// The dispatcher only calls HandleInput for points inside
		// InputBounds() (== m.panel), so this branch is defensive: it never
		// fires through the normal dispatch path, but any direct caller that
		// hands us an outside point gets the same click-outside-closes
		// contract as the rest of the UI.
		if pressed {
			m.Close()
		}
		return InputIgnored
	}

	order := []string{
		"save", "close", "vol-", "vol+", "pit-", "pit+", "dur-", "dur+",
		"ruleonoff", "param", "delta-", "delta+", "n-", "n+", "delete",
	}
	for _, id := range order {
		btn, ok := m.btns[id]
		if !ok {
			continue
		}
		r, ok := m.rects[id]
		if !ok || r.Empty() {
			continue
		}
		btn.SetRect(r)
		if btn.HandleInputResult(x, y, pressed) != InputIgnored {
			return InputConsumed
		}
	}
	// Opaque to z: any press inside the panel that landed on no control is
	// still consumed so it can't fall through to the grid beneath.
	return InputConsumed
}

// group returns the live group; closes the menu if it vanished (auto-delete
// on last-member removal, or explicit DeleteGroup).
func (m *GroupMenu) group() (model.NodeGroup, bool) {
	grp, ok := m.game.graph.Group(m.groupID)
	if !ok && m.open {
		m.Close()
	}
	return grp, ok
}

// titleText resolves the header title shown in Draw: the group's own name
// when it has one, falling back to the generic i18n title for unnamed
// groups or a vanished group (spec §4 gap — the header used to always show
// the static i18n title regardless of the group's actual Name).
func (m *GroupMenu) titleText() string {
	grp, ok := m.group()
	if !ok || grp.Name == "" {
		return i18n.T(i18n.KeyGroupMenuTitle)
	}
	return grp.Name
}

// applyBatch applies a one-shot edit to every member as ONE undo step.
// action ∈ {"vol-","vol+","pit-","pit+","dur-","dur+"}.
func (m *GroupMenu) applyBatch(action string) {
	g := m.game
	grp, ok := m.group()
	if !ok {
		return
	}
	g.enqueueUI(func() {
		beginUndoGroup("group edit")
		defer endUndoGroup()
		for _, id := range grp.NodeIDs {
			mn, ok := g.graph.GetNodeByID(id)
			if !ok {
				continue
			}
			p := mn.Params
			switch action {
			case "vol-":
				p.Volume = clampF64(p.Volume-0.1, 0, 2)
			case "vol+":
				p.Volume = clampF64(p.Volume+0.1, 0, 2)
			case "pit-":
				p.Pitch = clampF64(p.Pitch-1, -24, 24)
			case "pit+":
				p.Pitch = clampF64(p.Pitch+1, -24, 24)
			case "dur-":
				p.Duration = clampF64(p.Duration-0.1, 0.1, 4)
			case "dur+":
				p.Duration = clampF64(p.Duration+0.1, 0.1, 4)
			}
			g.graph.SetNodeParams(id, p)
			g.notifyPredictorNode(id)
			emitNodeParamsChanged(id, p)
		}
		// Node edits are real regardless of the group's provisional fate (the
		// per-node emits above always fire), but the group-level "something
		// about this group changed" emit is only meaningful once the group
		// itself is committed.
		if !m.provisional {
			emitGroupChanged(grp.ID)
		}
	})
}

// currentRule returns Rules[0] or a default-shaped disabled rule.
func (m *GroupMenu) currentRule() (model.GroupRule, bool) {
	grp, ok := m.group()
	if !ok || len(grp.Rules) == 0 {
		return model.GroupRule{Param: m.ruleParam, Delta: 1, EveryN: 1}, false
	}
	return grp.Rules[0], true
}

func (m *GroupMenu) writeRule(r model.GroupRule, enabled bool) {
	g := m.game
	id := m.groupID
	g.enqueueUI(func() {
		// While provisional, rule edits still mutate the model (so rings/
		// rules preview correctly) but must not emit — there is no undo step
		// for settings on a group that hasn't been Saved yet.
		if !enabled {
			if err := g.graph.ClearGroupRules(id); err == nil && !m.provisional {
				emitGroupChanged(id)
			}
			return
		}
		if err := g.graph.SetGroupRules(id, []model.GroupRule{r}); err == nil && !m.provisional {
			emitGroupChanged(id)
		}
	})
}

func (m *GroupMenu) setRuleEnabled(on bool) {
	r, _ := m.currentRule()
	m.writeRule(r, on)
}

func (m *GroupMenu) stepRuleDelta(dir int) {
	r, _ := m.currentRule()
	step := 1.0
	if r.Param != model.GroupParamPitch {
		step = 0.05 // multiplier-offset step for vol/dur
	}
	r.Delta += float64(dir) * step
	m.writeRule(r, true) // stepping delta implicitly enables
}

func (m *GroupMenu) stepRuleEveryN(dir int) {
	r, _ := m.currentRule()
	r.EveryN += dir
	if r.EveryN < 1 {
		r.EveryN = 1
	}
	m.writeRule(r, true) // stepping every-N implicitly enables
}

func (m *GroupMenu) cycleRuleParam() {
	r, enabled := m.currentRule()
	switch r.Param {
	case model.GroupParamPitch:
		r.Param = model.GroupParamVolume
	case model.GroupParamVolume:
		r.Param = model.GroupParamDuration
	default:
		r.Param = model.GroupParamPitch
	}
	// Reset clamps so normalizeGroupRules re-derives per-param defaults.
	r.Min, r.Max = 0, 0
	m.ruleParam = r.Param
	m.writeRule(r, enabled)
}

// save commits a provisional group: emits the single emitGroupCreated undo
// step for the whole creation (membership + any rule/batch edits already
// applied to the model while provisional), clears provisional, then closes
// the menu. Clearing provisional BEFORE Close() is required — otherwise
// Close() would treat the just-saved group as still-uncommitted and silently
// delete it.
func (m *GroupMenu) save() {
	g := m.game
	id := m.groupID
	g.enqueueUI(func() {
		if grp, ok := g.graph.Group(id); ok {
			emitGroupCreated(id, grp.Name, grp.NodeIDs)
		}
		m.provisional = false
		m.Close()
	})
}

func (m *GroupMenu) deleteGroup() {
	g := m.game
	id := m.groupID
	g.enqueueUI(func() {
		if m.provisional {
			// Silent discard: identical to closing without Save — the
			// creation was never committed, so no emit/undo step. Clear
			// provisional BEFORE Close() so Close() doesn't attempt a second
			// DeleteGroup on a group we just removed.
			m.provisional = false
			_ = g.graph.DeleteGroup(id)
			m.Close()
			return
		}
		if err := g.graph.DeleteGroup(id); err == nil {
			emitGroupDeleted(id)
		}
		m.Close()
	})
}

// ─── Layout ────────────────────────────────────────────────────────────────

// groupMenuFullRows is the number of full-height control rows in the
// compressed layout: vol, pit, dur steppers, the combined rule on/off+param
// row, the combined delta+everyN stepper row, and delete.
const groupMenuFullRows = 6

// groupMenuHalfRows is the number of half-height section-label rows: the
// "Edit all" batch label and the "Rule" label. Each is drawn as small inline
// text above its control block rather than consuming a full button row (see
// layoutAt's row list) — folding these (and the old standalone "members"
// row, merged into the header by headerLine) out of the fixed row count is
// what makes the panel fit the grid pane without needing to shrink rows at
// ordinary window sizes.
const groupMenuHalfRows = 2

// computeGroupMenuRowSizing derives the row height/gap pair that makes the
// GroupMenu's full row stack fit within maxH (the grid pane height). Starts
// at the sidebar's ideal sizing (sidebarBtnH/sidebarGap) and shrinks gap
// first, then row height, down to floors (24px row / 3px gap — still
// legible/tappable). If a window is so small even the floors overflow, keeps
// shrinking evenly below the floors as a last resort: the panel must NEVER
// overflow the grid pane, like the node sidebar.
func computeGroupMenuRowSizing(maxH int) (rowH, gap int) {
	rowH, gap = sidebarBtnH, sidebarGap
	contentH := func(rh, gp int) int {
		halfH := rh / 2
		return sidebarPad*2 + sidebarHeaderH + gp +
			groupMenuHalfRows*(halfH+gp) + groupMenuFullRows*(rh+gp)
	}
	for contentH(rowH, gap) > maxH && (rowH > 24 || gap > 3) {
		switch {
		case gap > 3:
			gap--
		case rowH > 24:
			rowH--
		}
	}
	for contentH(rowH, gap) > maxH && (rowH > 0 || gap > 0) {
		if rowH > 0 {
			rowH--
		}
		if gap > 0 {
			gap--
		}
	}
	return rowH, gap
}

// layoutAt recomputes rects for a fixed-width panel anchored near (x, y),
// clamped inside the grid pane on BOTH axes (position and size — see
// computeGroupMenuRowSizing), then wires the buttons against the fresh rects
// and current model state. Reuses the NodeSidebar's pad/header constants
// (sidebarPad, sidebarHeaderH, ...) so the two menus share the same keycap
// rhythm — GroupMenu is deliberately NOT a scrollable clone, just a compact
// adaptive stack of rows that always fits the pane, like the sidebar does.
func (m *GroupMenu) layoutAt(x, y int) {
	g := m.game
	m.rects = map[string]image.Rectangle{}

	gridW := g.split.GridW(g.winW)
	gridH := g.split.GridH(g.winH)

	w := groupMenuW
	if w > gridW {
		w = gridW
	}
	if w < 0 {
		w = 0
	}

	rowH, gap := computeGroupMenuRowSizing(gridH)
	halfH := rowH / 2

	// Rows, top to bottom: header (title + live member count merged into one
	// line — see headerLine), a half-row "Edit all" label, 3 batch stepper
	// rows, a half-row "Rule" label, the combined rule on/off+param row, the
	// combined delta+everyN stepper row, delete.
	h := sidebarPad*2 + sidebarHeaderH + gap +
		groupMenuHalfRows*(halfH+gap) + groupMenuFullRows*(rowH+gap)

	ox, oy := x, y
	if ox+w > gridW {
		ox = gridW - w
	}
	if ox < 0 {
		ox = 0
	}
	if oy+h > gridH {
		oy = gridH - h
	}
	if oy < 0 {
		oy = 0
	}

	panel := image.Rect(ox, oy, ox+w, oy+h)
	m.panel = panel
	m.rects["panel"] = panel

	headerR := image.Rect(panel.Min.X+sidebarPad, panel.Min.Y+sidebarPad, panel.Max.X-sidebarPad, panel.Min.Y+sidebarPad+sidebarHeaderH)
	m.rects["title"] = headerR
	closeSize := sidebarBtnH
	m.rects["close"] = image.Rect(headerR.Max.X-closeSize, headerR.Min.Y, headerR.Max.X, headerR.Min.Y+closeSize)

	// Save button: header row, immediately left of close, ONLY while
	// provisional. Non-provisional (or once saved) leaves no "save" rect, so
	// ensureButtons()/mk() prunes the button — same conditional-control
	// pattern as every other rect-absent-means-no-control case in this file.
	if m.provisional {
		label := i18n.T(i18n.KeyGroupSave)
		textW := int(float64(TextWidth(label)) * sidebarLabelScale())
		saveW := textW + 4*SpaceXS
		if minW := closeSize * 2; saveW < minW {
			saveW = minW
		}
		saveX2 := m.rects["close"].Min.X - sidebarGap
		saveX1 := saveX2 - saveW
		if saveX1 < headerR.Min.X {
			saveX1 = headerR.Min.X
		}
		m.rects["save"] = image.Rect(saveX1, headerR.Min.Y, saveX2, headerR.Min.Y+closeSize)
	} else {
		delete(m.rects, "save")
	}

	yy := headerR.Max.Y + gap
	rowRect := func(id string) image.Rectangle {
		r := image.Rect(panel.Min.X+sidebarPad, yy, panel.Max.X-sidebarPad, yy+rowH)
		m.rects[id] = r
		yy += rowH + gap
		return r
	}
	// halfRowRect is a compact section-label row (see groupMenuHalfRows'
	// doc comment) — half the height of a control row, no buttons.
	halfRowRect := func(id string) image.Rectangle {
		r := image.Rect(panel.Min.X+sidebarPad, yy, panel.Max.X-sidebarPad, yy+halfH)
		m.rects[id] = r
		yy += halfH + gap
		return r
	}
	// stepRowAt wires a −/value/+ trio into an arbitrary rect r (either a
	// full-width row from rowRect, or a half-width split of one — see the
	// combined delta+everyN row below).
	stepRowAt := func(base string, r image.Rectangle) {
		pillW := Profile().DensityValues().SidebarValuePillW
		plusX := r.Max.X - sidebarIncBtnW
		pillX := plusX - gap - pillW
		minusX := pillX - gap - sidebarIncBtnW
		incH := sidebarIncBtnH
		if incH > rowH {
			incH = rowH
		}
		btnY := r.Min.Y + (rowH-incH)/2
		m.rects[base+"-"] = image.Rect(minusX, btnY, minusX+sidebarIncBtnW, btnY+incH)
		m.rects[base+"val"] = image.Rect(pillX, btnY, pillX+pillW, btnY+incH)
		m.rects[base+"+"] = image.Rect(plusX, btnY, plusX+sidebarIncBtnW, btnY+incH)
	}
	stepRow := func(base string) image.Rectangle {
		r := rowRect(base)
		stepRowAt(base, r)
		return r
	}

	halfRowRect("batchlabel")
	stepRow("vol")
	stepRow("pit")
	stepRow("dur")
	halfRowRect("rulelabel")

	ruleR := rowRect("ruleonoff")
	halfW := (ruleR.Dx() - gap) / 2
	m.rects["ruleonoff"] = image.Rect(ruleR.Min.X, ruleR.Min.Y, ruleR.Min.X+halfW, ruleR.Max.Y)
	m.rects["param"] = image.Rect(ruleR.Min.X+halfW+gap, ruleR.Min.Y, ruleR.Max.X, ruleR.Max.Y)

	// Delta and Every-N used to be two separate stepper rows; combined into
	// one row (delta left half, everyN right half) as part of the layout-fit
	// compression (task-groupsave layout-fit fix).
	deltanR := rowRect("deltan")
	dHalfW := (deltanR.Dx() - gap) / 2
	deltaR := image.Rect(deltanR.Min.X, deltanR.Min.Y, deltanR.Min.X+dHalfW, deltanR.Max.Y)
	nR := image.Rect(deltanR.Min.X+dHalfW+gap, deltanR.Min.Y, deltanR.Max.X, deltanR.Max.Y)
	m.rects["delta"] = deltaR
	m.rects["n"] = nR
	stepRowAt("delta", deltaR)
	stepRowAt("n", nR)

	rowRect("delete")

	m.ensureButtons()
}

// ─── Button wiring ─────────────────────────────────────────────────────────

func (m *GroupMenu) ensureButtons() {
	if _, ok := m.group(); !ok {
		return
	}
	rule, ruleOn := m.currentRule()

	mk := func(id, label string, icon string, onClick func()) {
		r, ok := m.rects[id]
		if !ok || r.Empty() {
			delete(m.btns, id)
			return
		}
		if _, exists := m.btns[id]; !exists {
			m.btns[id] = NewButton(label, DropdownStyle, onClick)
		} else {
			m.btns[id].Text = label
			m.btns[id].OnClick = onClick
		}
		b := m.btns[id]
		b.SetRect(r)
		b.ConsumeOnPress = true
		b.TextScale = sidebarLabelScale()
		b.Icon = icon
		if icon != "" {
			b.Text = ""
			b.IconColor = colTextPrimary
		}
	}

	mk("close", "", "close", func() { m.Close() })
	if b := m.btns["close"]; b != nil {
		b.IconColor = closeIconColor()
	}

	mk("save", i18n.T(i18n.KeyGroupSave), "", func() { m.save() })

	mk("vol-", "", "minus", func() { m.applyBatch("vol-") })
	mk("vol+", "", "plus", func() { m.applyBatch("vol+") })
	mk("pit-", "", "minus", func() { m.applyBatch("pit-") })
	mk("pit+", "", "plus", func() { m.applyBatch("pit+") })
	mk("dur-", "", "minus", func() { m.applyBatch("dur-") })
	mk("dur+", "", "plus", func() { m.applyBatch("dur+") })

	ruleToggleLabel := i18n.T(i18n.KeyGroupRuleOff)
	if ruleOn {
		ruleToggleLabel = i18n.T(i18n.KeyGroupRuleOn)
	}
	mk("ruleonoff", ruleToggleLabel, "", func() { m.setRuleEnabled(!ruleOn) })
	mk("param", groupParamLabel(rule.Param), "", func() { m.cycleRuleParam() })

	mk("delta-", "", "minus", func() { m.stepRuleDelta(-1) })
	mk("delta+", "", "plus", func() { m.stepRuleDelta(1) })
	mk("n-", "", "minus", func() { m.stepRuleEveryN(-1) })
	mk("n+", "", "plus", func() { m.stepRuleEveryN(1) })

	mk("delete", i18n.T(i18n.KeyGroupDelete), "", func() { m.deleteGroup() })
	if b := m.btns["delete"]; b != nil {
		b.TextColor = colError
	}
}

// groupParamRectBase maps a model.GroupParam to the batch stepper row's rect
// id prefix ("vol"/"pit"/"dur" — see layoutAt's stepRow calls).
func groupParamRectBase(p model.GroupParam) string {
	switch p {
	case model.GroupParamVolume:
		return "vol"
	case model.GroupParamDuration:
		return "dur"
	default:
		return "pit"
	}
}

// groupParamValue reads the live param value off a node's Params for the
// given GroupParam.
func groupParamValue(p model.NodeParams, param model.GroupParam) float64 {
	switch param {
	case model.GroupParamVolume:
		return p.Volume
	case model.GroupParamDuration:
		return p.Duration
	default:
		return p.Pitch
	}
}

// formatGroupParamValue formats a uniform param value exactly like the node
// sidebar does (game_node_sidebar.go): pitch as a signed semitone integer,
// duration as a multiplier, volume as the dB readout (formatNodeVolumeDb —
// shared with the sidebar so "0 dB"/"Muted" never drift between the two
// surfaces).
func formatGroupParamValue(param model.GroupParam, v float64) string {
	switch param {
	case model.GroupParamVolume:
		return formatNodeVolumeDb(v)
	case model.GroupParamDuration:
		return fmt.Sprintf("%.2fx", v)
	default:
		return fmt.Sprintf("%+d", int(v))
	}
}

// pillFitOrFallback returns label if it fits inside the named rect's width
// (with the same everyNPillLabelFitMargin slack everyNPillLabel uses), else
// fallback. Shared fit-or-fallback contract for every batch/rule value pill.
func (m *GroupMenu) pillFitOrFallback(rectID, label, fallback string) string {
	pillW := 0
	if r, ok := m.rects[rectID]; ok {
		pillW = r.Dx()
	}
	if pillW > 0 && StyledTextWidth(label, RoleBody) > pillW-everyNPillLabelFitMargin {
		return fallback
	}
	return label
}

// batchValueText resolves the batch-edit pill's display text for param: the
// formatted value (matching the node sidebar's own formatting) when every
// live member of the group agrees, the localized "mixed" label when they
// differ (falling back to "±" if the localized label doesn't fit the pill —
// same fit contract as everyNPillLabel), or "-" when the group has vanished
// or has no members. Fixes the pre-fix hardcoded "-" placeholder that gave
// zero feedback for successful batch edits (Draw previously never read live
// model state for these three pills).
func (m *GroupMenu) batchValueText(param model.GroupParam) string {
	grp, ok := m.group()
	if !ok || len(grp.NodeIDs) == 0 {
		return "-"
	}
	var first float64
	have := false
	uniform := true
	for _, id := range grp.NodeIDs {
		mn, ok := m.game.graph.GetNodeByID(id)
		if !ok {
			continue
		}
		v := groupParamValue(mn.Params, param)
		if !have {
			first, have = v, true
			continue
		}
		if v != first {
			uniform = false
		}
	}
	if !have {
		return "-"
	}
	if !uniform {
		base := groupParamRectBase(param)
		return m.pillFitOrFallback(base+"val", i18n.T(i18n.KeyGroupMixed), "±")
	}
	return formatGroupParamValue(param, first)
}

// groupParamLabel resolves the display label for a GroupParam via i18n.
func groupParamLabel(p model.GroupParam) string {
	switch p {
	case model.GroupParamVolume:
		return i18n.T(i18n.KeyGroupParamVolume)
	case model.GroupParamDuration:
		return i18n.T(i18n.KeyGroupParamDuration)
	default:
		return i18n.T(i18n.KeyGroupParamPitch)
	}
}

// ─── Draw ──────────────────────────────────────────────────────────────────

func (m *GroupMenu) Draw(dst *ebiten.Image) {
	if !m.open {
		return
	}
	m.layoutAt(m.anchorX, m.anchorY)
	grp, ok := m.group()
	if !ok {
		return
	}

	panel := m.panel
	drawPanelShadow(dst, panel, 6)
	drawRoundedRect(dst, panel, colPanelBG, RadiusLG, true)
	drawRoundedRect(dst, panel, WithAlpha(genColorSidebarSectionBg, genAlphaSidebarSection), RadiusLG, false)

	// Title: the group's own name (or the i18n fallback — see titleText, spec
	// §4) merged with the live member count into one header line (the old
	// standalone "members" row was folded in here — task-groupsave
	// layout-fit fix). Truncated to fit beside the close/save buttons.
	if r, ok := m.rects["title"]; ok {
		title := m.headerLine(grp)
		ty := r.Min.Y + (sidebarHeaderH-StyledTextHeight(RoleSectionHeader))/2
		DrawTextStyled(dst, title, r.Min.X, ty, RoleSectionHeader, colTextPrimary)
	}
	m.drawBtn(dst, "close")
	m.drawBtn(dst, "save")

	// Batch section — "Edit all" is now a half-row inline label (no full
	// button row) sitting just above the three steppers.
	if r, ok := m.rects["batchlabel"]; ok {
		ty := r.Min.Y + (r.Dy()-StyledTextHeight(RoleCaption))/2
		DrawTextStyled(dst, i18n.T(i18n.KeyGroupBatch), r.Min.X, ty, RoleCaption, colTextSecondary)
	}
	// The three batch-edit steppers apply a relative +/- nudge to every
	// member: the pill shows the live formatted value when every member
	// agrees, or the localized "mixed" label when they don't (batchValueText)
	// — replacing the old hardcoded "-" placeholder that gave zero feedback
	// for successful batch edits.
	m.drawStepRow(dst, "vol", i18n.T(i18n.KeyGroupParamVolume), m.batchValueText(model.GroupParamVolume))
	m.drawStepRow(dst, "pit", i18n.T(i18n.KeyGroupParamPitch), m.batchValueText(model.GroupParamPitch))
	m.drawStepRow(dst, "dur", i18n.T(i18n.KeyGroupParamDuration), m.batchValueText(model.GroupParamDuration))

	// Rule section — "Rule" is likewise a half-row inline label above the
	// combined on/off+param row.
	if r, ok := m.rects["rulelabel"]; ok {
		ty := r.Min.Y + (r.Dy()-StyledTextHeight(RoleCaption))/2
		DrawTextStyled(dst, i18n.T(i18n.KeyGroupRule), r.Min.X, ty, RoleCaption, colTextSecondary)
	}
	m.drawBtn(dst, "ruleonoff")
	m.drawBtn(dst, "param")

	// Delta and Every-N share one row (delta left half, everyN right half —
	// task-groupsave layout-fit fix); labels stay empty (the values are
	// self-explanatory: a signed delta and a "every N rounds" pill).
	rule, _ := m.currentRule()
	m.drawStepRow(dst, "delta", "", fmt.Sprintf("%+.2f", rule.Delta))
	m.drawStepRow(dst, "n", "", m.everyNPillLabel(rule.EveryN))

	m.drawBtn(dst, "delete")
}

// everyNPillLabelFitMargin is the slack (px) subtracted from the pill's
// width before comparing against the candidate label's measured width — a
// small buffer so the text never touches the pill's rounded border.
const everyNPillLabelFitMargin = 4

// everyNPillLabel resolves the Every-N pill's display text: the short form
// ("Every %d" via KeyGroupEveryNShort — see task-groupsave Every-N row fix)
// when it fits the pill rect, else the bare number. The long-form
// KeyGroupEveryN sentence ("Every %d rounds") is too wide for the half-row
// pill at every density and used to overflow past the pill's left edge,
// painting over the "n-" button drawn just before it in drawStepRow (the
// button's rect was always present and clickable — only the paint order made
// it look missing).
func (m *GroupMenu) everyNPillLabel(n int) string {
	label := fmt.Sprintf(i18n.T(i18n.KeyGroupEveryNShort), n)
	pillW := 0
	if r, ok := m.rects["nval"]; ok {
		pillW = r.Dx()
	}
	if pillW > 0 && StyledTextWidth(label, RoleBody) > pillW-everyNPillLabelFitMargin {
		return fmt.Sprintf("%d", n)
	}
	return label
}

// headerLine composes the header's display text: the group's title (see
// titleText) plus the live member count (formerly its own "members" row),
// truncated to fit the header rect minus whatever close/save buttons occupy
// on its right edge so a long group name never overlaps or overflows past
// them.
func (m *GroupMenu) headerLine(grp model.NodeGroup) string {
	full := m.titleText() + " · " + fmt.Sprintf(i18n.T(i18n.KeyGroupMembers), len(grp.NodeIDs))
	headerR, ok := m.rects["title"]
	if !ok {
		return full
	}
	rightEdge := headerR.Max.X
	if r, ok := m.rects["save"]; ok {
		rightEdge = r.Min.X
	} else if r, ok := m.rects["close"]; ok {
		rightEdge = r.Min.X
	}
	avail := rightEdge - headerR.Min.X - sidebarGap
	if avail <= 0 {
		return full
	}
	return truncateName(full, avail, sidebarLabelScale())
}

// drawStepRow draws a batch/rule stepper row's left-side label (when non-
// empty) and the −/value/+ trio.
func (m *GroupMenu) drawStepRow(dst *ebiten.Image, base, label, value string) {
	if r, ok := m.rects[base]; ok && label != "" {
		ty := r.Min.Y + (r.Dy()-StyledTextHeight(RoleCaption))/2
		DrawTextStyled(dst, label, r.Min.X, ty, RoleCaption, colTextSecondary)
	}
	m.drawBtn(dst, base+"-")
	if r, ok := m.rects[base+"val"]; ok {
		drawRecessedPill(dst, r, value)
	}
	m.drawBtn(dst, base+"+")
}

func (m *GroupMenu) drawBtn(dst *ebiten.Image, id string) {
	b := m.btns[id]
	if b == nil {
		return
	}
	b.Draw(dst)
}
