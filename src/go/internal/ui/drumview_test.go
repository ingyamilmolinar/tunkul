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
	mx := dv.playBtn.Min.X + 1
	my := dv.playBtn.Min.Y + 1
	pressed := true
	restore := SetInputForTest(
		func() (int, int) { return mx, my },
		func(b ebiten.MouseButton) bool { return pressed },
		func(ebiten.Key) bool { return false },
		func() []rune { return nil },
		func() (float64, float64) { return 0, 0 },
		func() (int, int) { return 800, 600 },
	)
	defer restore()

	dv.Update()
	if !dv.playing {
		t.Fatal("expected playing after clicking play")
	}

	mx = dv.stopBtn.Min.X + 1
	my = dv.stopBtn.Min.Y + 1
	dv.Update()
	if dv.playing {
		t.Fatal("expected stopped after clicking stop")
	}
	pressed = false
}

func TestBPMInput(t *testing.T) {
	dv := setupDV()
	dv.bpm = 0
	dv.focusBPM = true
	chars := []rune{'9', '0'}
	backspace := false
	restore := SetInputForTest(
		func() (int, int) { return 0, 0 },
		func(ebiten.MouseButton) bool { return false },
		func(k ebiten.Key) bool {
			if k == ebiten.KeyBackspace {
				return backspace
			}
			return false
		},
		func() []rune {
			c := chars
			chars = nil
			return c
		},
		func() (float64, float64) { return 0, 0 },
		func() (int, int) { return 800, 600 },
	)
	defer restore()

	dv.Update()
	if dv.bpm != 90 {
		t.Fatalf("expected bpm 90, got %d", dv.bpm)
	}
	backspace = true
	dv.Update()
	if dv.bpm != 9 {
		t.Fatalf("expected bpm 9 after backspace, got %d", dv.bpm)
	}
}

func TestRowHeightConstant(t *testing.T) {
	dv := setupDV()
	dv.Update()
	initial := dv.bgCache[0].Bounds().Dy()
	dv.resizeSteps(+1)
	dv.Update()
	if dv.bgCache[0].Bounds().Dy() != initial {
		t.Fatalf("row height changed from %d to %d", initial, dv.bgCache[0].Bounds().Dy())
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
