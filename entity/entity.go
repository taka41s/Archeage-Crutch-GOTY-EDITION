package entity

import (
	"fmt"
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
	Distance float32
	VTable   uint32
	IsPlayer bool
	IsNPC    bool
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

	return player
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

	// ADICIONA DEBUG AQUI
	DebugPrintEntities(entities)

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

	// DEBUG das entidades filtradas também
	fmt.Println("\n=== ENTIDADES FILTRADAS ===")
	DebugPrintEntities(filtered)

	return filtered
}

// DebugPrintEntities imprime informações de debug das entidades
func DebugPrintEntities(entities []Entity) {
	fmt.Println("\n========================================")
	fmt.Println("DEBUG: Entidades Encontradas")
	fmt.Println("========================================")
	
	if len(entities) == 0 {
		fmt.Println("Nenhuma entidade encontrada")
		return
	}
	
	for i, e := range entities {
		fmt.Println("----------------------------------------")
		fmt.Printf("[%d] Entidade:\n", i+1)
		fmt.Printf("  Endereço: 0x%08X\n", e.Address)
		fmt.Printf("  Nome: %s\n", e.Name)
		fmt.Printf("  HP: %d / %d", e.HP, e.MaxHP)
		
		// Calcula porcentagem de HP
		if e.MaxHP > 0 {
			percentage := float32(e.HP) * 100 / float32(e.MaxHP)
			fmt.Printf(" (%.1f%%)", percentage)
		}
		fmt.Println()
		
		fmt.Printf("  Posição: X=%.2f, Y=%.2f, Z=%.2f\n", e.PosX, e.PosY, e.PosZ)
		fmt.Printf("  Distância: %.2f metros\n", e.Distance)
		fmt.Printf("  VTable: 0x%08X\n", e.VTable)
		
		entityType := "Unknown"
		if e.IsPlayer {
			entityType = "Player"
		} else if e.IsNPC {
			entityType = "NPC"
		}
		fmt.Printf("  Tipo: %s\n", entityType)
	}
	
	fmt.Println("========================================")
	fmt.Printf("Total de entidades: %d\n", len(entities))
	fmt.Println("========================================\n")
}

// Função adicional para debug compacto (uma linha por entidade)
func DebugPrintEntitiesCompact(entities []Entity) {
	fmt.Println("\n=== DEBUG COMPACTO ===")
	fmt.Printf("%-10s | %-20s | %-10s | %-30s | %-10s\n", 
		"ENDEREÇO", "NOME", "HP", "POSIÇÃO (X,Y,Z)", "DISTÂNCIA")
	fmt.Println(strings.Repeat("-", 90))
	
	for _, e := range entities {
		hpInfo := fmt.Sprintf("%d/%d", e.HP, e.MaxHP)
		posInfo := fmt.Sprintf("%.1f, %.1f, %.1f", e.PosX, e.PosY, e.PosZ)
		fmt.Printf("0x%08X | %-20s | %-10s | %-30s | %.2fm\n",
			e.Address, e.Name, hpInfo, posInfo, e.Distance)
	}
	fmt.Printf("\nTotal: %d entidades\n", len(entities))
}