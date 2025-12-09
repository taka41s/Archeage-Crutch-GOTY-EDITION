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
	"muletinha/mount"
	"muletinha/ui"
	"sync"
	"time"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"golang.org/x/sys/windows"
)

var (
	debuffBuffer = make([]byte, 30*config.DEBUFF_SIZE)
	buffBuffer   = make([]byte, 30*config.BUFF_SIZE)
)

type Game struct {
	handle      windows.Handle
	x2game      uintptr
	icudt42     uintptr
	localPlayer entity.Entity
	playerMount entity.Entity
	entities    []entity.Entity
	mutex       sync.RWMutex
	connected   bool
	frameCount  int
	mountConfig   *mount.MountConfig

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

	g := &Game{
		autoPotEnabled:     true,
		debuffMonitor:      monitor.NewDebuffMonitor(),
		buffMonitor:        monitor.NewBuffMonitor(),
		entityScanInterval: 1000 * time.Millisecond,
		mountConfig: mount.NewMountConfig(),
		entities:           make([]entity.Entity, 0, 100),
		masterToggleBtn: &ui.Button{
			X: 25, Y: 0, W: 100, H: 22,
			Label: "AutoPot:ON",
		},
		debuffMonitorBtn: &ui.Button{
			X: 130, Y: 0, W: 100, H: 22,
			Label: "Debuff:ON",
		},
		ccBreakBtn: &ui.Button{
			X: 235, Y: 0, W: 100, H: 22,
			Label: "CCBreak:ON",
		},
		buffMonitorBtn: &ui.Button{
			X: 340, Y: 0, W: 100, H: 22,
			Label: "Buff:ON",
		},
		buffBreakBtn: &ui.Button{
			X: 445, Y: 0, W: 100, H: 22,
			Label: "BuffBrk:ON",
		},
		desertFire: &ui.PotionConfig{
			Name:      "Desert Fire",
			KeyCombo:  input.ParseKeyCombo("F1"),
			Threshold: 0.60,
			Cooldown:  1500 * time.Millisecond,
			Enabled:   true,
			Slider: &ui.Slider{
				X: 120, Y: 0, W: 250, H: 14,
				Value: 0.60,
				Color: color.RGBA{255, 150, 50, 255},
				Label: "Desert Fire (F1)",
			},
			ToggleBtn: &ui.Button{X: 380, Y: 0, W: 50, H: 20, Label: "ON"},
		},
		nuiNova: &ui.PotionConfig{
			Name:      "Nui's Nova",
			KeyCombo:  input.ParseKeyCombo("F2"),
			Threshold: 0.20,
			Cooldown:  30 * time.Second,
			Enabled:   true,
			Slider: &ui.Slider{
				X: 120, Y: 0, W: 250, H: 14,
				Value: 0.20,
				Color: color.RGBA{150, 100, 255, 255},
				Label: "Nui's Nova (F2)",
			},
			ToggleBtn: &ui.Button{X: 380, Y: 0, W: 50, H: 20, Label: "ON"},
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

	icudt42, err := process.GetModuleBase(pid, "icudt42.dll")
	if err != nil {
		fmt.Println("icudt42.dll não encontrado!")
		windows.CloseHandle(handle)
		return g
	}

	g.handle = handle
	g.x2game = x2game
	g.icudt42 = icudt42 
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

	listPtr := memory.ReadU32(g.handle, uintptr(base)+config.OFF_DEBUFF_PTR)
	if !memory.IsValidPtr(listPtr) {
		return 0
	}

	g.cachedBuffListAddr = uintptr(listPtr)
	g.lastBuffCheck = time.Now()
	return g.cachedBuffListAddr
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

	g.updateDebuffsInstant()
	g.updateBuffsInstant()

	if g.frameCount%5 == 0 {
		g.localPlayer = entity.GetLocalPlayer(g.handle, g.x2game)
		g.playerMount = entity.GetPlayerMount(g.handle, g.icudt42)

		// g.handleAutoMount() DISABLED FOR NOW
		g.checkAndUsePotion()
	}

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

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return config.SCREEN_WIDTH, config.SCREEN_HEIGHT
}


func (g *Game) handleAutoMount() {
	g.debugMountOwner()
	g.mountConfig.Update(g.playerMount.Address, g.playerMount.Name)
}

func (g *Game) debugMountOwner() {
	if g.playerMount.Address == 0 || g.localPlayer.Address == 0 {
		fmt.Println("[Mount] Precisa estar montado")
		return
	}

	mountAddr := uintptr(g.playerMount.Address)
	playerAddr := g.localPlayer.Address
	
	fmt.Printf("\n[Mount Owner Search] mount=0x%X player=0x%X\n", mountAddr, playerAddr)
	fmt.Println("Procurando por ponteiro pro player...")

	for offset := uint32(0); offset < 0x200; offset += 0x4 {
		val := memory.ReadU32(g.handle, mountAddr+uintptr(offset))

		if val == playerAddr {
			fmt.Printf("★ ENCONTRADO! Offset 0x%X = 0x%X (player address)\n", offset, val)
		}
	}

	fmt.Println("\nPrimeiros offsets da mount:")
	for offset := uint32(0); offset < 0x80; offset += 0x4 {
		val := memory.ReadU32(g.handle, mountAddr+uintptr(offset))
		if val != 0 {
			fmt.Printf("0x%02X: 0x%08X\n", offset, val)
		}
	}
}