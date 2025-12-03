package ui

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Slider struct {
	X, Y, W, H float32
	Value      float32
	Dragging   bool
	Label      string
	Color      color.RGBA
}

func (s *Slider) Contains(x, y int) bool {
	fx, fy := float32(x), float32(y)
	return fx >= s.X && fx <= s.X+s.W && fy >= s.Y-5 && fy <= s.Y+s.H+5
}

func (s *Slider) SetValueFromX(x int) {
	fx := float32(x)
	newVal := (fx - s.X) / s.W
	if newVal < 0.05 {
		newVal = 0.05
	}
	if newVal > 0.95 {
		newVal = 0.95
	}
	s.Value = newVal
}

func (s *Slider) GetPercent() int {
	return int(s.Value * 100)
}

func (s *Slider) Draw(screen *ebiten.Image) {
	vector.DrawFilledRect(screen, s.X, s.Y, s.W, s.H, color.RGBA{40, 40, 40, 255}, false)
	vector.DrawFilledRect(screen, s.X, s.Y, s.W*s.Value, s.H, s.Color, false)
	vector.StrokeRect(screen, s.X, s.Y, s.W, s.H, 1, color.RGBA{80, 80, 80, 255}, false)
	handleX := s.X + s.W*s.Value
	vector.DrawFilledRect(screen, handleX-4, s.Y-3, 8, s.H+6, color.RGBA{200, 200, 200, 255}, false)
	labelText := fmt.Sprintf("%s: %d%%", s.Label, s.GetPercent())
	ebitenutil.DebugPrintAt(screen, labelText, int(s.X), int(s.Y)-18)
}