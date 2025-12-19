package game

import (
    "fmt"
    "image/color"
    "muletinha/config"
    "muletinha/entity"
    "muletinha/input"
    "muletinha/memory"
    "muletinha/monitor"
    "muletinha/mount"
    "muletinha/process"
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
    mountConfig *mount.MountConfig

    autoPotEnabled  bool
    masterToggleBtn *ui.Button

    // HP Potions
    desertFire *ui.PotionConfig
    nuiNova    *ui.PotionConfig

    // Mana Potions
    mossyPool    *ui.PotionConfig
    krakenMight  *ui.PotionConfig

    debuffMonitor    *monitor.DebuffMonitor
    debuffMonitorBtn *ui.Button
    ccBreakBtn       *ui.Button

    buffMonitor    *monitor.BuffMonitor
    buffMonitorBtn *ui.Button
    buffBreakBtn   *ui.Button

    buffFreezeEnabled bool
    buffFreezeValue   uint32
    buffFreezeBtn     *ui.Button

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
        mountConfig:        mount.NewMountConfig(),
        entities:           make([]entity.Entity, 0, 100),
        buffFreezeEnabled:  false,
        buffFreezeValue:    0,
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
        buffFreezeBtn: &ui.Button{
            X: 550, Y: 0, W: 100, H: 22,
            Label: "Freeze:OFF",
        },
        // HP Potions
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
		mossyPool: &ui.PotionConfig{
			Name:      "Mossy Pool",
			KeyCombo:  input.ParseKeyCombo("CTRL+9"),
			Threshold: 0.50,
			Cooldown:  1500 * time.Millisecond,
			Enabled:   true,
			Slider: &ui.Slider{
				X: 550, Y: 0, W: 250, H: 14,
				Value: 0.50,
				Color: color.RGBA{50, 150, 255, 255},
				Label: "Mossy Pool (Ctrl+9)",
			},
			ToggleBtn: &ui.Button{X: 810, Y: 0, W: 50, H: 20, Label: "ON"},
		},
		krakenMight: &ui.PotionConfig{
			Name:      "Kraken's Might",
			KeyCombo:  input.ParseKeyCombo("CTRL+0"),
			Threshold: 0.20,
			Cooldown:  30 * time.Second,
			Enabled:   true,
			Slider: &ui.Slider{
				X: 550, Y: 0, W: 250, H: 14,
				Value: 0.20,
				Color: color.RGBA{100, 200, 255, 255},
				Label: "Kraken's Might (Ctrl+0)",
			},
			ToggleBtn: &ui.Button{X: 810, Y: 0, W: 50, H: 20, Label: "ON"},
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

    fmt.Printf("[INFO] x2game.dll base: %08X\n", x2game)

    return g
}

func (g *Game) GetHandle() windows.Handle {
    return g.handle
}

func sendKeyPotion(combo input.KeyCombo) {
    input.SendKeyCombo(combo)
}

func (g *Game) checkAndUsePotion() {
    if !g.autoPotEnabled || g.localPlayer.Address == 0 {
        return
    }

    now := time.Now()

    // === HP POTIONS ===
    if g.localPlayer.MaxHP > 0 {
        hpPercent := float32(g.localPlayer.HP) / float32(g.localPlayer.MaxHP)

        g.desertFire.Threshold = g.desertFire.Slider.Value
        g.nuiNova.Threshold = g.nuiNova.Slider.Value

        // Nui's Nova (emergency)
        if g.nuiNova.Enabled && hpPercent <= g.nuiNova.Threshold {
            if now.Sub(g.nuiNova.LastUsed) >= g.nuiNova.Cooldown {
                go sendKeyPotion(g.nuiNova.KeyCombo)
                g.nuiNova.LastUsed = now
                g.nuiNova.UseCount++
                return
            }
        }

        // Desert Fire (regular)
        if g.desertFire.Enabled && hpPercent <= g.desertFire.Threshold {
            if now.Sub(g.desertFire.LastUsed) >= g.desertFire.Cooldown {
                go sendKeyPotion(g.desertFire.KeyCombo)
                g.desertFire.LastUsed = now
                g.desertFire.UseCount++
            }
        }
    }

    // === MANA POTIONS ===
    if g.localPlayer.MaxMP > 0 {
        mpPercent := float32(g.localPlayer.MP) / float32(g.localPlayer.MaxMP)

        g.mossyPool.Threshold = g.mossyPool.Slider.Value
        g.krakenMight.Threshold = g.krakenMight.Slider.Value

        // Kraken's Might (emergency)
        if g.krakenMight.Enabled && mpPercent <= g.krakenMight.Threshold {
            if now.Sub(g.krakenMight.LastUsed) >= g.krakenMight.Cooldown {
                go sendKeyPotion(g.krakenMight.KeyCombo)
                g.krakenMight.LastUsed = now
                g.krakenMight.UseCount++
                return
            }
        }

        // Mossy Pool (regular)
        if g.mossyPool.Enabled && mpPercent <= g.mossyPool.Threshold {
            if now.Sub(g.mossyPool.LastUsed) >= g.mossyPool.Cooldown {
                go sendKeyPotion(g.mossyPool.KeyCombo)
                g.mossyPool.LastUsed = now
                g.mossyPool.UseCount++
            }
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

// getBuffFreezeAddress resolves the pointer chain for buff freeze
// x2game.dll+01325640 -> +0x4 -> +0x20 -> +0x8 -> +0x384
func (g *Game) getBuffFreezeAddress() uintptr {
    ptr1 := memory.ReadU32(g.handle, g.x2game+config.PTR_BUFF_FREEZE)
    if ptr1 == 0 {
        return 0
    }

    ptr2 := memory.ReadU32(g.handle, uintptr(ptr1)+uintptr(config.OFF_BUFF_FREEZE_PTR1))
    if ptr2 == 0 {
        return 0
    }

    ptr3 := memory.ReadU32(g.handle, uintptr(ptr2)+uintptr(config.OFF_BUFF_FREEZE_PTR2))
    if ptr3 == 0 {
        return 0
    }

    ptr4 := memory.ReadU32(g.handle, uintptr(ptr3)+uintptr(config.OFF_BUFF_FREEZE_PTR3))
    if ptr4 == 0 {
        return 0
    }

    return uintptr(ptr4) + uintptr(config.OFF_BUFF_FREEZE_FINAL)
}

// freezeBuffValue writes the frozen value to the buff count address
func (g *Game) freezeBuffValue() {
    if !g.buffFreezeEnabled {
        return
    }

    addr := g.getBuffFreezeAddress()
    if addr == 0 {
        return
    }

    memory.WriteU32(g.handle, addr, g.buffFreezeValue)
}

// readBuffFreezeValue reads the current value at the freeze address
func (g *Game) readBuffFreezeValue() uint32 {
    addr := g.getBuffFreezeAddress()
    if addr == 0 {
        return 0
    }
    return memory.ReadU32(g.handle, addr)
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

    // Hover states
    g.masterToggleBtn.Hovered = g.masterToggleBtn.Contains(g.mouseX, g.mouseY)
    g.debuffMonitorBtn.Hovered = g.debuffMonitorBtn.Contains(g.mouseX, g.mouseY)
    g.ccBreakBtn.Hovered = g.ccBreakBtn.Contains(g.mouseX, g.mouseY)
    g.buffMonitorBtn.Hovered = g.buffMonitorBtn.Contains(g.mouseX, g.mouseY)
    g.buffBreakBtn.Hovered = g.buffBreakBtn.Contains(g.mouseX, g.mouseY)
    g.buffFreezeBtn.Hovered = g.buffFreezeBtn.Contains(g.mouseX, g.mouseY)
    g.desertFire.ToggleBtn.Hovered = g.desertFire.ToggleBtn.Contains(g.mouseX, g.mouseY)
    g.nuiNova.ToggleBtn.Hovered = g.nuiNova.ToggleBtn.Contains(g.mouseX, g.mouseY)
    g.mossyPool.ToggleBtn.Hovered = g.mossyPool.ToggleBtn.Contains(g.mouseX, g.mouseY)
    g.krakenMight.ToggleBtn.Hovered = g.krakenMight.ToggleBtn.Contains(g.mouseX, g.mouseY)

    if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
        // Master toggle
        if g.masterToggleBtn.Contains(g.mouseX, g.mouseY) {
            g.autoPotEnabled = !g.autoPotEnabled
            if g.autoPotEnabled {
                g.masterToggleBtn.Label = "AutoPot:ON"
            } else {
                g.masterToggleBtn.Label = "AutoPot:OFF"
            }
        }

        // Debuff monitor
        if g.debuffMonitorBtn.Contains(g.mouseX, g.mouseY) {
            g.debuffMonitor.Enabled = !g.debuffMonitor.Enabled
            if g.debuffMonitor.Enabled {
                g.debuffMonitorBtn.Label = "Debuff:ON"
            } else {
                g.debuffMonitorBtn.Label = "Debuff:OFF"
            }
        }

        // CC Break
        if g.ccBreakBtn.Contains(g.mouseX, g.mouseY) {
            g.debuffMonitor.CCWhitelist.Enabled = !g.debuffMonitor.CCWhitelist.Enabled
            if g.debuffMonitor.CCWhitelist.Enabled {
                g.ccBreakBtn.Label = "CCBreak:ON"
            } else {
                g.ccBreakBtn.Label = "CCBreak:OFF"
            }
        }

        // Buff monitor
        if g.buffMonitorBtn.Contains(g.mouseX, g.mouseY) {
            g.buffMonitor.Enabled = !g.buffMonitor.Enabled
            if g.buffMonitor.Enabled {
                g.buffMonitorBtn.Label = "Buff:ON"
            } else {
                g.buffMonitorBtn.Label = "Buff:OFF"
            }
        }

        // Buff break
        if g.buffBreakBtn.Contains(g.mouseX, g.mouseY) {
            g.buffMonitor.Whitelist.Enabled = !g.buffMonitor.Whitelist.Enabled
            if g.buffMonitor.Whitelist.Enabled {
                g.buffBreakBtn.Label = "BuffBrk:ON"
            } else {
                g.buffBreakBtn.Label = "BuffBrk:OFF"
            }
        }

        // Buff Freeze toggle
        if g.buffFreezeBtn.Contains(g.mouseX, g.mouseY) {
            if !g.buffFreezeEnabled {
                // Ativando: captura o valor atual para congelar
                g.buffFreezeValue = g.readBuffFreezeValue()
                g.buffFreezeEnabled = true
                g.buffFreezeBtn.Label = fmt.Sprintf("Freeze:%d", g.buffFreezeValue)
                fmt.Printf("[FREEZE] ON - Value: %d\n", g.buffFreezeValue)
            } else {
                g.buffFreezeEnabled = false
                g.buffFreezeBtn.Label = "Freeze:OFF"
                fmt.Println("[FREEZE] OFF")
            }
        }

        // HP Potion toggles
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

        // Mana Potion toggles
        if g.mossyPool.ToggleBtn.Contains(g.mouseX, g.mouseY) {
            g.mossyPool.Enabled = !g.mossyPool.Enabled
            if g.mossyPool.Enabled {
                g.mossyPool.ToggleBtn.Label = "ON"
            } else {
                g.mossyPool.ToggleBtn.Label = "OFF"
            }
        }

        if g.krakenMight.ToggleBtn.Contains(g.mouseX, g.mouseY) {
            g.krakenMight.Enabled = !g.krakenMight.Enabled
            if g.krakenMight.Enabled {
                g.krakenMight.ToggleBtn.Label = "ON"
            } else {
                g.krakenMight.ToggleBtn.Label = "OFF"
            }
        }

        // Sliders
        if g.desertFire.Slider.Contains(g.mouseX, g.mouseY) {
            g.desertFire.Slider.Dragging = true
        }
        if g.nuiNova.Slider.Contains(g.mouseX, g.mouseY) {
            g.nuiNova.Slider.Dragging = true
        }
        if g.mossyPool.Slider.Contains(g.mouseX, g.mouseY) {
            g.mossyPool.Slider.Dragging = true
        }
        if g.krakenMight.Slider.Contains(g.mouseX, g.mouseY) {
            g.krakenMight.Slider.Dragging = true
        }
    }

    if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
        if g.desertFire.Slider.Dragging {
            g.desertFire.Slider.SetValueFromX(g.mouseX)
        }
        if g.nuiNova.Slider.Dragging {
            g.nuiNova.Slider.SetValueFromX(g.mouseX)
        }
        if g.mossyPool.Slider.Dragging {
            g.mossyPool.Slider.SetValueFromX(g.mouseX)
        }
        if g.krakenMight.Slider.Dragging {
            g.krakenMight.Slider.SetValueFromX(g.mouseX)
        }
    } else {
        g.desertFire.Slider.Dragging = false
        g.nuiNova.Slider.Dragging = false
        g.mossyPool.Slider.Dragging = false
        g.krakenMight.Slider.Dragging = false
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

    // F5 - Buff Freeze toggle
    if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
        if !g.buffFreezeEnabled {
            g.buffFreezeValue = g.readBuffFreezeValue()
            g.buffFreezeEnabled = true
            g.buffFreezeBtn.Label = fmt.Sprintf("Freeze:%d", g.buffFreezeValue)
            fmt.Printf("[FREEZE] ON - Value: %d\n", g.buffFreezeValue)
        } else {
            g.buffFreezeEnabled = false
            g.buffFreezeBtn.Label = "Freeze:OFF"
            fmt.Println("[FREEZE] OFF")
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

    // Freeze buff value every frame if enabled
    g.freezeBuffValue()

    if g.frameCount%5 == 0 {
        g.localPlayer = entity.GetLocalPlayer(g.handle, g.x2game)
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