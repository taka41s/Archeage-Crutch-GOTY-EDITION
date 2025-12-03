package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procReadProcessMemory = kernel32.NewProc("ReadProcessMemory")
)

var (
	handle  windows.Handle
	x2game  uintptr
	pid     uint32
)

// Endereços que você encontrou
var knownAddresses = []struct {
	addr        uint32
	description string
}{
	{0x88C89A58, "String 1 (nome?)"},
	{0x88C8D400, "String 2 (nome?)"},
	{0x8881F934, "HP Target (1)"},
	{0x8881F930, "HP Target (2)"},
	{0x8881D054, "HP Target (3)"},
	{0x8881D050, "HP Target (4)"},
}

func main() {
	fmt.Println("╔═══════════════════════════════════════════╗")
	fmt.Println("║     Target Structure Finder               ║")
	fmt.Println("╚═══════════════════════════════════════════╝")
	fmt.Println()

	if !setup() {
		return
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println()
		fmt.Println("=== Menu ===")
		fmt.Println("1. Ler valores atuais dos endereços conhecidos")
		fmt.Println("2. Analisar estrutura ao redor do HP (0x8881F930)")
		fmt.Println("3. Analisar estrutura ao redor do HP (0x8881D050)")
		fmt.Println("4. Fazer pointer scan manual")
		fmt.Println("5. Procurar ponteiro para target em x2game.dll")
		fmt.Println("6. Comparar estrutura Target vs LocalPlayer")
		fmt.Println("7. Dump completo da região do target")
		fmt.Println("0. Sair")
		fmt.Print("\nEscolha: ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			readKnownAddresses()
		case "2":
			analyzeStructure(0x8881F930, "HP_1")
		case "3":
			analyzeStructure(0x8881D050, "HP_2")
		case "4":
			pointerScan(reader)
		case "5":
			findTargetPointer()
		case "6":
			compareStructures(reader)
		case "7":
			dumpRegion(reader)
		case "0":
			return
		}
	}
}

func setup() bool {
	var err error
	pid, err = findProcess("archeage.exe")
	if err != nil || pid == 0 {
		fmt.Println("[ERRO] ArcheAge não encontrado!")
		waitExit()
		return false
	}

	handle, err = windows.OpenProcess(0x1F0FFF, false, pid)
	if err != nil {
		fmt.Println("[ERRO] Execute como Administrador!")
		waitExit()
		return false
	}

	x2game, _ = getModuleBase(pid, "x2game.dll")
	fmt.Printf("[OK] PID: %d, x2game.dll: 0x%X\n", pid, x2game)

	return true
}

func readKnownAddresses() {
	fmt.Println()
	fmt.Println("=== Valores Atuais ===")
	fmt.Println()

	for _, known := range knownAddresses {
		// Tentar ler como diferentes tipos
		valU32 := readU32(uintptr(known.addr))
		valFloat := math.Float32frombits(valU32)
		valString := readString(uintptr(known.addr), 32)

		fmt.Printf("0x%08X [%s]\n", known.addr, known.description)
		fmt.Printf("  uint32: %d\n", valU32)
		fmt.Printf("  float:  %.4f\n", valFloat)
		fmt.Printf("  string: \"%s\"\n", sanitizeString(valString))
		fmt.Println()
	}
}

func analyzeStructure(baseAddr uint32, name string) {
	fmt.Printf("\n=== Analisando estrutura ao redor de 0x%08X (%s) ===\n\n", baseAddr, name)

	// Assumindo que o HP está em offset 0x84C como no LocalPlayer
	// Vamos calcular o possível início da entidade
	possibleEntityStart := baseAddr - 0x84C

	fmt.Printf("Se HP está em +0x84C, Entity começa em: 0x%08X\n", possibleEntityStart)
	fmt.Println()

	// Ler 0x1000 bytes começando do possível início
	buffer := make([]byte, 0x1000)
	if err := readMemoryBytes(uintptr(possibleEntityStart), buffer); err != nil {
		fmt.Println("Erro ao ler memória!")
		return
	}

	// Analisar campos conhecidos baseado no LocalPlayer
	offsets := []struct {
		offset uint32
		name   string
		typ    string
	}{
		{0x000, "VTable", "ptr"},
		{0x00C, "NamePtr1", "ptr"},
		{0x010, "Unknown", "uint32"},
		{0x038, "EntityBase", "ptr"},
		{0x830, "PosX", "float"},
		{0x834, "PosZ", "float"},
		{0x838, "PosY", "float"},
		{0x84C, "HP", "uint32"},
	}

	fmt.Println("=== Campos (assumindo mesma estrutura do LocalPlayer) ===")
	for _, off := range offsets {
		if off.offset+4 > uint32(len(buffer)) {
			continue
		}

		val := bytesToUint32(buffer[off.offset : off.offset+4])

		switch off.typ {
		case "ptr":
			fmt.Printf("+0x%03X [%-10s]: 0x%08X", off.offset, off.name, val)
			if isValidPtr(val) {
				fmt.Println(" [VALID PTR]")
			} else {
				fmt.Println(" [INVALID]")
			}
		case "float":
			f := math.Float32frombits(val)
			fmt.Printf("+0x%03X [%-10s]: %.4f", off.offset, off.name, f)
			if f > -100000 && f < 100000 && f != 0 {
				fmt.Println(" [VALID COORD]")
			} else {
				fmt.Println()
			}
		case "uint32":
			fmt.Printf("+0x%03X [%-10s]: %d (0x%X)\n", off.offset, off.name, val, val)
		}
	}

	// Verificar VTable
	vtable := bytesToUint32(buffer[0:4])
	fmt.Println()
	fmt.Printf("VTable: 0x%08X\n", vtable)
	if vtable >= 0x39000000 && vtable < 0x3B000000 {
		fmt.Println("  -> VTable parece válido para entidade!")
	}

	// Tentar ler o nome
	namePtr1 := bytesToUint32(buffer[0x0C:0x10])
	if isValidPtr(namePtr1) {
		namePtr2 := readU32(uintptr(namePtr1) + 0x1C)
		if isValidPtr(namePtr2) {
			name := readString(uintptr(namePtr2), 32)
			fmt.Printf("  -> Nome do target: \"%s\"\n", name)
		}
	}
}

func pointerScan(reader *bufio.Reader) {
	fmt.Println()
	fmt.Println("=== Pointer Scan Manual ===")
	fmt.Println()

	// Vamos procurar ponteiros para os endereços conhecidos
	targetAddrs := []uint32{
		0x8881F930, // HP area 1
		0x8881D050, // HP area 2
	}

	// Calcular possíveis entity bases
	for _, hpAddr := range targetAddrs {
		entityBase := hpAddr - 0x84C // Se HP está em +0x84C
		fmt.Printf("Procurando ponteiros para 0x%08X (entity base de HP 0x%08X)...\n", entityBase, hpAddr)

		// Scan em x2game.dll
		scanForPointerInModule(entityBase, "x2game.dll", x2game)
	}
}

func scanForPointerInModule(targetAddr uint32, moduleName string, moduleBase uintptr) {
	scanSize := uint32(0x200000) // 2MB
	buffer := make([]byte, scanSize)

	if err := readMemoryBytes(moduleBase, buffer); err != nil {
		return
	}

	found := 0
	for offset := uint32(0); offset < scanSize-4; offset += 4 {
		val := bytesToUint32(buffer[offset : offset+4])

		// Procurar ponteiro direto
		if val == targetAddr {
			fmt.Printf("  [%s+0x%X] -> 0x%08X (DIRETO)\n", moduleName, offset, val)
			found++
		}

		// Procurar ponteiro que aponta para região próxima (pode ser struct que contém entity)
		if val >= targetAddr-0x1000 && val <= targetAddr+0x1000 && isValidPtr(val) {
			diff := int32(val) - int32(targetAddr)
			if diff != 0 && found < 20 {
				// Verificar se é um ponteiro válido que faz sentido
				testVal := readU32(uintptr(val))
				if testVal != 0 && isValidPtr(testVal) {
					fmt.Printf("  [%s+0x%X] -> 0x%08X (offset %+d do target)\n", moduleName, offset, val, diff)
					found++
				}
			}
		}
	}

	if found == 0 {
		fmt.Printf("  Nenhum ponteiro direto encontrado em %s\n", moduleName)
	}
}

func findTargetPointer() {
	fmt.Println()
	fmt.Println("=== Procurando Target Pointer em x2game.dll ===")
	fmt.Println()

	// Offsets comuns para ponteiros de target em jogos
	commonOffsets := []uint32{
		0xE9DC54 + 0x08, // Próximo ao LocalPlayer
		0xE9DC54 + 0x10,
		0xE9DC54 + 0x18,
		0xE9DC54 + 0x20,
		0xE9DC54 - 0x08,
		0xE9DC54 - 0x10,
	}

	fmt.Println("Testando offsets próximos ao LocalPlayer pointer...")
	for _, offset := range commonOffsets {
		ptr1 := readU32(x2game + uintptr(offset))
		if isValidPtr(ptr1) {
			ptr2 := readU32(uintptr(ptr1))
			fmt.Printf("  x2game+0x%X -> 0x%08X", offset, ptr1)
			if isValidPtr(ptr2) {
				// Verificar se parece uma entidade (tem VTable válido)
				vtable := readU32(uintptr(ptr2))
				if vtable >= 0x39000000 && vtable < 0x3B000000 {
					hp := readU32(uintptr(ptr2) + 0x84C)
					fmt.Printf(" -> Entity? VTable=0x%X, HP=%d", vtable, hp)
				}
			}
			fmt.Println()
		}
	}

	// Scan mais amplo
	fmt.Println()
	fmt.Println("Scan amplo por estruturas de target...")

	// Ler região ao redor do LocalPlayer pointer
	scanStart := x2game + 0xE90000
	scanSize := uint32(0x20000)
	buffer := make([]byte, scanSize)

	if err := readMemoryBytes(scanStart, buffer); err != nil {
		fmt.Println("Erro ao ler memória!")
		return
	}

	foundTargets := 0
	for offset := uint32(0); offset < scanSize-4; offset += 4 {
		ptr := bytesToUint32(buffer[offset : offset+4])
		if !isValidPtr(ptr) {
			continue
		}

		// Ler o que o ponteiro aponta
		data := make([]byte, 0x10)
		if err := readMemoryBytes(uintptr(ptr), data); err != nil {
			continue
		}

		innerPtr := bytesToUint32(data[0:4])
		if !isValidPtr(innerPtr) {
			continue
		}

		// Verificar se parece entidade
		entityData := make([]byte, 0x900)
		if err := readMemoryBytes(uintptr(innerPtr), entityData); err != nil {
			continue
		}

		vtable := bytesToUint32(entityData[0:4])
		if vtable < 0x39000000 || vtable >= 0x3B000000 {
			continue
		}

		hp := bytesToUint32(entityData[0x84C:0x850])
		if hp < 100 || hp > 10000000 {
			continue
		}

		posX := math.Float32frombits(bytesToUint32(entityData[0x830:0x834]))
		if posX < -100000 || posX > 100000 || posX == 0 {
			continue
		}

		// Possível target/entity pointer encontrado!
		realOffset := uint32(scanStart-x2game) + offset
		fmt.Printf("  [x2game+0x%X] -> [+0x%X] -> Entity (HP=%d, X=%.1f)\n",
			realOffset, 0, hp, posX)

		// Tentar ler nome
		namePtr1 := bytesToUint32(entityData[0x0C:0x10])
		if isValidPtr(namePtr1) {
			namePtr2 := readU32(uintptr(namePtr1) + 0x1C)
			if isValidPtr(namePtr2) {
				name := readString(uintptr(namePtr2), 32)
				if len(name) > 0 {
					fmt.Printf("      -> Nome: \"%s\"\n", name)
				}
			}
		}

		foundTargets++
		if foundTargets >= 10 {
			fmt.Println("  ... (mostrando apenas 10)")
			break
		}
	}
}

func compareStructures(reader *bufio.Reader) {
	fmt.Println()
	fmt.Println("=== Comparar Target vs LocalPlayer ===")
	fmt.Println()

	// Ler LocalPlayer
	ptr1 := readU32(x2game + 0xE9DC54)
	if ptr1 == 0 {
		fmt.Println("LocalPlayer não encontrado!")
		return
	}

	localEntity := readU32(uintptr(ptr1) + 0x10)
	if localEntity == 0 {
		fmt.Println("LocalPlayer entity não encontrada!")
		return
	}

	fmt.Printf("LocalPlayer Entity: 0x%08X\n", localEntity)

	// Ler dados do LocalPlayer
	localData := make([]byte, 0x900)
	readMemoryBytes(uintptr(localEntity), localData)

	// Possíveis targets baseado nos endereços encontrados
	targetCandidates := []uint32{
		0x8881F930 - 0x84C, // Entity base calculada
		0x8881D050 - 0x84C,
	}

	for _, targetEntity := range targetCandidates {
		fmt.Printf("\n--- Target Candidate: 0x%08X ---\n", targetEntity)

		targetData := make([]byte, 0x900)
		if err := readMemoryBytes(uintptr(targetEntity), targetData); err != nil {
			fmt.Println("  Não foi possível ler!")
			continue
		}

		// Comparar campos
		fields := []struct {
			offset uint32
			name   string
			typ    string
		}{
			{0x000, "VTable", "hex"},
			{0x00C, "NamePtr1", "ptr"},
			{0x038, "EntityBase", "ptr"},
			{0x830, "PosX", "float"},
			{0x834, "PosZ", "float"},
			{0x838, "PosY", "float"},
			{0x84C, "HP", "uint32"},
		}

		fmt.Printf("\n%-12s | %-20s | %-20s\n", "Campo", "LocalPlayer", "Target")
		fmt.Println(strings.Repeat("-", 60))

		for _, f := range fields {
			localVal := bytesToUint32(localData[f.offset : f.offset+4])
			targetVal := bytesToUint32(targetData[f.offset : f.offset+4])

			var localStr, targetStr string

			switch f.typ {
			case "float":
				localStr = fmt.Sprintf("%.2f", math.Float32frombits(localVal))
				targetStr = fmt.Sprintf("%.2f", math.Float32frombits(targetVal))
			case "ptr", "hex":
				localStr = fmt.Sprintf("0x%08X", localVal)
				targetStr = fmt.Sprintf("0x%08X", targetVal)
			default:
				localStr = fmt.Sprintf("%d", localVal)
				targetStr = fmt.Sprintf("%d", targetVal)
			}

			fmt.Printf("%-12s | %-20s | %-20s\n", f.name, localStr, targetStr)
		}

		// Ler nomes
		fmt.Println()

		localName := getEntityName(localEntity)
		targetName := getEntityName(targetEntity)

		fmt.Printf("LocalPlayer Nome: \"%s\"\n", localName)
		fmt.Printf("Target Nome:      \"%s\"\n", targetName)
	}
}

func dumpRegion(reader *bufio.Reader) {
	fmt.Println()
	fmt.Print("Endereço base (hex, ex: 8881D050): ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var addr uint64
	fmt.Sscanf(input, "%x", &addr)

	// Calcular entity base
	entityBase := uint32(addr) - 0x84C

	fmt.Printf("\nDump de 0x%08X (entity base calculada):\n\n", entityBase)

	buffer := make([]byte, 0x100)
	readMemoryBytes(uintptr(entityBase), buffer)

	for i := 0; i < len(buffer); i += 16 {
		fmt.Printf("%08X: ", entityBase+uint32(i))

		// Hex
		for j := 0; j < 16 && i+j < len(buffer); j++ {
			fmt.Printf("%02X ", buffer[i+j])
		}

		fmt.Print(" | ")

		// ASCII
		for j := 0; j < 16 && i+j < len(buffer); j++ {
			c := buffer[i+j]
			if c >= 32 && c < 127 {
				fmt.Printf("%c", c)
			} else {
				fmt.Print(".")
			}
		}

		fmt.Println()
	}
}

// === HELPER FUNCTIONS ===

func getEntityName(entityAddr uint32) string {
	namePtr1 := readU32(uintptr(entityAddr) + 0x0C)
	if !isValidPtr(namePtr1) {
		return ""
	}
	namePtr2 := readU32(uintptr(namePtr1) + 0x1C)
	if !isValidPtr(namePtr2) {
		return ""
	}
	return readString(uintptr(namePtr2), 32)
}

func readU32(addr uintptr) uint32 {
	var v uint32
	procReadProcessMemory.Call(uintptr(handle), addr, uintptr(unsafe.Pointer(&v)), 4, 0)
	return v
}

func readMemoryBytes(addr uintptr, buf []byte) error {
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

func readString(addr uintptr, maxLen int) string {
	buf := make([]byte, maxLen)
	readMemoryBytes(addr, buf)
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

func sanitizeString(s string) string {
	var result strings.Builder
	for _, c := range s {
		if c >= 32 && c < 127 {
			result.WriteRune(c)
		} else if c == 0 {
			break
		} else {
			result.WriteRune('.')
		}
	}
	return result.String()
}

func bytesToUint32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func isValidPtr(ptr uint32) bool {
	return ptr >= 0x10000000 && ptr < 0xF0000000
}

func waitExit() {
	fmt.Println("Pressione Enter para sair...")
	bufio.NewReader(os.Stdin).ReadString('\n')
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