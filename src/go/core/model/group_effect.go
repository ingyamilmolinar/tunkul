package model

import "sort"

// GroupIndex is an immutable per-node view over group rules + membership,
// rebuilt on group mutation and read at trigger time. Readers must treat it
// as frozen; mutations swap in a freshly built index.
type GroupIndex struct {
	rulesByNode  map[NodeID][]GroupRule
	memberByNode map[NodeID]bool
}

// BuildGroupIndex flattens groups into a per-node rule list. Deterministic:
// groups are folded in ascending GroupID order (stacking is order-independent
// mathematically, but keep iteration stable anyway). Nil-safe.
func BuildGroupIndex(groups map[GroupID]*NodeGroup) *GroupIndex {
	idx := &GroupIndex{
		rulesByNode:  map[NodeID][]GroupRule{},
		memberByNode: map[NodeID]bool{},
	}
	ids := make([]GroupID, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		grp := groups[id]
		for _, n := range grp.NodeIDs {
			idx.memberByNode[n] = true
			if len(grp.Rules) > 0 {
				idx.rulesByNode[n] = append(idx.rulesByNode[n], grp.Rules...)
			}
		}
	}
	return idx
}

// Member reports whether n belongs to any group (for rendering rings).
func (idx *GroupIndex) Member(n NodeID) bool {
	if idx == nil {
		return false
	}
	return idx.memberByNode[n]
}

// HasRules reports whether any group rule affects n.
func (idx *GroupIndex) HasRules(n NodeID) bool {
	if idx == nil {
		return false
	}
	return len(idx.rulesByNode[n]) > 0
}

// GroupRound maps an absolute subdivision index to a loop round. Indices at
// or before loopStart, and non-loop rows (loopLen <= 0), are round 0. This is
// the same cycle math the mute-node every_n_triggers logic uses.
func GroupRound(idx, loopStart, loopLen int) int {
	if loopLen <= 0 || idx <= loopStart {
		return 0
	}
	return (idx - loopStart) / loopLen
}

func clampGroupF(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// GroupEffect returns the stacked rule effect for n at round: pitchDelta in
// semitones (sums across rules), volMul and durMul as multipliers (multiply
// across rules; each rule contributes clamp(1 + Delta×steps, Min, Max)).
// Pure. Identity is (0, 1, 1).
func GroupEffect(idx *GroupIndex, n NodeID, round int) (float64, float64, float64) {
	pitchDelta, volMul, durMul := 0.0, 1.0, 1.0
	if idx == nil {
		return pitchDelta, volMul, durMul
	}
	for _, r := range idx.rulesByNode[n] {
		steps := 0
		if round > 0 && r.EveryN > 0 {
			steps = round / r.EveryN
		}
		// Skip rule contribution when steps == 0 to preserve identity (0 for pitch, 1 for vol/dur)
		if steps == 0 {
			continue
		}
		switch r.Param {
		case GroupParamPitch:
			pitchDelta += clampGroupF(r.Delta*float64(steps), r.Min, r.Max)
		case GroupParamVolume:
			volMul *= clampGroupF(1+r.Delta*float64(steps), r.Min, r.Max)
		case GroupParamDuration:
			durMul *= clampGroupF(1+r.Delta*float64(steps), r.Min, r.Max)
		}
	}
	return pitchDelta, volMul, durMul
}
