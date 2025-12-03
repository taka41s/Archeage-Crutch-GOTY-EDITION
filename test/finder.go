package entity

import (
	"fmt"
	"muletinha/memory"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Estruturas comuns em entity lists
type EntityListPatterns struct {
	// Padrão 1: Array/Vector de ponteiros
	ArrayPattern struct {
		Count    uint32
		Capacity uint32
		Items    []uint32 // Ponteiros para entidades
	}
	
	// Padrão 2: Lista ligada
	LinkedListPattern struct {
		First uint32
		Last  uint32
		Count uint32
	}
	
	// Padrão 3: Hash table/map
	HashMapPattern struct {
		Buckets  uint32
		Size     uint32
		Capacity uint32
	}
}

// FindEntityListByVTable tenta encontrar a entity list usando a VTable conhecida
func FindEntityListByVTable(handle windows.Handle, knownVTable uint32, knownEntities []uint32) {
	fmt.Printf("\n=== PROCURANDO ENTITY LIST ===\n")
	fmt.Printf("VTable alvo: 0x%08X\n", knownVTable)
	fmt.Printf("Entidades conhecidas: %d\n\n", len(knownEntities))
	
	// 1. Primeiro, vamos verificar se há um padrão de distância entre entidades
	analyzeEntityMemoryPattern(knownEntities)
	
	// 2. Procurar por arrays/vetores que contenham ponteiros para as entidades
	findEntityArray(handle, knownEntities)
	
	// 3. Verificar se é uma lista ligada
	checkLinkedListPattern(handle, knownEntities)
	
	// 4. Procurar referências cruzadas
	findCrossReferences(handle, knownEntities)
}

// analyzeEntityMemoryPattern analisa o padrão de memória das entidades
func analyzeEntityMemoryPattern(entities []uint32) {
	fmt.Println("=== ANÁLISE DE PADRÃO DE MEMÓRIA ===")
	
	if len(entities) < 2 {
		return
	}
	
	// Calcula distâncias entre entidades consecutivas
	var distances []int32
	for i := 1; i < len(entities); i++ {
		dist := int32(entities[i]) - int32(entities[i-1])
		distances = append(distances, dist)
		fmt.Printf("Distância [%d]->[%d]: 0x%X (%d bytes)\n", 
			i-1, i, dist, dist)
	}
	
	// Verifica se há um padrão consistente
	if len(distances) > 0 {
		// Encontra a distância mais comum
		distCount := make(map[int32]int)
		for _, d := range distances {
			distCount[d]++
		}
		
		var mostCommon int32
		maxCount := 0
		for dist, count := range distCount {
			if count > maxCount {
				maxCount = count
				mostCommon = dist
			}
		}
		
		if maxCount > 1 {
			fmt.Printf("\nPadrão encontrado! Distância comum: 0x%X (%d bytes) aparece %d vezes\n",
				mostCommon, mostCommon, maxCount)
			
			// Se o padrão é consistente, pode ser um array alocado
			if mostCommon > 0 && mostCommon < 0x10000 {
				fmt.Printf("Possível tamanho da estrutura Entity: %d bytes\n", mostCommon)
			}
		}
	}
	fmt.Println()
}

// findEntityArray procura arrays que contenham ponteiros para as entidades
func findEntityArray(handle windows.Handle, entities []uint32) {
	fmt.Println("=== PROCURANDO ARRAYS DE ENTIDADES ===")
	
	if len(entities) < 2 {
		return
	}
	
	// Procura na memória por sequências de ponteiros para as entidades
	searchRegions := []struct {
		start uint32
		end   uint32
	}{
		{0x00400000, 0x10000000}, // Região do executável e heap baixo
		{0x30000000, 0x40000000}, // Região comum para dados do jogo
		{0x50000000, 0x70000000}, // Outra região comum
	}
	
	buffer := make([]byte, 0x10000)
	
	for _, region := range searchRegions {
		for addr := region.start; addr < region.end; addr += 0x10000 {
			var bytesRead uintptr
			ret, _, _ := memory.ProcReadProcessMemory.Call(
				uintptr(handle), uintptr(addr),
				uintptr(unsafe.Pointer(&buffer[0])),
				0x10000, uintptr(unsafe.Pointer(&bytesRead)),
			)
			
			if ret == 0 || bytesRead < 8 {
				continue
			}
			
			// Procura por ponteiros consecutivos para nossas entidades
			for i := uint32(0); i < uint32(bytesRead)-8; i += 4 {
				ptr1 := *(*uint32)(unsafe.Pointer(&buffer[i]))
				ptr2 := *(*uint32)(unsafe.Pointer(&buffer[i+4]))
				
				// Verifica se encontramos dois ponteiros consecutivos para entidades
				found1, found2 := false, false
				for _, entity := range entities {
					if ptr1 == entity {
						found1 = true
					}
					if ptr2 == entity {
						found2 = true
					}
				}
				
				if found1 && found2 {
					fmt.Printf("🎯 POSSÍVEL ARRAY encontrado em 0x%08X!\n", addr+i)
					
					// Vamos ver quantos ponteiros válidos existem aqui
					countValidPointers(handle, addr+i, entities)
					
					// Procura por um contador antes do array
					checkForCounter(handle, addr+i)
				}
			}
		}
	}
	fmt.Println()
}

// checkLinkedListPattern verifica se as entidades formam uma lista ligada
func checkLinkedListPattern(handle windows.Handle, entities []uint32) {
	fmt.Println("=== VERIFICANDO PADRÃO DE LISTA LIGADA ===")
	
	for _, entity := range entities[:min(5, len(entities))] { // Verifica as primeiras 5
		fmt.Printf("\nAnalisando entidade 0x%08X:\n", entity)
		
		// Lê os primeiros bytes da entidade procurando por ponteiros
		buffer := make([]byte, 0x100)
		var bytesRead uintptr
		ret, _, _ := memory.ProcReadProcessMemory.Call(
			uintptr(handle), uintptr(entity),
			uintptr(unsafe.Pointer(&buffer[0])),
			0x100, uintptr(unsafe.Pointer(&bytesRead)),
		)
		
		if ret == 0 {
			continue
		}
		
		// Procura por ponteiros que apontem para outras entidades conhecidas
		for offset := uint32(0); offset < uint32(bytesRead)-4; offset += 4 {
			ptr := *(*uint32)(unsafe.Pointer(&buffer[offset]))
			
			for _, otherEntity := range entities {
				if ptr == otherEntity && ptr != entity {
					fmt.Printf("  Offset 0x%X aponta para outra entidade: 0x%08X\n", 
						offset, ptr)
					
					// Possíveis offsets para next/prev em lista ligada
					if offset == 0x4 || offset == 0x8 || offset == 0xC {
						fmt.Printf("    → Possível ponteiro NEXT em offset 0x%X\n", offset)
					}
					if offset == 0x0 || offset == 0x10 {
						fmt.Printf("    → Possível ponteiro PREV em offset 0x%X\n", offset)
					}
				}
			}
		}
	}
	fmt.Println()
}

// findCrossReferences procura referências cruzadas para as entidades
func findCrossReferences(handle windows.Handle, entities []uint32) {
	fmt.Println("=== PROCURANDO REFERÊNCIAS CRUZADAS ===")
	
	// Pega uma amostra de entidades para procurar
	sample := entities[:min(3, len(entities))]
	
	searchRegions := []struct {
		start uint32
		end   uint32
		name  string
	}{
		{0x00400000, 0x01000000, "Código do executável"},
		{0x30000000, 0x40000000, "Dados estáticos"},
		{0x10000000, 0x30000000, "Heap baixo"},
	}
	
	for _, entity := range sample {
		fmt.Printf("\nProcurando referências para 0x%08X...\n", entity)
		foundRefs := []uint32{}
		
		buffer := make([]byte, 0x10000)
		
		for _, region := range searchRegions {
			for addr := region.start; addr < region.end && len(foundRefs) < 10; addr += 0x10000 {
				var bytesRead uintptr
				ret, _, _ := memory.ProcReadProcessMemory.Call(
					uintptr(handle), uintptr(addr),
					uintptr(unsafe.Pointer(&buffer[0])),
					0x10000, uintptr(unsafe.Pointer(&bytesRead)),
				)
				
				if ret == 0 || bytesRead < 4 {
					continue
				}
				
				for i := uint32(0); i < uint32(bytesRead)-4; i += 4 {
					ptr := *(*uint32)(unsafe.Pointer(&buffer[i]))
					if ptr == entity {
						refAddr := addr + i
						foundRefs = append(foundRefs, refAddr)
						fmt.Printf("  Referência encontrada em 0x%08X (%s)\n", 
							refAddr, region.name)
					}
				}
			}
		}
		
		if len(foundRefs) > 1 {
			fmt.Printf("  Total de referências: %d\n", len(foundRefs))
		}
	}
}

// Funções auxiliares
func countValidPointers(handle windows.Handle, arrayAddr uint32, knownEntities []uint32) {
	fmt.Printf("  Contando ponteiros válidos em 0x%08X...\n", arrayAddr)
	
	buffer := make([]byte, 0x400) // Lê 1KB
	var bytesRead uintptr
	ret, _, _ := memory.ProcReadProcessMemory.Call(
		uintptr(handle), uintptr(arrayAddr),
		uintptr(unsafe.Pointer(&buffer[0])),
		0x400, uintptr(unsafe.Pointer(&bytesRead)),
	)
	
	if ret == 0 {
		return
	}
	
	validCount := 0
	for i := uint32(0); i < uint32(bytesRead)-4; i += 4 {
		ptr := *(*uint32)(unsafe.Pointer(&buffer[i]))
		for _, entity := range knownEntities {
			if ptr == entity {
				validCount++
				if validCount <= 5 { // Mostra os primeiros 5
					fmt.Printf("    [%d] 0x%08X ✓\n", validCount, ptr)
				}
				break
			}
		}
	}
	
	fmt.Printf("  Total de ponteiros válidos encontrados: %d\n", validCount)
}

func checkForCounter(handle windows.Handle, arrayAddr uint32) {
	// Verifica se há um contador antes do array
	for offset := uint32(4); offset <= 16; offset += 4 {
		count := memory.ReadU32(handle, uintptr(arrayAddr-offset))
		if count > 0 && count < 10000 { // Valor razoável para contador
			fmt.Printf("  Possível contador em -0x%X: %d\n", offset, count)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FindEntityManager procura pelo gerenciador de entidades usando a VTable
func FindEntityManager(handle windows.Handle, vtable uint32) uint32 {
	fmt.Printf("\n=== PROCURANDO ENTITY MANAGER ===\n")
	fmt.Printf("Procurando gerenciador com VTable: 0x%08X\n", vtable)
	
	// Estratégia: procurar por estruturas que parecem gerenciadores
	// Geralmente contêm: contador, capacidade, ponteiro para array
	
	searchRegions := []uint32{
		0x00400000, // Base do executável
		0x30000000, // Região de dados
		0x10000000, // Heap
	}
	
	buffer := make([]byte, 0x1000)
	
	for _, baseAddr := range searchRegions {
		for offset := uint32(0); offset < 0x100000; offset += 0x1000 {
			addr := baseAddr + offset
			var bytesRead uintptr
			ret, _, _ := memory.ProcReadProcessMemory.Call(
				uintptr(handle), uintptr(addr),
				uintptr(unsafe.Pointer(&buffer[0])),
				0x1000, uintptr(unsafe.Pointer(&bytesRead)),
			)
			
			if ret == 0 || bytesRead < 12 {
				continue
			}
			
			// Procura por padrão de manager: [count][capacity][pointer]
			for i := uint32(0); i < uint32(bytesRead)-12; i += 4 {
				count := *(*uint32)(unsafe.Pointer(&buffer[i]))
				capacity := *(*uint32)(unsafe.Pointer(&buffer[i+4]))
				pointer := *(*uint32)(unsafe.Pointer(&buffer[i+8]))
				
				// Validações básicas
				if count > 0 && count <= 10000 &&
				   capacity >= count && capacity <= 10000 &&
				   pointer > 0x10000000 && pointer < 0xF0000000 {
					
					// Verifica se o pointer aponta para dados válidos
					testBuf := make([]byte, 4)
					var testRead uintptr
					ret, _, _ := memory.ProcReadProcessMemory.Call(
						uintptr(handle), uintptr(pointer),
						uintptr(unsafe.Pointer(&testBuf[0])),
						4, uintptr(unsafe.Pointer(&testRead)),
					)
					
					if ret != 0 && testRead == 4 {
						fmt.Printf("📍 Possível Entity Manager em 0x%08X:\n", addr+i)
						fmt.Printf("   Count: %d, Capacity: %d, Pointer: 0x%08X\n",
							count, capacity, pointer)
					}
				}
			}
		}
	}
	
	return 0
}