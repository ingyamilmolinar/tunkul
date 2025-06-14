//go:build test

package ebiten

import (
	"image"
	"image/color"
)

type Image struct{}

func NewImage(w, h int) *Image                 { return &Image{} }
func NewImageFromImage(img image.Image) *Image { return &Image{} }

func (i *Image) DrawImage(src *Image, opts *DrawImageOptions) {}
func (i *Image) Fill(c color.Color)                           {}
func (i *Image) Bounds() image.Rectangle                      { return image.Rect(0, 0, 0, 0) }
func (i *Image) SubImage(r image.Rectangle) image.Image       { return &Image{} }
func (i *Image) Size() (int, int)                             { return 0, 0 }
func (i *Image) ColorModel() color.Model                      { return color.RGBAModel }
func (i *Image) At(x, y int) color.Color                      { return color.RGBA{} }

var (
	MockCursorX, MockCursorY int
	MousePressed             = map[MouseButton]bool{}
	KeysPressed              = map[Key]bool{}
	Chars                    []rune
)

func CursorPosition() (int, int)              { return MockCursorX, MockCursorY }
func IsMouseButtonPressed(b MouseButton) bool { return MousePressed[b] }
func IsKeyPressed(k Key) bool                 { return KeysPressed[k] }
func InputChars() []rune                      { c := Chars; Chars = nil; return c }
func Wheel() (float64, float64)               { return 0, 0 }
func ScreenSizeInFullscreen() (int, int)      { return 0, 0 }

// Drawing options
type DrawImageOptions struct{ GeoM GeoM }

// Constants
type MouseButton int

const (
	MouseButtonLeft MouseButton = iota
	MouseButtonRight
)

type Key int

const (
	KeyShiftLeft Key = iota
	KeyShiftRight
	KeyBackspace
)

// Window and run stubs
type Game interface {
	Update() error
	Draw(*Image)
	Layout(int, int) (int, int)
}

func SetWindowSize(w, h int)      {}
func SetWindowTitle(title string) {}
func RunGame(g Game) error        { return nil }
