package ui

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func DrawCircle(screen *ebiten.Image, cx, cy, radius float32, clr color.RGBA) {
	segments := 48
	for i := 0; i < segments; i++ {
		angle1 := float64(i) * 2 * math.Pi / float64(segments)
		angle2 := float64(i+1) * 2 * math.Pi / float64(segments)
		x1 := cx + radius*float32(math.Cos(angle1))
		y1 := cy + radius*float32(math.Sin(angle1))
		x2 := cx + radius*float32(math.Cos(angle2))
		y2 := cy + radius*float32(math.Sin(angle2))
		vector.StrokeLine(screen, x1, y1, x2, y2, 1, clr, false)
	}
}

func TruncStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "."
}