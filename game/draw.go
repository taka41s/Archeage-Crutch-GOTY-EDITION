package game

import (
	"fmt"
	"image/color"
	"muletinha/config"
	"muletinha/entity"
	"muletinha/ui"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var (
	colorBg         = color.RGBA{20, 25, 30, 255}
	colorPanel      = color.RGBA{25, 30, 38, 255}
	colorPanelLight = color.RGBA{32, 38, 48, 255}
	colorBorder     = color.RGBA{50, 58, 70, 255}
	colorText       = color.RGBA{220, 220, 220, 255}
	colorTextDim    = color.RGBA{140, 140, 140, 255}
	colorGreen      = color.RGBA{50, 200, 80, 255}
	colorRed        = color.RGBA{255, 60, 60, 255}
	colorYellow     = color.RGBA{255, 220, 50, 255}
	colorOrange     = color.RGBA{255, 150, 50, 255}
	colorPurple     = color.RGBA{150, 100, 255, 255}
	colorCyan       = color.RGBA{50, 200, 200, 255}
)

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(colorBg)

	if !g.connected {
		g.drawCenteredText(screen, "ArcheAge não conectado!", config.SCREEN_WIDTH/2, config.SCREEN_HEIGHT/2)
		return
	}

	g.mutex.RLock()
	localPlayer := g.localPlayer
	entities := make([]entity.Entity, len(g.entities))
	copy(entities, g.entities)
	g.mutex.RUnlock()

	if localPlayer.Address == 0 {
		g.drawCenteredText(screen, "Aguardando LocalPlayer...", config.SCREEN_WIDTH/2, config.SCREEN_HEIGHT/2)
		return
	}

	// Layout 1920x1080:
	// ┌─────────────────────────────────────────────────────────────┐
	// │  LEFT PANEL (400px)  │  CENTER (1120px)  │  RIGHT (400px)   │
	// │  - Player Info       │  - Radar          │  - Entity List   │
	// │  - HP Bar            │                   │  - Events        │
	// │  - Buffs             │                   │                  │
	// │  - Debuffs           │                   │                  │
	// ├─────────────────────────────────────────────────────────────┤
	// │                    BOTTOM CONFIG PANEL                      │
	// └─────────────────────────────────────────────────────────────┘

	leftPanelX := float32(10)
	leftPanelW := float32(420)
	centerX := float32(config.SCREEN_WIDTH / 2)
	rightPanelX := float32(config.SCREEN_WIDTH - 430)
	rightPanelW := float32(420)
	bottomPanelH := float32(180)

	// === LEFT PANEL ===
	g.drawLeftPanel(screen, localPlayer, leftPanelX, 10, leftPanelW)

	// === CENTER - RADAR ===
	radarY := float32(280)
	g.drawRadar(screen, localPlayer, entities, centerX, radarY)

	// === RIGHT PANEL ===
	g.drawRightPanel(screen, entities, rightPanelX, 10, rightPanelW)

	// === BOTTOM CONFIG PANEL ===
	g.drawConfigPanel(screen, float32(config.SCREEN_HEIGHT)-bottomPanelH-10, bottomPanelH)

	// FPS
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("FPS: %.0f", ebiten.ActualFPS()), config.SCREEN_WIDTH-80, 10)
}

func (g *Game) drawLeftPanel(screen *ebiten.Image, player entity.Entity, x, y, w float32) {
	panelH := float32(680)

	// Background
	vector.DrawFilledRect(screen, x, y, w, panelH, colorPanel, false)
	vector.StrokeRect(screen, x, y, w, panelH, 1, colorBorder, false)

	padding := float32(15)
	innerX := x + padding
	innerW := w - padding*2
	currentY := y + padding

	// === PLAYER INFO ===
	g.drawSectionHeader(screen, "PLAYER INFO", innerX, currentY, innerW)
	currentY += 25

	ebitenutil.DebugPrintAt(screen, player.Name, int(innerX), int(currentY))
	currentY += 16
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Pos: %.0f, %.0f, %.0f", player.PosX, player.PosY, player.PosZ), int(innerX), int(currentY))
	currentY += 25

	// === HP BAR ===
	g.drawSectionHeader(screen, "HEALTH", innerX, currentY, innerW)
	currentY += 25

	hpPercent := float32(0)
	if player.MaxHP > 0 {
		hpPercent = float32(player.HP) / float32(player.MaxHP)
	}

	barH := float32(28)
	vector.DrawFilledRect(screen, innerX, currentY, innerW, barH, color.RGBA{30, 30, 30, 255}, false)

	hpColor := colorGreen
	if hpPercent <= g.nuiNova.Slider.Value {
		hpColor = colorRed
	} else if hpPercent <= g.desertFire.Slider.Value {
		hpColor = colorOrange
	}
	vector.DrawFilledRect(screen, innerX, currentY, innerW*hpPercent, barH, hpColor, false)

	// Threshold lines
	dfX := innerX + innerW*g.desertFire.Slider.Value
	vector.StrokeLine(screen, dfX, currentY, dfX, currentY+barH, 2, colorOrange, false)
	nnX := innerX + innerW*g.nuiNova.Slider.Value
	vector.StrokeLine(screen, nnX, currentY, nnX, currentY+barH, 2, colorPurple, false)

	vector.StrokeRect(screen, innerX, currentY, innerW, barH, 1, colorBorder, false)

	hpText := fmt.Sprintf("%d / %d  (%.0f%%)", player.HP, player.MaxHP, hpPercent*100)
	textW := len(hpText) * 7
	ebitenutil.DebugPrintAt(screen, hpText, int(innerX)+int(innerW/2)-textW/2, int(currentY)+8)
	currentY += barH + 20

	// === STATUS ===
	g.drawSectionHeader(screen, "STATUS", innerX, currentY, innerW)
	currentY += 25

	// CC Break status
	ccColor := colorRed
	ccStatus := "OFF"
	if g.debuffMonitor.CCWhitelist.Enabled {
		ccColor = colorGreen
		ccStatus = "ON"
	}
	vector.DrawFilledRect(screen, innerX, currentY, 12, 12, ccColor, false)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf(" CC Break: %s (%d)", ccStatus, g.debuffMonitor.CCWhitelist.Reactions), int(innerX)+14, int(currentY)-1)
	currentY += 18

	// Buff Break status
	buffColor := colorRed
	buffStatus := "OFF"
	if g.buffMonitor.Whitelist.Enabled {
		buffColor = colorGreen
		buffStatus = "ON"
	}
	vector.DrawFilledRect(screen, innerX, currentY, 12, 12, buffColor, false)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf(" Buff Break: %s (%d)", buffStatus, g.buffMonitor.Whitelist.Reactions), int(innerX)+14, int(currentY)-1)
	currentY += 30

	// === BUFFS ===
	g.drawSectionHeader(screen, fmt.Sprintf("BUFFS (%d)", g.buffMonitor.RawCount), innerX, currentY, innerW)
	currentY += 25

	if len(g.buffMonitor.Buffs) == 0 {
		ebitenutil.DebugPrintAt(screen, "(none)", int(innerX), int(currentY))
		currentY += 16
	} else {
		maxShow := 6
		for i, b := range g.buffMonitor.Buffs {
			if i >= maxShow {
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("+%d more...", len(g.buffMonitor.Buffs)-maxShow), int(innerX), int(currentY))
				currentY += 14
				break
			}

			barColor := color.RGBA{50, 100, 50, 255}
			if b.Name != "" {
				barColor = color.RGBA{200, 150, 50, 255}
			}

			pct := float32(0)
			if b.Duration > 0 {
				pct = float32(b.TimeLeft) / float32(b.Duration)
				if pct > 1 {
					pct = 1
				}
			}

			barW := float32(120)
			barH := float32(10)
			vector.DrawFilledRect(screen, innerX, currentY, barW, barH, color.RGBA{40, 40, 40, 255}, false)
			vector.DrawFilledRect(screen, innerX, currentY, barW*pct, barH, barColor, false)

			text := fmt.Sprintf("ID:%d", b.ID)
			if b.Name != "" {
				text = fmt.Sprintf("[%s]", b.Name)
			}
			if b.Duration > 0 {
				text += fmt.Sprintf(" %.1fs", float64(b.TimeLeft)/1000)
			}
			ebitenutil.DebugPrintAt(screen, text, int(innerX)+125, int(currentY)-1)
			currentY += 14
		}
	}
	currentY += 15

	// === DEBUFFS ===
	g.drawSectionHeader(screen, fmt.Sprintf("DEBUFFS (%d)", g.debuffMonitor.RawCount), innerX, currentY, innerW)
	currentY += 25

	if len(g.debuffMonitor.Debuffs) == 0 {
		ebitenutil.DebugPrintAt(screen, "(none)", int(innerX), int(currentY))
	} else {
		maxShow := 6
		for i, d := range g.debuffMonitor.Debuffs {
			if i >= maxShow {
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("+%d more...", len(g.debuffMonitor.Debuffs)-maxShow), int(innerX), int(currentY))
				currentY += 14
				break
			}

			pct := float32(d.DurLeft) / float32(d.DurMax)
			if pct > 1 {
				pct = 1
			}

			barColor := color.RGBA{100, 100, 100, 255}
			if d.CCName != "" {
				barColor = colorRed
			}

			barW := float32(120)
			barH := float32(10)
			vector.DrawFilledRect(screen, innerX, currentY, barW, barH, color.RGBA{40, 40, 40, 255}, false)
			vector.DrawFilledRect(screen, innerX, currentY, barW*pct, barH, barColor, false)

			text := fmt.Sprintf("T:%d %.1fs", d.TypeID, float64(d.DurLeft)/1000)
			if d.CCName != "" {
				text = fmt.Sprintf("[%s] %.1fs", strings.ToUpper(d.CCName), float64(d.DurLeft)/1000)
			}
			ebitenutil.DebugPrintAt(screen, text, int(innerX)+125, int(currentY)-1)
			currentY += 14
		}
	}
}

func (g *Game) drawRadar(screen *ebiten.Image, player entity.Entity, entities []entity.Entity, centerX, centerY float32) {
	radius := float32(config.RADAR_RADIUS)

	// Radar background circles
	ui.DrawCircle(screen, centerX, centerY, radius, color.RGBA{35, 40, 50, 255})
	ui.DrawCircle(screen, centerX, centerY, radius*0.66, color.RGBA{30, 35, 45, 255})
	ui.DrawCircle(screen, centerX, centerY, radius*0.33, color.RGBA{28, 32, 42, 255})

	// Cross lines
	vector.StrokeLine(screen, centerX-radius, centerY, centerX+radius, centerY, 1, color.RGBA{45, 50, 60, 255}, false)
	vector.StrokeLine(screen, centerX, centerY-radius, centerX, centerY+radius, 1, color.RGBA{45, 50, 60, 255}, false)

	// Player dot (center)
	vector.DrawFilledCircle(screen, centerX, centerY, 6, colorGreen, false)

	// Entities
	scale := radius / float32(config.RADAR_RANGE)
	playerCount := 0
	npcCount := 0

	for _, e := range entities {
		if e.Distance > config.RADAR_RANGE {
			continue
		}

		dx := e.PosX - player.PosX
		dy := e.PosY - player.PosY
		radarX := centerX + dx*scale
		radarY := centerY - dy*scale

		distFromCenter := (radarX-centerX)*(radarX-centerX) + (radarY-centerY)*(radarY-centerY)
		if distFromCenter > radius*radius {
			continue
		}

		var dotColor color.RGBA
		if e.IsPlayer {
			dotColor = colorRed
			playerCount++
		} else {
			dotColor = colorYellow
			npcCount++
		}
		vector.DrawFilledCircle(screen, radarX, radarY, 5, dotColor, false)

		if e.Distance < 50 {
			ebitenutil.DebugPrintAt(screen, ui.TruncStr(e.Name, 10), int(radarX)+8, int(radarY)-4)
		}
	}

	// Radar info
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Range: %dm", config.RADAR_RANGE), int(centerX-radius)+5, int(centerY+radius)+10)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("P:%d  N:%d", playerCount, npcCount), int(centerX+radius)-60, int(centerY+radius)+10)
}

func (g *Game) drawRightPanel(screen *ebiten.Image, entities []entity.Entity, x, y, w float32) {
	panelH := float32(680)

	// Background
	vector.DrawFilledRect(screen, x, y, w, panelH, colorPanel, false)
	vector.StrokeRect(screen, x, y, w, panelH, 1, colorBorder, false)

	padding := float32(15)
	innerX := x + padding
	innerW := w - padding*2
	currentY := y + padding

	// === NEARBY ENTITIES ===
	g.drawSectionHeader(screen, fmt.Sprintf("NEARBY ENTITIES (%d)", len(entities)), innerX, currentY, innerW)
	currentY += 25

	if len(entities) == 0 {
		ebitenutil.DebugPrintAt(screen, "(none)", int(innerX), int(currentY))
		currentY += 16
	} else {
		maxShow := 15
		for i, e := range entities {
			if i >= maxShow {
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("+%d more...", len(entities)-maxShow), int(innerX), int(currentY))
				currentY += 14
				break
			}

			typeColor := colorYellow
			typeChar := "N"
			if e.IsPlayer {
				typeColor = colorRed
				typeChar = "P"
			}

			vector.DrawFilledCircle(screen, innerX+6, currentY+6, 4, typeColor, false)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("[%s] %-12s %3.0fm", typeChar, ui.TruncStr(e.Name, 12), e.Distance), int(innerX)+14, int(currentY))
			currentY += 15
		}
	}
	currentY += 25

	// === EVENTS ===
	g.drawSectionHeader(screen, "EVENTS (!! = reacted)", innerX, currentY, innerW)
	currentY += 25

	allEvents := make([]string, 0)

	for _, ev := range g.debuffMonitor.Events {
		prefix := ev.Type
		if ev.Reacted {
			prefix = "!!"
		}
		line := fmt.Sprintf("[%s] %s CC T:%d", ev.Time.Format("15:04:05"), prefix, ev.TypeID)
		if ev.CCName != "" {
			line += " " + ev.CCName
		}
		allEvents = append(allEvents, line)
	}

	for _, ev := range g.buffMonitor.Events {
		prefix := ev.Type
		if ev.Reacted {
			prefix = "!!"
		}
		line := fmt.Sprintf("[%s] %s BF ID:%d", ev.Time.Format("15:04:05"), prefix, ev.ID)
		if ev.Name != "" {
			line += " " + ev.Name
		}
		allEvents = append(allEvents, line)
	}

	if len(allEvents) == 0 {
		ebitenutil.DebugPrintAt(screen, "(none)", int(innerX), int(currentY))
	} else {
		maxShow := 20
		startIdx := 0
		if len(allEvents) > maxShow {
			startIdx = len(allEvents) - maxShow
		}
		for i := startIdx; i < len(allEvents); i++ {
			ebitenutil.DebugPrintAt(screen, ui.TruncStr(allEvents[i], 45), int(innerX), int(currentY))
			currentY += 14
		}
	}
}

func (g *Game) drawConfigPanel(screen *ebiten.Image, y, h float32) {
	x := float32(10)
	w := float32(config.SCREEN_WIDTH - 20)

	// Background
	vector.DrawFilledRect(screen, x, y, w, h, colorPanel, false)
	vector.StrokeRect(screen, x, y, w, h, 1, colorBorder, false)

	padding := float32(15)
	innerX := x + padding
	currentY := y + padding

	// Title
	ebitenutil.DebugPrintAt(screen, "=== CONFIGURATION ===   [F3] Toggle CC Break  |  [F4] Toggle Buff Break", int(innerX), int(currentY))
	currentY += 25

	// === ROW 1: Toggle Buttons ===
	g.masterToggleBtn.Y = currentY
	g.debuffMonitorBtn.Y = currentY
	g.ccBreakBtn.Y = currentY
	g.buffMonitorBtn.Y = currentY
	g.buffBreakBtn.Y = currentY

	// Draw all buttons
	btnColor := color.RGBA{40, 80, 40, 255}
	hoverColor := color.RGBA{50, 100, 50, 255}
	if !g.autoPotEnabled {
		btnColor = color.RGBA{80, 40, 40, 255}
		hoverColor = color.RGBA{100, 50, 50, 255}
	}
	g.masterToggleBtn.Draw(screen, btnColor, hoverColor)

	dbBtnColor := color.RGBA{40, 40, 80, 255}
	dbHoverColor := color.RGBA{50, 50, 100, 255}
	if !g.debuffMonitor.Enabled {
		dbBtnColor = color.RGBA{60, 40, 40, 255}
		dbHoverColor = color.RGBA{80, 50, 50, 255}
	}
	g.debuffMonitorBtn.Draw(screen, dbBtnColor, dbHoverColor)

	ccBtnColor := color.RGBA{80, 40, 80, 255}
	ccHoverColor := color.RGBA{100, 50, 100, 255}
	if !g.debuffMonitor.CCWhitelist.Enabled {
		ccBtnColor = color.RGBA{60, 40, 40, 255}
		ccHoverColor = color.RGBA{80, 50, 50, 255}
	}
	g.ccBreakBtn.Draw(screen, ccBtnColor, ccHoverColor)

	buffBtnColor := color.RGBA{40, 80, 80, 255}
	buffHoverColor := color.RGBA{50, 100, 100, 255}
	if !g.buffMonitor.Enabled {
		buffBtnColor = color.RGBA{60, 40, 40, 255}
		buffHoverColor = color.RGBA{80, 50, 50, 255}
	}
	g.buffMonitorBtn.Draw(screen, buffBtnColor, buffHoverColor)

	buffBrkColor := color.RGBA{80, 80, 40, 255}
	buffBrkHover := color.RGBA{100, 100, 50, 255}
	if !g.buffMonitor.Whitelist.Enabled {
		buffBrkColor = color.RGBA{60, 40, 40, 255}
		buffBrkHover = color.RGBA{80, 50, 50, 255}
	}
	g.buffBreakBtn.Draw(screen, buffBrkColor, buffBrkHover)

	currentY += 35

	// === ROW 2: Potion Sliders ===
	g.desertFire.Slider.Y = currentY
	g.desertFire.ToggleBtn.Y = currentY - 3
	g.desertFire.Slider.Draw(screen)

	dfBtnColor := color.RGBA{60, 80, 40, 255}
	dfHoverColor := color.RGBA{80, 100, 50, 255}
	if !g.desertFire.Enabled {
		dfBtnColor = color.RGBA{60, 50, 50, 255}
	}
	g.desertFire.ToggleBtn.Draw(screen, dfBtnColor, dfHoverColor)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Used: %d", g.desertFire.UseCount), int(g.desertFire.ToggleBtn.X+50), int(currentY))

	currentY += 35

	g.nuiNova.Slider.Y = currentY
	g.nuiNova.ToggleBtn.Y = currentY - 3
	g.nuiNova.Slider.Draw(screen)

	nnBtnColor := color.RGBA{50, 40, 80, 255}
	nnHoverColor := color.RGBA{70, 50, 100, 255}
	if !g.nuiNova.Enabled {
		nnBtnColor = color.RGBA{50, 45, 55, 255}
	}
	g.nuiNova.ToggleBtn.Draw(screen, nnBtnColor, nnHoverColor)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Used: %d", g.nuiNova.UseCount), int(g.nuiNova.ToggleBtn.X+50), int(currentY))

	currentY += 30

	// === ROW 3: Info ===
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("CC Whitelist: %d entries  |  Buff Whitelist: %d entries",
		len(g.debuffMonitor.CCWhitelist.Entries), len(g.buffMonitor.Whitelist.Entries)), int(innerX), int(currentY))
}

func (g *Game) drawSectionHeader(screen *ebiten.Image, title string, x, y, w float32) {
	// Line
	vector.StrokeLine(screen, x, y+8, x+w, y+8, 1, colorBorder, false)

	// Background for text
	textW := float32(len(title)*7 + 10)
	vector.DrawFilledRect(screen, x, y, textW, 16, colorPanel, false)

	// Text
	ebitenutil.DebugPrintAt(screen, title, int(x), int(y))
}

func (g *Game) drawCenteredText(screen *ebiten.Image, text string, x, y int) {
	textW := len(text) * 7
	ebitenutil.DebugPrintAt(screen, text, x-textW/2, y)
}