module github.com/ingyamilmolinar/tunkul

go 1.23.4

require (
	github.com/hajimehoshi/ebiten/v2 v2.8.6
	github.com/hajimehoshi/ebiten/v2/ebitenutil v0.0.0-00010101000000-000000000000
)

replace github.com/hajimehoshi/ebiten/v2 => ./internal/ebitestub/ebiten

replace github.com/hajimehoshi/ebiten/v2/ebitenutil => ./internal/ebitestub/ebitenutil

replace golang.org/x/sync => github.com/golang/sync v0.8.0

replace golang.org/x/sys => github.com/golang/sys v0.25.0

replace golang.org/x/image => github.com/golang/image v0.20.0
