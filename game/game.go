package game

import (
	"fmt"
	"image/color"
	"muletinha/config"
	"muletinha/entity"
	"muletinha/input"
	"muletinha/memory"
	"muletinha/monitor"
	"muletinha/process"
	"muletinha/ui"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/sys/windows"
)

var (
	debuffBuffer = make([]byte, 30*config.DEBUFF_SIZE)
	buffBuffer   = make([]byte, 30*config.BUFF_SIZE)
)

type Game struct {
	handle      windows.Handle
	x2game      uintptr
	localPlayer entity.Entity
	playerMount entity.Entity
	entities    []entity.Entity
	mutex       sync.RWMutex
	connected   bool
	frameCount  int

	autoPotEnabled   bool
	masterToggleBtn  *ui.Button
	desertFire       *ui.PotionConfig
	nuiNova          *ui.PotionConfig

	debuffMonitor    *monitor.DebuffMonitor
	debuffMonitorBtn *ui.Button
	ccBreakBtn       *ui.Button

	buffMonitor    *monitor.BuffMonitor
	buffMonitorBtn *ui.Button
	buffBreakBtn   *ui.Button

	mouseX, mouseY int

	cachedDebuffBase   uintptr
	cachedBuffListAddr uintptr
	lastBaseCheck      time.Time
	lastBuffCheck      time.Time
	lastEntityScan     time.Time
	entityScanInterval time.Duration
	scanningEntities   bool
}

func NewGame() *Game {
	panelY := float32(config.SCREEN_HEIGHT - 200)
	sliderW := float32(200)

	g := &Game{
		autoPotEnabled:     true,
		debuffMonitor:      monitor.NewDebuffMonitor(),
		buffMonitor:        monitor.NewBuffMonitor(),
		entityScanInterval: 1000 * time.Millisecond,
		entities:           make([]entity.Entity, 0, 100),
		masterToggleBtn: &ui.Button{
			X: 10, Y: panelY, W: 100, H: 22,
			Label: "AutoPot:ON",
		},
		debuffMonitorBtn: &ui.Button{
			X: 115, Y: panelY, W: 100, H: 22,
			Label: "Debuff:ON",
		},
		ccBreakBtn: &ui.Button{
			X: 220, Y: panelY, W: 100, H: 22,
			Label: "CCBreak:ON",
		},
		buffMonitorBtn: &ui.Button{
			X: 325, Y: panelY, W: 100, H: 22,
			Label: "Buff:ON",
		},
		buffBreakBtn: &ui.Button{
			X: 430, Y: panelY, W: 100, H: 22,
			Label: "BuffBrk:ON",
		},
		desertFire: &ui.PotionConfig{
			Name:      "Desert Fire",
			KeyCombo:  input.ParseKeyCombo("F1"),
			Threshold: 0.60,
			Cooldown:  1500 * time.Millisecond,
			Enabled:   true,
			Slider: &ui.Slider{
				X: 120, Y: panelY + 35, W: sliderW, H: 12,
				Value: 0.60,
				Color: color.RGBA{255, 150, 50, 255},
				Label: "Desert Fire (F1)",
			},
			ToggleBtn: &ui.Button{X: 330, Y: panelY + 32, W: 40, H: 18, Label: "ON"},
		},
		nuiNova: &ui.PotionConfig{
			Name:      "Nui's Nova",
			KeyCombo:  input.ParseKeyCombo("F2"),
			Threshold: 0.20,
			Cooldown:  30 * time.Second,
			Enabled:   true,
			Slider: &ui.Slider{
				X: 120, Y: panelY + 70, W: sliderW, H: 12,
				Value: 0.20,
				Color: color.RGBA{150, 100, 255, 255},
				Label: "Nui's Nova (F2)",
			},
			ToggleBtn: &ui.Button{X: 330, Y: panelY + 67, W: 40, H: 18, Label: "ON"},
		},
	}

	pid, err := process.FindProcess("archeage.exe")
	if err != nil || pid == 0 {
		fmt.Println("ArcheAge não encontrado!")
		return g
	}

	handle, err := windows.OpenProcess(0x1F0FFF, false, pid)
	if err != nil {
		fmt.Println("Erro ao abrir processo:", err)
		return g
	}

	x2game, err := process.GetModuleBase(pid, "x2game.dll")
	if err != nil {
		fmt.Println("x2game.dll não encontrado!")
		windows.CloseHandle(handle)
		return g
	}

	g.handle = handle
	g.x2game = x2game
	g.connected = true

	return g
}

func (g *Game) GetHandle() windows.Handle {
	return g.handle
}

func sendKeyPotion(combo input.KeyCombo) {
	input.SendKeyCombo(combo)
}

func (g *Game) checkAndUsePotion() {
	if !g.autoPotEnabled || g.localPlayer.Address == 0 || g.localPlayer.MaxHP == 0 {
		return
	}

	hpPercent := float32(g.localPlayer.HP) / float32(g.localPlayer.MaxHP)
	now := time.Now()

	g.desertFire.Threshold = g.desertFire.Slider.Value
	g.nuiNova.Threshold = g.nuiNova.Slider.Value

	if g.nuiNova.Enabled && hpPercent <= g.nuiNova.Threshold {
		if now.Sub(g.nuiNova.LastUsed) >= g.nuiNova.Cooldown {
			go sendKeyPotion(g.nuiNova.KeyCombo)
			g.nuiNova.LastUsed = now
			g.nuiNova.UseCount++
			return
		}
	}

	if g.desertFire.Enabled && hpPercent <= g.desertFire.Threshold {
		if now.Sub(g.desertFire.LastUsed) >= g.desertFire.Cooldown {
			go sendKeyPotion(g.desertFire.KeyCombo)
			g.desertFire.LastUsed = now
			g.desertFire.UseCount++
		}
	}
}

func (g *Game) getDebuffBaseFast() uintptr {
	if time.Since(g.lastBaseCheck) < 50*time.Millisecond && g.cachedDebuffBase != 0 {
		return g.cachedDebuffBase
	}

	ptr1 := memory.ReadU32(g.handle, g.x2game+config.PTR_LOCALPLAYER)
	if ptr1 == 0 {
		return 0
	}
	ptr2 := memory.ReadU32(g.handle, uintptr(ptr1)+config.PTR_ENTITY)
	if ptr2 == 0 {
		return 0
	}
	entityBase := memory.ReadU32(g.handle, uintptr(ptr2)+config.OFF_ENTITY_BASE)
	if entityBase == 0 {
		return 0
	}
	debuffBase := memory.ReadU32(g.handle, uintptr(entityBase)+config.OFF_DEBUFF_PTR)
	if debuffBase == 0 || !memory.IsValidPtr(debuffBase) {
		return 0
	}

	g.cachedDebuffBase = uintptr(debuffBase)
	g.lastBaseCheck = time.Now()
	return g.cachedDebuffBase
}

func (g *Game) findBuffListFromPlayer() uintptr {
	if time.Since(g.lastBuffCheck) < 100*time.Millisecond && g.cachedBuffListAddr != 0 {
		return g.cachedBuffListAddr
	}

	ptr1 := memory.ReadU32(g.handle, g.x2game+config.PTR_LOCALPLAYER)
	if ptr1 == 0 {
		return 0
	}
	playerAddr := memory.ReadU32(g.handle, uintptr(ptr1)+config.PTR_ENTITY)
	if playerAddr == 0 {
		return 0
	}

	base := memory.ReadU32(g.handle, uintptr(playerAddr)+config.OFF_ENTITY_BASE)
	if !memory.IsValidPtr(base) {
		return 0
	}

	baseData := make([]byte, 0x5000)
	if memory.ReadMemoryBytes(g.handle, uintptr(base), baseData) != nil {
		return 0
	}

	for off := uint32(0); off < 0x4800; off += 4 {
		ptr := memory.BytesToUint32(baseData[off : off+4])
		if !memory.IsValidPtr(ptr) {
			continue
		}

		listData := make([]byte, 0x40)
		if memory.ReadMemoryBytes(g.handle, uintptr(ptr), listData) != nil {
			continue
		}

		count := memory.BytesToUint32(listData[config.BUFF_COUNT_OFF : config.BUFF_COUNT_OFF+4])
		if count >= 1 && count <= 50 {
			arrayStart := ptr + config.BUFF_ARRAY_OFF
			firstBuffData := make([]byte, 0x10)
			if memory.ReadMemoryBytes(g.handle, uintptr(arrayStart), firstBuffData) == nil {
				buffID := memory.BytesToUint32(firstBuffData[config.BUFF_OFF_ID : config.BUFF_OFF_ID+4])
				if buffID > 1000 && buffID < 9999999 {
					g.cachedBuffListAddr = uintptr(ptr)
					g.lastBuffCheck = time.Now()
					return g.cachedBuffListAddr
				}
			}
		}
	}

	return 0
}

func (g *Game) updateBuffsInstant() {
	if !g.buffMonitor.Enabled {
		return
	}

	buffListAddr := g.findBuffListFromPlayer()
	if buffListAddr == 0 {
		return
	}

	g.buffMonitor.BuffListAddr = buffListAddr
	count := memory.ReadU32(g.handle, buffListAddr+config.BUFF_COUNT_OFF)
	g.buffMonitor.RawCount = count

	if count == 0 || count > 50 {
		if len(g.buffMonitor.KnownIDs) > 0 {
			for k := range g.buffMonitor.KnownIDs {
				delete(g.buffMonitor.KnownIDs, k)
			}
		}
		g.buffMonitor.Buffs = g.buffMonitor.Buffs[:0]
		return
	}

	arrayAddr := buffListAddr + config.BUFF_ARRAY_OFF

	totalSize := 30 * config.BUFF_SIZE
	if totalSize > len(buffBuffer) {
		totalSize = len(buffBuffer)
	}

	var bytesRead uintptr
	ret, _, _ := memory.ProcReadProcessMemory.Call(
		uintptr(g.handle),
		uintptr(arrayAddr),
		uintptr(unsafe.Pointer(&buffBuffer[0])),
		uintptr(totalSize),
		uintptr(unsafe.Pointer(&bytesRead)),
	)

	if ret == 0 {
		return
	}

	newBuffs := g.buffMonitor.Buffs[:0]
	currentIDs := make(map[uint32]bool, count)

	maxItems := int(bytesRead) / config.BUFF_SIZE
	if maxItems > 30 {
		maxItems = 30
	}

	foundCount := 0
	for i := 0; i < maxItems && foundCount < int(count); i++ {
		offset := i * config.BUFF_SIZE

		buffID := memory.BytesToUint32(buffBuffer[offset+config.BUFF_OFF_ID : offset+config.BUFF_OFF_ID+4])
		duration := memory.BytesToUint32(buffBuffer[offset+config.BUFF_OFF_DUR : offset+config.BUFF_OFF_DUR+4])
		timeLeft := memory.BytesToUint32(buffBuffer[offset+config.BUFF_OFF_LEFT : offset+config.BUFF_OFF_LEFT+4])

		if buffID < 1000 || buffID > 9999999 {
			continue
		}

		currentIDs[buffID] = true
		foundCount++

		buffName := g.buffMonitor.Whitelist.GetName(buffID)

		if !g.buffMonitor.KnownIDs[buffID] {
			g.buffMonitor.KnownIDs[buffID] = true

			reacted, reactedName := g.buffMonitor.Whitelist.ReactInstant(buffID)

			if reacted {
				fmt.Printf("[BUFF] %s (ID:%d) -> REACT!\n", reactedName, buffID)
			}

			g.buffMonitor.AddEvent("+", buffID, buffName, reacted)
		}

		newBuffs = append(newBuffs, monitor.BuffInfo{
			Index:    i,
			ID:       buffID,
			Duration: duration,
			TimeLeft: timeLeft,
			Name:     buffName,
		})
	}

	for id := range g.buffMonitor.KnownIDs {
		if !currentIDs[id] {
			delete(g.buffMonitor.KnownIDs, id)
			name := g.buffMonitor.Whitelist.GetName(id)
			g.buffMonitor.AddEvent("-", id, name, false)
		}
	}

	g.buffMonitor.Buffs = newBuffs
}

func (g *Game) updateDebuffsInstant() {
	if !g.debuffMonitor.Enabled {
		return
	}

	debuffBase := g.getDebuffBaseFast()
	if debuffBase == 0 {
		return
	}

	g.debuffMonitor.DebuffBase = debuffBase
	count := memory.ReadU32(g.handle, debuffBase+config.OFF_DEBUFF_COUNT)
	g.debuffMonitor.RawCount = count

	if count == 0 || count > 50 {
		if len(g.debuffMonitor.KnownIDs) > 0 {
			for k := range g.debuffMonitor.KnownIDs {
				delete(g.debuffMonitor.KnownIDs, k)
			}
		}
		g.debuffMonitor.Debuffs = g.debuffMonitor.Debuffs[:0]
		return
	}

	arrayAddr := debuffBase + config.OFF_DEBUFF_ARRAY

	totalSize := int(count) * config.DEBUFF_SIZE
	if totalSize > len(debuffBuffer) {
		totalSize = len(debuffBuffer)
	}

	var bytesRead uintptr
	ret, _, _ := memory.ProcReadProcessMemory.Call(
		uintptr(g.handle),
		uintptr(arrayAddr),
		uintptr(unsafe.Pointer(&debuffBuffer[0])),
		uintptr(totalSize),
		uintptr(unsafe.Pointer(&bytesRead)),
	)

	if ret == 0 {
		return
	}

	newDebuffs := g.debuffMonitor.Debuffs[:0]
	currentIDs := make(map[uint64]bool, count)

	maxItems := int(bytesRead) / config.DEBUFF_SIZE
	if maxItems > 30 {
		maxItems = 30
	}

	for i := 0; i < maxItems; i++ {
		offset := i * config.DEBUFF_SIZE

		id := memory.BytesToUint32(debuffBuffer[offset : offset+4])
		typeID := memory.BytesToUint32(debuffBuffer[offset+4 : offset+8])
		durMax := memory.BytesToUint32(debuffBuffer[offset+0x30 : offset+0x34])
		durLeft := memory.BytesToUint32(debuffBuffer[offset+0x34 : offset+0x38])

		if id < 1 || id > 50000 || durMax < 1000 || durMax > 300000 {
			continue
		}

		key := monitor.MakeKey(id, typeID)
		currentIDs[key] = true

		ccName := g.debuffMonitor.CCWhitelist.GetName(typeID)

		if !g.debuffMonitor.KnownIDs[key] {
			g.debuffMonitor.KnownIDs[key] = true

			reacted, reactedName := g.debuffMonitor.CCWhitelist.ReactInstant(typeID)

			if reacted {
				fmt.Printf("[CC] %s (T:%d) -> SPAM!\n", reactedName, typeID)
			}

			g.debuffMonitor.AddEvent("+", id, typeID, ccName, reacted)
		}

		newDebuffs = append(newDebuffs, monitor.DebuffInfo{
			Index:   i,
			ID:      id,
			TypeID:  typeID,
			DurMax:  durMax,
			DurLeft: durLeft,
			CCName:  ccName,
		})
	}

	for key := range g.debuffMonitor.KnownIDs {
		if !currentIDs[key] {
			delete(g.debuffMonitor.KnownIDs, key)
			id := uint32(key >> 32)
			typeID := uint32(key & 0xFFFFFFFF)
			g.debuffMonitor.AddEvent("-", id, typeID, "", false)
		}
	}

	g.debuffMonitor.Debuffs = newDebuffs
}

func (g *Game) handleInput() {
	g.mouseX, g.mouseY = ebiten.CursorPosition()

	g.masterToggleBtn.Hovered = g.masterToggleBtn.Contains(g.mouseX, g.mouseY)
	g.debuffMonitorBtn.Hovered = g.debuffMonitorBtn.Contains(g.mouseX, g.mouseY)
	g.ccBreakBtn.Hovered = g.ccBreakBtn.Contains(g.mouseX, g.mouseY)
	g.buffMonitorBtn.Hovered = g.buffMonitorBtn.Contains(g.mouseX, g.mouseY)
	g.buffBreakBtn.Hovered = g.buffBreakBtn.Contains(g.mouseX, g.mouseY)
	g.desertFire.ToggleBtn.Hovered = g.desertFire.ToggleBtn.Contains(g.mouseX, g.mouseY)
	g.nuiNova.ToggleBtn.Hovered = g.nuiNova.ToggleBtn.Contains(g.mouseX, g.mouseY)

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if g.masterToggleBtn.Contains(g.mouseX, g.mouseY) {
			g.autoPotEnabled = !g.autoPotEnabled
			if g.autoPotEnabled {
				g.masterToggleBtn.Label = "AutoPot:ON"
			} else {
				g.masterToggleBtn.Label = "AutoPot:OFF"
			}
		}

		if g.debuffMonitorBtn.Contains(g.mouseX, g.mouseY) {
			g.debuffMonitor.Enabled = !g.debuffMonitor.Enabled
			if g.debuffMonitor.Enabled {
				g.debuffMonitorBtn.Label = "Debuff:ON"
			} else {
				g.debuffMonitorBtn.Label = "Debuff:OFF"
			}
		}

		if g.ccBreakBtn.Contains(g.mouseX, g.mouseY) {
			g.debuffMonitor.CCWhitelist.Enabled = !g.debuffMonitor.CCWhitelist.Enabled
			if g.debuffMonitor.CCWhitelist.Enabled {
				g.ccBreakBtn.Label = "CCBreak:ON"
			} else {
				g.ccBreakBtn.Label = "CCBreak:OFF"
			}
		}

		if g.buffMonitorBtn.Contains(g.mouseX, g.mouseY) {
			g.buffMonitor.Enabled = !g.buffMonitor.Enabled
			if g.buffMonitor.Enabled {
				g.buffMonitorBtn.Label = "Buff:ON"
			} else {
				g.buffMonitorBtn.Label = "Buff:OFF"
			}
		}

		if g.buffBreakBtn.Contains(g.mouseX, g.mouseY) {
			g.buffMonitor.Whitelist.Enabled = !g.buffMonitor.Whitelist.Enabled
			if g.buffMonitor.Whitelist.Enabled {
				g.buffBreakBtn.Label = "BuffBrk:ON"
			} else {
				g.buffBreakBtn.Label = "BuffBrk:OFF"
			}
		}

		if g.desertFire.ToggleBtn.Contains(g.mouseX, g.mouseY) {
			g.desertFire.Enabled = !g.desertFire.Enabled
			if g.desertFire.Enabled {
				g.desertFire.ToggleBtn.Label = "ON"
			} else {
				g.desertFire.ToggleBtn.Label = "OFF"
			}
		}

		if g.nuiNova.ToggleBtn.Contains(g.mouseX, g.mouseY) {
			g.nuiNova.Enabled = !g.nuiNova.Enabled
			if g.nuiNova.Enabled {
				g.nuiNova.ToggleBtn.Label = "ON"
			} else {
				g.nuiNova.ToggleBtn.Label = "OFF"
			}
		}

		if g.desertFire.Slider.Contains(g.mouseX, g.mouseY) {
			g.desertFire.Slider.Dragging = true
		}
		if g.nuiNova.Slider.Contains(g.mouseX, g.mouseY) {
			g.nuiNova.Slider.Dragging = true
		}
	}

	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if g.desertFire.Slider.Dragging {
			g.desertFire.Slider.SetValueFromX(g.mouseX)
		}
		if g.nuiNova.Slider.Dragging {
			g.nuiNova.Slider.SetValueFromX(g.mouseX)
		}
	} else {
		g.desertFire.Slider.Dragging = false
		g.nuiNova.Slider.Dragging = false
	}

	// Hotkeys
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		g.debuffMonitor.CCWhitelist.Enabled = !g.debuffMonitor.CCWhitelist.Enabled
		if g.debuffMonitor.CCWhitelist.Enabled {
			g.ccBreakBtn.Label = "CCBreak:ON"
		} else {
			g.ccBreakBtn.Label = "CCBreak:OFF"
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF4) {
		g.buffMonitor.Whitelist.Enabled = !g.buffMonitor.Whitelist.Enabled
		if g.buffMonitor.Whitelist.Enabled {
			g.buffBreakBtn.Label = "BuffBrk:ON"
		} else {
			g.buffBreakBtn.Label = "BuffBrk:OFF"
		}
	}
}

func (g *Game) Update() error {
	g.handleInput()

	if !g.connected {
		return nil
	}

	g.frameCount++

	// CC BREAK e BUFF BREAK - Prioridade máxima, todo frame
	g.updateDebuffsInstant()
	g.updateBuffsInstant()

	// Player info e potions - a cada 5 frames
	if g.frameCount%5 == 0 {
		g.localPlayer = entity.GetLocalPlayer(g.handle, g.x2game)
		g.playerMount = entity.GetPlayerMount(g.handle, g.x2game)
		g.checkAndUsePotion()
	}

	// Scan de entidades em background - a cada 1 segundo
	if time.Since(g.lastEntityScan) >= g.entityScanInterval && !g.scanningEntities {
		g.lastEntityScan = time.Now()

		if g.localPlayer.Address != 0 {
			g.scanningEntities = true

			playerCopy := g.localPlayer
			handleCopy := g.handle

			go func() {
				entities := entity.FindAllEntities(handleCopy, playerCopy, config.SCAN_RANGE)
				filtered := entity.FilterEntities(entities, playerCopy)

				g.mutex.Lock()
				g.entities = filtered
				g.scanningEntities = false
				g.mutex.Unlock()
			}()
		}
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{20, 25, 30, 255})

	centerX := float32(config.SCREEN_WIDTH / 2)
	centerY := float32(160)

	// RADAR
	ui.DrawCircle(screen, centerX, centerY, config.RADAR_RADIUS, color.RGBA{40, 45, 55, 255})
	ui.DrawCircle(screen, centerX, centerY, config.RADAR_RADIUS*0.66, color.RGBA{35, 40, 50, 255})
	ui.DrawCircle(screen, centerX, centerY, config.RADAR_RADIUS*0.33, color.RGBA{30, 35, 45, 255})
	vector.StrokeLine(screen, centerX-config.RADAR_RADIUS, centerY, centerX+config.RADAR_RADIUS, centerY, 1, color.RGBA{40, 45, 55, 255}, false)
	vector.StrokeLine(screen, centerX, centerY-config.RADAR_RADIUS, centerX, centerY+config.RADAR_RADIUS, 1, color.RGBA{40, 45, 55, 255}, false)
	vector.DrawFilledCircle(screen, centerX, centerY, 5, color.RGBA{0, 255, 100, 255}, false)

	if !g.connected {
		ebitenutil.DebugPrintAt(screen, "ArcheAge não conectado!", 10, 10)
		return
	}

	g.mutex.RLock()
	localPlayer := g.localPlayer
	entities := make([]entity.Entity, len(g.entities))
	copy(entities, g.entities)
	g.mutex.RUnlock()

	if localPlayer.Address == 0 {
		ebitenutil.DebugPrintAt(screen, "Aguardando LocalPlayer...", 10, 10)
		return
	}

	// Radar entities
	scale := float32(config.RADAR_RADIUS) / float32(config.RADAR_RANGE)
	playerCount := 0
	npcCount := 0

	for _, e := range entities {
		if e.Distance > config.RADAR_RANGE {
			continue
		}
		dx := e.PosX - localPlayer.PosX
		dy := e.PosY - localPlayer.PosY
		radarX := centerX + dx*scale
		radarY := centerY - dy*scale

		distFromCenter := float32(0)
		distFromCenter = (radarX-centerX)*(radarX-centerX) + (radarY-centerY)*(radarY-centerY)
		if distFromCenter > config.RADAR_RADIUS*config.RADAR_RADIUS {
			continue
		}

		var dotColor color.RGBA
		if e.IsPlayer {
			dotColor = color.RGBA{255, 60, 60, 255}
			playerCount++
		} else {
			dotColor = color.RGBA{255, 220, 50, 255}
			npcCount++
		}
		vector.DrawFilledCircle(screen, radarX, radarY, 4, dotColor, false)

		if e.Distance < 60 {
			ebitenutil.DebugPrintAt(screen, ui.TruncStr(e.Name, 8), int(radarX)+6, int(radarY)-4)
		}
	}

	// Player info
	info := fmt.Sprintf("%s | (%.0f, %.0f, %.0f)", localPlayer.Name, localPlayer.PosX, localPlayer.PosY, localPlayer.PosZ)
	ebitenutil.DebugPrintAt(screen, info, 10, 10)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Players:%d NPCs:%d", playerCount, npcCount), 10, 26)

	if g.playerMount.Address != 0 {
		mountInfo := fmt.Sprintf("Mount: %s | HP: %d/%d | 0x%08X", 
			g.playerMount.Name, g.playerMount.HP, g.playerMount.MaxHP, g.playerMount.Address)
		ebitenutil.DebugPrintAt(screen, mountInfo, 10, 42)
	}

	// HP BAR
	hpPercent := float32(0)
	if localPlayer.MaxHP > 0 {
		hpPercent = float32(localPlayer.HP) / float32(localPlayer.MaxHP)
	}

	barWidth := float32(380)
	barHeight := float32(22)
	barX := float32(config.SCREEN_WIDTH/2) - barWidth/2
	barY := float32(360)

	vector.DrawFilledRect(screen, barX, barY, barWidth, barHeight, color.RGBA{30, 30, 30, 255}, false)

	hpColor := color.RGBA{50, 200, 80, 255}
	if hpPercent <= g.nuiNova.Slider.Value {
		hpColor = color.RGBA{255, 50, 50, 255}
	} else if hpPercent <= g.desertFire.Slider.Value {
		hpColor = color.RGBA{255, 180, 50, 255}
	}
	vector.DrawFilledRect(screen, barX, barY, barWidth*hpPercent, barHeight, hpColor, false)

	dfX := barX + barWidth*g.desertFire.Slider.Value
	vector.StrokeLine(screen, dfX, barY, dfX, barY+barHeight, 2, color.RGBA{255, 150, 50, 200}, false)
	nnX := barX + barWidth*g.nuiNova.Slider.Value
	vector.StrokeLine(screen, nnX, barY, nnX, barY+barHeight, 2, color.RGBA{150, 100, 255, 200}, false)
	vector.StrokeRect(screen, barX, barY, barWidth, barHeight, 1, color.RGBA{80, 80, 80, 255}, false)

	hpText := fmt.Sprintf("%d/%d (%.0f%%)", localPlayer.HP, localPlayer.MaxHP, hpPercent*100)
	ebitenutil.DebugPrintAt(screen, hpText, int(barX)+int(barWidth/2)-50, int(barY)+5)

	// CC STATUS
	sectionY := int(barY) + 35

	ccColor := color.RGBA{150, 50, 50, 255}
	if g.debuffMonitor.CCWhitelist.Enabled {
		ccColor = color.RGBA{50, 200, 50, 255}
	}
	vector.DrawFilledRect(screen, barX, float32(sectionY), 14, 14, ccColor, false)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf(" CC Break: %d | Buff Break: %d",
		g.debuffMonitor.CCWhitelist.Reactions, g.buffMonitor.Whitelist.Reactions), int(barX)+16, sectionY)
	sectionY += 20

	// BUFFS Section
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("--- Buffs (%d) ---", g.buffMonitor.RawCount), int(barX), sectionY)
	sectionY += 16

	if len(g.buffMonitor.Buffs) == 0 {
		ebitenutil.DebugPrintAt(screen, "(none)", int(barX), sectionY)
		sectionY += 14
	} else {
		for _, b := range g.buffMonitor.Buffs {
			barColor := color.RGBA{50, 100, 50, 255}
			if b.Name != "" {
				barColor = color.RGBA{200, 150, 50, 255}
			}

			dbBarW := float32(100)
			dbBarH := float32(10)
			pct := float64(0)
			if b.Duration > 0 {
				pct = float64(b.TimeLeft) / float64(b.Duration) * 100
				if pct > 100 {
					pct = 100
				}
			}

			vector.DrawFilledRect(screen, barX, float32(sectionY), dbBarW, dbBarH, color.RGBA{40, 40, 40, 255}, false)
			vector.DrawFilledRect(screen, barX, float32(sectionY), dbBarW*float32(pct/100), dbBarH, barColor, false)

			text := fmt.Sprintf("ID:%-5d", b.ID)
			if b.Name != "" {
				text = fmt.Sprintf("[%s]", b.Name)
			}
			if b.Duration > 0 {
				text += fmt.Sprintf(" %.1fs", float64(b.TimeLeft)/1000)
			}
			ebitenutil.DebugPrintAt(screen, text, int(barX)+110, sectionY-1)
			sectionY += 14
		}
	}

	// Debuffs Section
	sectionY += 4
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("--- Debuffs (%d) ---", g.debuffMonitor.RawCount), int(barX), sectionY)
	sectionY += 16

	if len(g.debuffMonitor.Debuffs) == 0 {
		ebitenutil.DebugPrintAt(screen, "(none)", int(barX), sectionY)
		sectionY += 14
	} else {
		for _, d := range g.debuffMonitor.Debuffs {
			pct := float64(d.DurLeft) / float64(d.DurMax) * 100
			if pct > 100 {
				pct = 100
			}

			barColor := color.RGBA{100, 100, 100, 255}
			if d.CCName != "" {
				barColor = color.RGBA{200, 50, 50, 255}
			}

			dbBarW := float32(100)
			dbBarH := float32(10)
			vector.DrawFilledRect(screen, barX, float32(sectionY), dbBarW, dbBarH, color.RGBA{40, 40, 40, 255}, false)
			vector.DrawFilledRect(screen, barX, float32(sectionY), dbBarW*float32(pct/100), dbBarH, barColor, false)

			text := fmt.Sprintf("T:%-5d %.1fs", d.TypeID, float64(d.DurLeft)/1000)
			if d.CCName != "" {
				text = fmt.Sprintf("[%s] %.1fs", strings.ToUpper(d.CCName), float64(d.DurLeft)/1000)
			}
			ebitenutil.DebugPrintAt(screen, text, int(barX)+110, sectionY-1)
			sectionY += 14
		}
	}

	// Events
	sectionY += 8
	ebitenutil.DebugPrintAt(screen, "--- Events (!! = reacted) ---", int(barX), sectionY)
	sectionY += 16

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

	startIdx := 0
	if len(allEvents) > 6 {
		startIdx = len(allEvents) - 6
	}
	for i := startIdx; i < len(allEvents); i++ {
		ebitenutil.DebugPrintAt(screen, ui.TruncStr(allEvents[i], 50), int(barX), sectionY)
		sectionY += 13
	}

	// CONFIG PANEL
	panelY := float32(config.SCREEN_HEIGHT - 200)
	vector.DrawFilledRect(screen, 5, panelY-15, float32(config.SCREEN_WIDTH-10), 195, color.RGBA{25, 28, 35, 255}, false)
	vector.StrokeRect(screen, 5, panelY-15, float32(config.SCREEN_WIDTH-10), 195, 1, color.RGBA{50, 55, 65, 255}, false)

	ebitenutil.DebugPrintAt(screen, "=== CONFIG === [F3] CC Break | [F4] Buff Break", 10, int(panelY)-12)

	// Buttons
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

	// Sliders
	g.desertFire.Slider.Draw(screen)
	dfBtnColor := color.RGBA{60, 80, 40, 255}
	dfHoverColor := color.RGBA{80, 100, 50, 255}
	if !g.desertFire.Enabled {
		dfBtnColor = color.RGBA{60, 50, 50, 255}
	}
	g.desertFire.ToggleBtn.Draw(screen, dfBtnColor, dfHoverColor)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Used:%d", g.desertFire.UseCount), 380, int(g.desertFire.Slider.Y)-3)

	g.nuiNova.Slider.Draw(screen)
	nnBtnColor := color.RGBA{50, 40, 80, 255}
	nnHoverColor := color.RGBA{70, 50, 100, 255}
	if !g.nuiNova.Enabled {
		nnBtnColor = color.RGBA{50, 45, 55, 255}
	}
	g.nuiNova.ToggleBtn.Draw(screen, nnBtnColor, nnHoverColor)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Used:%d", g.nuiNova.UseCount), 380, int(g.nuiNova.Slider.Y)-3)

	// Whitelists info
	wlY := int(panelY) + 95
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("CC Whitelist: %d | Buff Whitelist: %d",
		len(g.debuffMonitor.CCWhitelist.Entries), len(g.buffMonitor.Whitelist.Entries)), 10, wlY)
	wlY += 14
	ebitenutil.DebugPrintAt(screen, "Teclas: F1, SHIFT+1, CTRL+ALT+F1...", 10, wlY)

	// Entity List
	listX := config.SCREEN_WIDTH - 180
	listY := 50
	ebitenutil.DebugPrintAt(screen, "--- Nearby ---", listX, listY)
	listY += 14

	for i, e := range entities {
		if i >= 8 {
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("+%d more", len(entities)-8), listX, listY)
			break
		}
		typeColor := color.RGBA{255, 220, 50, 255}
		typeChar := "N"
		if e.IsPlayer {
			typeColor = color.RGBA{255, 60, 60, 255}
			typeChar = "P"
		}
		vector.DrawFilledCircle(screen, float32(listX-8), float32(listY+5), 3, typeColor, false)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("[%s] %-8s %.0fm", typeChar, ui.TruncStr(e.Name, 8), e.Distance), listX, listY)
		listY += 13
	}

	// FPS
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("FPS: %.0f", ebiten.ActualFPS()), config.SCREEN_WIDTH-70, 10)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return config.SCREEN_WIDTH, config.SCREEN_HEIGHT
}
