package ui

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

/* ───────────────────────────────────────────────────────────── */

type DrumRow struct {
	Name   string
	Steps  []bool
	Offset int
	Color  color.Color
}

type DrumView struct {
	Rows   []*DrumRow
	Bounds image.Rectangle

	cell   int // px per cell
	labelW int

	// cached bg
	bgDirty bool
	bgCache []*ebiten.Image

	// beat-count buttons
	minusBtn image.Rectangle
	plusBtn  image.Rectangle
	playing  bool
	bpm      int
	playBtn  image.Rectangle
	stopBtn  image.Rectangle
	bpmBox   image.Rectangle
	focusBPM bool

	// drag-rotate
	dragRow    int
	prevMouseX int
}

/* ─── ctor ────────────────────────────────────────── */
func NewDrumView(b image.Rectangle) *DrumView {
	v := &DrumView{
		Bounds:  b,
		labelW:  40,
		bgDirty: true,
		dragRow: -1,
		bpm:     120,
		cell:    40,
	}
	return v
}

/* ─── interaction ─────────────────────────────────── */

func (dv *DrumView) inside(px, py int) bool {
	return px >= dv.Bounds.Min.X && px < dv.Bounds.Max.X &&
		py >= dv.Bounds.Min.Y && py < dv.Bounds.Max.Y
}

func (dv *DrumView) Update() {
	if len(dv.Rows) == 0 {
		return
	}
	dv.calcLayout()

	mx, my := ebiten.CursorPosition()
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		// beat-count buttons
		if ptIn(mx, my, dv.minusBtn) {
			dv.resizeSteps(-1)
			return
		}
		if ptIn(mx, my, dv.plusBtn) {
			dv.resizeSteps(+1)
			return
		}
	}

	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if ptIn(mx, my, dv.playBtn) {
			dv.playing = true
		}
		if ptIn(mx, my, dv.stopBtn) {
			dv.playing = false
		}
		if ptIn(mx, my, dv.bpmBox) {
			dv.focusBPM = true
		} else {
			dv.focusBPM = false
		}
	}
	if dv.focusBPM {
		for _, r := range ebiten.InputChars() {
			if r >= '0' && r <= '9' {
				dv.bpm = dv.bpm*10 + int(r-'0')
				if dv.bpm > 300 {
					dv.bpm = 300
				}
			}
		}
		if ebiten.IsKeyPressed(ebiten.KeyBackspace) {
			dv.bpm /= 10
			if dv.bpm == 0 {
				dv.bpm = 1
			}
		}
	}

	// rotate strip drag
	if !dv.inside(mx, my) {
		dv.dragRow = -1
		return
	}
	rowIdx := (my - dv.Bounds.Min.Y) / dv.cell
	if rowIdx < 0 || rowIdx >= len(dv.Rows) {
		dv.dragRow = -1
		return
	}

	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if dv.dragRow == -1 {
			dv.dragRow = rowIdx
			dv.prevMouseX = mx
		} else if dv.dragRow == rowIdx {
			dx := mx - dv.prevMouseX
			stepDelta := dx / dv.cell
			if stepDelta != 0 {
				r := dv.Rows[rowIdx]
				r.Offset = (r.Offset + stepDelta) % len(r.Steps)
				if r.Offset < 0 {
					r.Offset += len(r.Steps)
				}
				dv.prevMouseX = mx
			}
		}
	} else {
		dv.dragRow = -1
	}
}

/* ─── beat-count resize ───────────────────────────── */
func (dv *DrumView) resizeSteps(dir int) {
	row := dv.Rows[0]
	switch dir {
	case -1:
		if len(row.Steps) > 4 {
			row.Steps = row.Steps[:len(row.Steps)/2]
			dv.refreshLayout()
		}
	case +1:
		if len(row.Steps) < 64 {
			row.Steps = append(row.Steps, row.Steps...)
			dv.refreshLayout()
		}
	}
}

/* ─── cached background & layout ──────────────────── */

// calcCell ensures a sane default size for a drum row.
func (dv *DrumView) calcCell() {
	if dv.cell == 0 {
		dv.cell = 40
	}
}

func (dv *DrumView) refreshLayout() {
	dv.calcCell()
	dv.bgDirty = true
	dv.rebuildBG()
	if off := len(dv.Rows[0].Steps); off > 0 {
		dv.Rows[0].Offset %= off
	}
}

func (dv *DrumView) calcLayout() {
	dv.calcCell()
	if dv.bgDirty {
		dv.rebuildBG()
		dv.bgDirty = false
	}
}

func (dv *DrumView) rebuildBG() {
	dv.bgCache = make([]*ebiten.Image, len(dv.Rows))
	for idx := range dv.Rows {
		w := dv.Bounds.Dx()
		h := dv.cell
		img := ebiten.NewImage(w, h)
		img.Fill(color.RGBA{30, 30, 30, 255})

		// vertical grid
		for i := 0; i <= len(dv.Rows[idx].Steps); i++ {
			x := dv.labelW + i*dv.cell
			drawLine(img, float64(x), 0, float64(x), float64(h),
				color.RGBA{60, 60, 60, 255})
		}
		// label
		ebitenutil.DebugPrintAt(img, dv.Rows[idx].Name, 4, h/2-4)
		dv.bgCache[idx] = img
	}
}

func (dv *DrumView) recalcButtons() {
	dv.minusBtn = image.Rect(4, dv.Bounds.Min.Y+4, 32, dv.Bounds.Min.Y+24)
	dv.plusBtn = image.Rect(36, dv.Bounds.Min.Y+4, 64, dv.Bounds.Min.Y+24)
	dv.playBtn = image.Rect(dv.Bounds.Min.X+80, dv.Bounds.Min.Y+4, dv.Bounds.Min.X+104, dv.Bounds.Min.Y+24)
	dv.stopBtn = image.Rect(dv.Bounds.Min.X+110, dv.Bounds.Min.Y+4, dv.Bounds.Min.X+134, dv.Bounds.Min.Y+24)
	dv.bpmBox = image.Rect(dv.Bounds.Min.X+150, dv.Bounds.Min.Y+4, dv.Bounds.Min.X+210, dv.Bounds.Min.Y+24)
}

/* ─── drawing ─────────────────────────────────────── */

var idM ebiten.GeoM

func (dv *DrumView) Draw(dst *ebiten.Image) {
	if len(dv.Rows) == 0 {
		return
	}
	// Bounds may have moved (splitter): keep buttons aligned
	dv.recalcButtons()
	dv.calcLayout()

	// buttons
	drawRect(dst, dv.minusBtn, color.White)

	drawRect(dst, dv.plusBtn, color.White)

	// rows
	for idx, r := range dv.Rows {
		y := dv.Bounds.Min.Y + idx*dv.cell

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(dv.Bounds.Min.X), float64(y))
		dst.DrawImage(dv.bgCache[idx], op)

		for i := 0; i < len(r.Steps); i++ {
			if !r.Steps[(i+r.Offset)%len(r.Steps)] {
				continue
			}
			x := dv.Bounds.Min.X + dv.labelW + i*dv.cell
			scale := float64(dv.cell-4) / float64(NodeAnim.Bounds().Dx())
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(scale, scale)
			op.GeoM.Translate(float64(x+2), float64(y+2))
			dst.DrawImage(NodeFrames[0], op)
		}
	}

	// --- transport & length widgets (draw LAST so nothing hides them) ---
	drawRect(dst, dv.playBtn, color.White)
	ebitenutil.DebugPrintAt(dst, "▶",
		dv.playBtn.Min.X+4, dv.playBtn.Min.Y+2)

	drawRect(dst, dv.stopBtn, color.White)
	ebitenutil.DebugPrintAt(dst, "■",
		dv.stopBtn.Min.X+4, dv.stopBtn.Min.Y+2)

	drawRect(dst, dv.minusBtn, color.White)
	ebitenutil.DebugPrintAt(dst, "–",
		dv.minusBtn.Min.X+8, dv.minusBtn.Min.Y+2)

	drawRect(dst, dv.plusBtn, color.White)
	ebitenutil.DebugPrintAt(dst, "+",
		dv.plusBtn.Min.X+8, dv.plusBtn.Min.Y+2)

	drawRect(dst, dv.bpmBox, color.White)
	ebitenutil.DebugPrintAt(dst, fmt.Sprintf("%d", dv.bpm),
		dv.bpmBox.Min.X+4, dv.bpmBox.Min.Y+2)
}

/* ─── internal drawing utils ─────────────────────── */
func ptIn(x, y int, r image.Rectangle) bool {
	return x >= r.Min.X && x <= r.Max.X && y >= r.Min.Y && y <= r.Max.Y
}

func drawLine(dst *ebiten.Image, x1, y1, x2, y2 float64, col color.Color) {
	DrawLineCam(dst, x1, y1, x2, y2, &idM, col, 1)
}

func drawRect(dst *ebiten.Image, r image.Rectangle, col color.Color) {
	drawLine(dst, float64(r.Min.X), float64(r.Min.Y), float64(r.Max.X), float64(r.Min.Y), col)
	drawLine(dst, float64(r.Max.X), float64(r.Min.Y), float64(r.Max.X), float64(r.Max.Y), col)
	drawLine(dst, float64(r.Max.X), float64(r.Max.Y), float64(r.Min.X), float64(r.Max.Y), col)
	drawLine(dst, float64(r.Min.X), float64(r.Max.Y), float64(r.Min.X), float64(r.Min.Y), col)
}
