package ui

import "math"

const GridStep = 60 // world-space px between vertices

// snap world coords to nearest vertex
func Snap(x, y float64) (gx, gy float64, ix, iy int) {
	ix = int(math.Round(x / GridStep))
	iy = int(math.Round(y / GridStep))
	return float64(ix * GridStep), float64(iy * GridStep), ix, iy
}

