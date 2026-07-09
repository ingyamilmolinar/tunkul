package ui

import "github.com/ingyamilmolinar/beatmo/core/model"

// refreshGroupIndex rebuilds the immutable group-rule index from the graph.
// Called from the graph's group-changed hook (UI thread) and after import.
func (g *Game) refreshGroupIndex() {
	idx := model.BuildGroupIndex(g.graph.Groups)
	g.groupIdxMu.Lock()
	g.groupIdx = idx
	g.groupIdxMu.Unlock()
}

// groupIndexSnapshot returns the current immutable index (may be nil).
func (g *Game) groupIndexSnapshot() *model.GroupIndex {
	g.groupIdxMu.RLock()
	defer g.groupIdxMu.RUnlock()
	return g.groupIdx
}

// roundAtRowIdx maps an absolute subdivision index to the row's loop round.
// Non-loop rows are always round 0. Mirrors game_graph_beat_index.go's loop
// math; callers hold seqMu (sequencer schedule / Update), matching how
// isLoopByRow is read elsewhere.
func (g *Game) roundAtRowIdx(row, idx int) int {
	if row < 0 || row >= len(g.isLoopByRow) || !g.isLoopByRow[row] {
		return 0
	}
	if row >= len(g.loopStartByRow) || row >= len(g.beatInfosByRow) {
		return 0
	}
	start := g.loopStartByRow[row]
	loopLen := len(g.beatInfosByRow[row]) - start
	return model.GroupRound(idx, start, loopLen)
}

// onGroupsChanged reacts to any group mutation: rebuild the index and, while
// playing, drop in-flight audio + advance the parity generation so the new
// rule takes effect from the next re-predicted trigger (same mechanism as the
// node-param-change hook in game_new.go).
func (g *Game) onGroupsChanged() {
	g.refreshGroupIndex()
	if g.importing {
		return
	}
	if g.groupMenu != nil && g.groupMenu.IsOpen() {
		if _, ok := g.graph.Group(g.groupMenu.GroupID()); !ok {
			g.groupMenu.Close()
		}
	}
	if g.Playing() {
		g.audioGen.Add(1)
		g.bumpParityGen("group-rule-change", structuralMutationOptions{
			SkipPathsDirty:     true,
			SkipPathChangeMark: true,
			SkipBufferClear:    true,
		})
		g.parityMu.Lock()
		g.parityAudio = nil
		g.parityAudioMaxIdx = nil
		g.paritySeqDecisions = make(map[int]map[int]paritySeqDecision)
		g.parityMu.Unlock()
		g.ClearParityMismatches()
	}
}
