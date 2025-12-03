package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Button struct {
	X, Y, W, H float32
	Label      string
	Hovered    bool
}

func (b *Button) Contains(x, y int) bool {
	fx, fy := float32(x), float32(y)
	return fx >= b.X && fx <= b.X+b.W && fy >= b.Y && fy <= b.Y+b.H
}

func (b *Button) Draw(screen *ebiten.Image, bgColor, hoverColor color.RGBA) {
	c := bgColor
	if b.Hovered {
		c = hoverColor
	}
	vector.DrawFilledRect(screen, b.X, b.Y, b.W, b.H, c, false)
	vector.StrokeRect(screen, b.X, b.Y, b.W, b.H, 1, color.RGBA{100, 100, 100, 255}, false)
	textX := int(b.X) + int(b.W/2) - len(b.Label)*3
	textY := int(b.Y) + int(b.H/2) - 6
	ebitenutil.DebugPrintAt(screen, b.Label, textX, textY)
}