package ui

import (
	"muletinha/input"
	"time"
)

type PotionConfig struct {
	Name      string
	KeyCombo  input.KeyCombo
	Threshold float32
	Cooldown  time.Duration
	LastUsed  time.Time
	Enabled   bool
	UseCount  int
	Slider    *Slider
	ToggleBtn *Button
}