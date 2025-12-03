package main

import (
	"bufio"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procReadProcessMemory = kernel32.NewProc("ReadProcessMemory")
)

func main() {
	fmt.Println("=== Entity List Finder ===")
	fmt.Println()

	pid, _ := findProcess("archeage.exe")
	handle, _ := windows.OpenProcess(0x1F0FFF, false, pid)
	defer windows.CloseHandle(handle)

	x2game, _ := getModuleBase(pid, "x2game.dll")
	fmt.Printf("x2game.dll: 0x%X\n", x2game)

	// Pegar LocalPlayer para referência
	ptr1 := readU32(handle, x2game+0xE9DC54)
	localEntity := readU32(handle, uintptr(ptr1)+0x10)
	localName := getEntityName(handle, localEntity)
	fmt.Printf("LocalPlayer: %s (0x%X)\n\n", localName, localEntity)

	// Procurar estruturas que parecem listas de entidades
	fmt.Println("Procurando Entity Lists...")

	// Scan região próxima ao LocalPlayer pointer
	scanSize := uint32(0x200000) // 2MB
	buffer := make([]byte, scanSize)

	startAddr := x2game + 0xE80000
	readMemoryBytes(handle, startAddr, buffer)

	candidatesFound := 0

	for offset := uint32(0); offset < scanSize-0x100; offset += 4 {
		ptr := bytesToUint32(buffer[offset : offset+4])
		if !isValidPtr(ptr) {
			continue
		}

		// Ler estrutura potencial
		listData := make([]byte, 0x100)
		if readMemoryBytes(handle, uintptr(ptr), listData) != nil {
			continue
		}

		// Procurar por padrões de lista
		for innerOff := uint32(0); innerOff < 0x60; innerOff += 4 {
			count := bytesToUint32(listData[innerOff : innerOff+4])

			// Count razoável para entidades (5-1000)
			if count < 5 || count > 1000 {
				continue
			}

			// Verificar próximos valores como possíveis array pointers
			for arrayOff := innerOff + 4; arrayOff < innerOff+20 && arrayOff < 0x60; arrayOff += 4 {
				arrayPtr := bytesToUint32(listData[arrayOff : arrayOff+4])

				if !isValidPtr(arrayPtr) {
					continue
				}

				// Ler o array (buffer maior para verificar mais entidades)
				maxEntities := uint32(100)
				if count < maxEntities {
					maxEntities = count
				}
				
				arrayData := make([]byte, maxEntities*4)
				if readMemoryBytes(handle, uintptr(arrayPtr), arrayData) != nil {
					continue
				}

				// Contar entidades válidas
				validEntities := 0
				localFound := false
				localIndex := -1

				for i := uint32(0); i < maxEntities; i++ {
					if i*4+4 > uint32(len(arrayData)) {
						break
					}
					
					entPtr := bytesToUint32(arrayData[i*4 : i*4+4])
					if !isValidPtr(entPtr) {
						continue
					}

					// Verificar se é entidade válida
					if isValidEntity(handle, entPtr) {
						validEntities++
						
						if entPtr == localEntity {
							localFound = true
							localIndex = int(i)
						}
					}
				}

				// Se encontrou muitas entidades válidas, é um bom candidato
				if validEntities >= 5 {
					candidatesFound++
					
					fmt.Printf("\n[CANDIDATO #%d] x2game+0x%X -> 0x%X\n", 
						candidatesFound, uint32(startAddr-x2game)+offset, ptr)
					fmt.Printf("  Count offset: +0x%X = %d\n", innerOff, count)
					fmt.Printf("  Array offset: +0x%X -> 0x%X\n", arrayOff, arrayPtr)
					fmt.Printf("  Entidades válidas: %d/%d\n", validEntities, maxEntities)
					
					if localFound {
						fmt.Printf("  *** LocalPlayer encontrado no índice %d! ***\n", localIndex)
					}

					// Mostrar algumas entidades
					fmt.Println("  Primeiras entidades:")
					shown := 0
					for i := uint32(0); i < maxEntities && shown < 10; i++ {
						if i*4+4 > uint32(len(arrayData)) {
							break
						}
						
						entPtr := bytesToUint32(arrayData[i*4 : i*4+4])
						if !isValidPtr(entPtr) {
							continue
						}

						name := getEntityName(handle, entPtr)
						hp := readU32(handle, uintptr(entPtr)+0x84C)
						
						if len(name) > 0 && hp > 0 {
							marker := ""
							if entPtr == localEntity {
								marker = " <-- YOU"
							}
							fmt.Printf("    [%d] 0x%X: %s (HP: %d)%s\n", i, entPtr, name, hp, marker)
							shown++
						}
					}
				}
			}
		}
	}

	if candidatesFound == 0 {
		fmt.Println("\nNenhum candidato encontrado. Tentando scan mais amplo...")
		scanWider(handle, x2game, localEntity)
	}

	fmt.Printf("\n\nTotal de candidatos: %d\n", candidatesFound)
	fmt.Println("\nPressione Enter para sair...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}

func scanWider(handle windows.Handle, x2game uintptr, localEntity uint32) {
	fmt.Println("\n=== Scan Amplo ===")
	
	// Procurar ponteiro direto para localEntity em x2game.dll
	scanSize := uint32(0x300000)
	buffer := make([]byte, scanSize)
	
	startAddr := x2game + 0xE00000
	readMemoryBytes(handle, startAddr, buffer)
	
	fmt.Println("Procurando referências ao LocalPlayer...")
	
	for offset := uint32(0); offset < scanSize-4; offset += 4 {
		val := bytesToUint32(buffer[offset : offset+4])
		
		// Procurar ponteiros que eventualmente levam ao LocalPlayer
		if !isValidPtr(val) {
			continue
		}
		
		// Ler o que esse ponteiro aponta
		data := make([]byte, 0x100)
		if readMemoryBytes(handle, uintptr(val), data) != nil {
			continue
		}
		
		// Verificar se algum campo contém o localEntity
		for i := uint32(0); i < 0x100-4; i += 4 {
			innerVal := bytesToUint32(data[i : i+4])
			if innerVal == localEntity {
				fmt.Printf("  [REF] x2game+0x%X -> 0x%X -> [+0x%X] = LocalPlayer\n",
					uint32(startAddr-x2game)+offset, val, i)
			}
		}
	}
}

func isValidEntity(handle windows.Handle, addr uint32) bool {
	// Ler dados básicos da entidade
	entityData := make([]byte, 0x900)
	if readMemoryBytes(handle, uintptr(addr), entityData) != nil {
		return false
	}

	// Verificar VTable
	vtable := bytesToUint32(entityData[0:4])
	if vtable < 0x30000000 || vtable > 0x45000000 {
		return false
	}

	// Verificar HP
	hp := bytesToUint32(entityData[0x84C:0x850])
	if hp < 1 || hp > 50000000 {
		return false
	}

	// Verificar posição
	posX := bytesToFloat32(entityData[0x830:0x834])
	posY := bytesToFloat32(entityData[0x838:0x83C])
	
	if !isValidCoord(posX) || !isValidCoord(posY) {
		return false
	}

	// Verificar EntityBase pointer
	entityBase := bytesToUint32(entityData[0x38:0x3C])
	if !isValidPtr(entityBase) {
		return false
	}

	return true
}

func getEntityName(handle windows.Handle, entityAddr uint32) string {
	namePtr1 := readU32(handle, uintptr(entityAddr)+0x0C)
	if !isValidPtr(namePtr1) {
		return ""
	}
	namePtr2 := readU32(handle, uintptr(namePtr1)+0x1C)
	if !isValidPtr(namePtr2) {
		return ""
	}
	return readString(handle, uintptr(namePtr2), 32)
}

func isValidCoord(val float32) bool {
	if val != val { // NaN check
		return false
	}
	return val > -500000 && val < 500000 && val != 0
}

// Helper functions
func readU32(h windows.Handle, addr uintptr) uint32 {
	var v uint32
	procReadProcessMemory.Call(uintptr(h), addr, uintptr(unsafe.Pointer(&v)), 4, 0)
	return v
}

func readMemoryBytes(handle windows.Handle, addr uintptr, buf []byte) error {
	var bytesRead uintptr
	ret, _, _ := procReadProcessMemory.Call(
		uintptr(handle), addr,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&bytesRead)),
	)
	if ret == 0 {
		return fmt.Errorf("read failed")
	}
	return nil
}

func readString(handle windows.Handle, addr uintptr, maxLen int) string {
	buf := make([]byte, maxLen)
	readMemoryBytes(handle, addr, buf)
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

func bytesToUint32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func bytesToFloat32(b []byte) float32 {
	bits := bytesToUint32(b)
	return *(*float32)(unsafe.Pointer(&bits))
}

func isValidPtr(ptr uint32) bool {
	return ptr >= 0x10000000 && ptr < 0xF0000000
}

func findProcess(name string) (uint32, error) {
	snap, _ := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	windows.Process32First(snap, &pe)
	for {
		if windows.UTF16ToString(pe.ExeFile[:]) == name {
			return pe.ProcessID, nil
		}
		if windows.Process32Next(snap, &pe) != nil {
			break
		}
	}
	return 0, fmt.Errorf("not found")
}

func getModuleBase(pid uint32, name string) (uintptr, error) {
	snap, _ := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPMODULE|windows.TH32CS_SNAPMODULE32, pid)
	defer windows.CloseHandle(snap)
	var me windows.ModuleEntry32
	me.Size = uint32(unsafe.Sizeof(me))
	windows.Module32First(snap, &me)
	for {
		if windows.UTF16ToString(me.Module[:]) == name {
			return uintptr(me.ModBaseAddr), nil
		}
		if windows.Module32Next(snap, &me) != nil {
			break
		}
	}
	return 0, fmt.Errorf("not found")
}