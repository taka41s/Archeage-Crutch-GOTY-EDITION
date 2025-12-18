package entity

import (
	"muletinha/config"
	"muletinha/memory"
	"sort"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Entity struct {
	Address  uint32
	Name     string
	PosX     float32
	PosY     float32
	PosZ     float32
	HP       uint32
	MaxHP    uint32
	MP       uint32
	MaxMP    uint32
	Distance float32
	VTable   uint32
	IsPlayer bool
	IsNPC    bool
	IsMount bool
}

func GetLocalPlayer(handle windows.Handle, x2game uintptr) Entity {
	var player Entity

	ptr1 := memory.ReadU32(handle, x2game+config.PTR_LOCALPLAYER)
	if ptr1 == 0 {
		return player
	}

	player.Address = memory.ReadU32(handle, uintptr(ptr1+config.PTR_ENTITY))
	if player.Address == 0 {
		return player
	}

	player.VTable = memory.ReadU32(handle, uintptr(player.Address))
	player.Name = GetEntityName(handle, player.Address)
	player.PosX = memory.ReadF32(handle, uintptr(player.Address+config.OFF_POS_X))
	player.PosZ = memory.ReadF32(handle, uintptr(player.Address+config.OFF_POS_Z))
	player.PosY = memory.ReadF32(handle, uintptr(player.Address+config.OFF_POS_Y))
	player.HP = memory.ReadU32(handle, uintptr(player.Address+config.OFF_HP_ENTITY))
	player.MaxHP = GetMaxHP(handle, player.Address)
	player.MP, player.MaxMP = GetLocalPlayerMana(handle, x2game)

	return player
}

func GetLocalPlayerMana(handle windows.Handle, x2game uintptr) (current, max uint32) {
	p1 := memory.ReadU32(handle, x2game+config.PTR_MANA_BASE)
	if p1 == 0 {
		return 0, 0
	}

	p2 := memory.ReadU32(handle, uintptr(p1+config.OFF_MANA_PTR1))
	if p2 == 0 {
		return 0, 0
	}

	p3 := memory.ReadU32(handle, uintptr(p2+config.OFF_MANA_PTR2))
	if p3 == 0 {
		return 0, 0
	}

	p4 := memory.ReadU32(handle, uintptr(p3+config.OFF_MANA_PTR3))
	if p4 == 0 {
		return 0, 0
	}

	p5 := memory.ReadU32(handle, uintptr(p4+config.OFF_MANA_PTR4))
	if p5 == 0 {
		return 0, 0
	}

	p6 := memory.ReadU32(handle, uintptr(p5+config.OFF_MANA_PTR5))
	if p6 == 0 {
		return 0, 0
	}

	p7 := memory.ReadU32(handle, uintptr(p6+config.OFF_MANA_PTR6))
	if p7 == 0 {
		return 0, 0
	}

	current = memory.ReadU32(handle, uintptr(p7+config.OFF_MANA_CURRENT))
	max = memory.ReadU32(handle, uintptr(p7+config.OFF_MANA_MAX))

	return current, max
}

func GetMaxHP(handle windows.Handle, entityAddr uint32) uint32 {
	base := memory.ReadU32(handle, uintptr(entityAddr+config.OFF_ENTITY_BASE))
	if !memory.IsValidPtr(base) {
		return 0
	}

	esi := memory.ReadU32(handle, uintptr(base+config.OFF_TO_ESI))
	if !memory.IsValidPtr(esi) {
		return 0
	}

	stats := memory.ReadU32(handle, uintptr(esi+config.OFF_TO_STATS))
	if !memory.IsValidPtr(stats) {
		return 0
	}

	return memory.ReadU32(handle, uintptr(stats+config.OFF_MAXHP))
}

func GetEntityName(handle windows.Handle, entityAddr uint32) string {
	namePtr1 := memory.ReadU32(handle, uintptr(entityAddr+config.OFF_NAME_PTR1))
	if !memory.IsValidPtr(namePtr1) {
		return ""
	}

	namePtr2 := memory.ReadU32(handle, uintptr(namePtr1+config.OFF_NAME_PTR2))
	if !memory.IsValidPtr(namePtr2) {
		return ""
	}

	return memory.ReadString(handle, uintptr(namePtr2), 32)
}

func IsValidEntityName(name string) bool {
	if len(name) < 2 || len(name) > 32 {
		return false
	}

	alphaCount := 0
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			alphaCount++
		} else if c < 32 && c != 0 {
			return false
		}
	}
	return alphaCount >= 2
}

func FindAllEntities(handle windows.Handle, player Entity, maxDistance float32) []Entity {
	var entities []Entity

	regions := []struct {
		start uint32
		size  uint32
	}{
		{0x80000000, 0x10000000},
		{0x90000000, 0x10000000},
		{0xA0000000, 0x10000000},
		{0xB0000000, 0x10000000},
		{0xC0000000, 0x10000000},
	}

	seen := make(map[uint32]bool)
	buffer := make([]byte, 0x10000)

	for _, region := range regions {
		for offset := uint32(0); offset < region.size; offset += 0x10000 {
			addr := region.start + offset
			var bytesRead uintptr
			ret, _, _ := memory.ProcReadProcessMemory.Call(
				uintptr(handle), uintptr(addr),
				uintptr(unsafe.Pointer(&buffer[0])),
				0x10000, uintptr(unsafe.Pointer(&bytesRead)),
			)
			if ret == 0 || bytesRead < 0x1000 {
				continue
			}

			for i := uint32(0); i < uint32(bytesRead)-0x900; i += 4 {
				vtable := *(*uint32)(unsafe.Pointer(&buffer[i]))
				if vtable < 0x39000000 || vtable >= 0x3B000000 {
					continue
				}

				candidateAddr := addr + i
				if seen[candidateAddr] {
					continue
				}

				hpOffset := i + config.OFF_HP_ENTITY
				if hpOffset+4 > uint32(bytesRead) {
					continue
				}
				hp := *(*uint32)(unsafe.Pointer(&buffer[hpOffset]))
				if hp < 100 || hp > 10000000 {
					continue
				}

				posXOffset := i + config.OFF_POS_X
				posYOffset := i + config.OFF_POS_Y
				posZOffset := i + config.OFF_POS_Z
				if posZOffset+4 > uint32(bytesRead) {
					continue
				}

				posX := *(*float32)(unsafe.Pointer(&buffer[posXOffset]))
				posY := *(*float32)(unsafe.Pointer(&buffer[posYOffset]))
				posZ := *(*float32)(unsafe.Pointer(&buffer[posZOffset]))

				if !memory.IsValidCoord(posX) || !memory.IsValidCoord(posY) || !memory.IsValidCoord(posZ) {
					continue
				}

				distance := memory.CalculateDistance(player.PosX, player.PosY, player.PosZ, posX, posY, posZ)
				if distance > maxDistance {
					continue
				}

				name := GetEntityName(handle, candidateAddr)
				if !IsValidEntityName(name) {
					continue
				}

				maxHP := GetMaxHP(handle, candidateAddr)

				entities = append(entities, Entity{
					Address:  candidateAddr,
					Name:     name,
					PosX:     posX,
					PosY:     posY,
					PosZ:     posZ,
					HP:       hp,
					MaxHP:    maxHP,
					Distance: distance,
					VTable:   vtable,
				})
				seen[candidateAddr] = true
			}
		}
	}

	sort.Slice(entities, func(i, j int) bool {
		return entities[i].Distance < entities[j].Distance
	})

	return entities
}

func FilterEntities(entities []Entity, player Entity) []Entity {
	var filtered []Entity

	for _, e := range entities {
		if e.Address == player.Address {
			continue
		}

		nameLower := strings.ToLower(e.Name)
		if strings.HasPrefix(nameLower, "prefab_") ||
			strings.HasPrefix(nameLower, "object_") {
			continue
		}

		if strings.Contains(e.Name, " ") {
			e.IsNPC = true
		} else {
			e.IsPlayer = true
		}

		filtered = append(filtered, e)
	}

	return filtered
}
