package ui

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func setupDV() *DrumView {
	dv := NewDrumView(image.Rect(0, 0, 200, 100))
	dv.Rows = []*DrumRow{{Name: "H", Steps: make([]bool, 4)}}
	dv.recalcButtons()
	return dv
}

func TestPlayStopButtons(t *testing.T) {
	dv := setupDV()
	ebiten.MockCursorX = dv.playBtn.Min.X + 1
	ebiten.MockCursorY = dv.playBtn.Min.Y + 1
	ebiten.MousePressed[ebiten.MouseButtonLeft] = true
	dv.Update()
	if !dv.playing {
		t.Fatal("expected playing after clicking play")
	}
	ebiten.MockCursorX = dv.stopBtn.Min.X + 1
	ebiten.MockCursorY = dv.stopBtn.Min.Y + 1
	dv.Update()
	if dv.playing {
		t.Fatal("expected stopped after clicking stop")
	}
	ebiten.MousePressed[ebiten.MouseButtonLeft] = false
}

func TestBPMInput(t *testing.T) {
	dv := setupDV()
	dv.bpm = 0
	dv.focusBPM = true
	ebiten.Chars = []rune{'9', '0'}
	dv.Update()
	if dv.bpm != 90 {
		t.Fatalf("expected bpm 90, got %d", dv.bpm)
	}
	ebiten.KeysPressed[ebiten.KeyBackspace] = true
	dv.Update()
	if dv.bpm != 9 {
		t.Fatalf("expected bpm 9 after backspace, got %d", dv.bpm)
	}
	ebiten.KeysPressed[ebiten.KeyBackspace] = false
}

func TestRowHeightConstant(t *testing.T) {
	dv := setupDV()
	initial := dv.cell
	dv.resizeSteps(+1)
	if dv.cell != initial {
		t.Fatalf("cell changed from %d to %d", initial, dv.cell)
	}
}

func TestDrawAfterInit(t *testing.T) {
	dv := setupDV()
	dv.Update()
	img := ebiten.NewImage(200, 100)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Draw panicked: %v", r)
		}
	}()
	dv.Draw(img)
}
