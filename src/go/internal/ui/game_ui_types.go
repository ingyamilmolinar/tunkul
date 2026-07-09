package ui

import (
	"time"

	"github.com/ingyamilmolinar/beatmo/core/model"
)

/* ───────────────────────── data types ───────────────────────── */

type uiNode struct {
	ID       model.NodeID
	I, J     int     // grid indices
	X, Y     float64 // cached world coords (grid.Unit()*I, grid.Unit()*J)
	Selected bool
	Start    bool
}

func (n *uiNode) Bounds(scale float64) (x1, y1, x2, y2 float64) {
	halfSize := float64(NodeSpriteSize) / 2.0 * scale
	return n.X - halfSize, n.Y - halfSize, n.X + halfSize, n.Y + halfSize
}

type uiEdge struct {
	A, B  *uiNode
	t     float64 // connection animation progress 0..1
	pulse float64 // direction pulse progress (-1 inactive)
}

type dragLink struct {
	from     *uiNode
	toX, toY float64
	active   bool
}

// marqueeDrag tracks the Shift+drag-from-empty-space group-selection gesture.
// Screen-space (px), not world/grid coords — the marquee is a pure screen
// overlay drawn before any camera transform, mirroring how dragLink stores
// toX/toY in grid-snapped world coords for its own different purpose.
type marqueeDrag struct {
	active         bool
	startX, startY int // screen px at press
	curX, curY     int // screen px, updated each frame while held
}

type pulse struct {
	x1, y1, x2, y2           float64
	t, speed                 float64
	from, to                 *uiNode
	fromBeatInfo, toBeatInfo model.BeatInfo
	path                     []model.BeatInfo
	pathIdx                  int
	lastIdx                  int
	row                      int
	segBeats                 float64
}

type soundReq struct {
	id      string
	vol     float64
	pitch   float64
	dur     float64
	when    float64
	hasWhen bool
	enqAt   time.Time
	gen     uint64
	// parityGen is the parity generation in effect when this sound was
	// SCHEDULED (enqueued), captured under the same seqMu section that records
	// the matching seq decision. The audioLoop may only record the parity audio
	// event much later (goroutine lag under load), by which time a structural
	// mutation (e.g. a BPM change) can have advanced g.parityGen. Stamping the
	// event with the current generation at record time would land the audio on
	// a newer generation than its decision, so the parity gen-filter drops the
	// decision but keeps the audio — a phantom audio_vs_seq mismatch. Carrying
	// the scheduling generation here keeps both sides of the comparison in the
	// same generation (see recordParityAudio + parityScan gen filter).
	parityGen uint64
	// Parity metadata (row/abs) is only set for sequencer-driven playback
	// events. Preview sounds use (-1,-1) to avoid polluting parity buffers.
	row int
	abs int
}
