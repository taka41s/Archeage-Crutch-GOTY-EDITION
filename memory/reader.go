package memory

import (
	"fmt"
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	ProcReadProcessMemory = kernel32.NewProc("ReadProcessMemory")
)

func ReadMemoryBytes(handle windows.Handle, addr uintptr, buf []byte) error {
	var bytesRead uintptr
	ret, _, _ := ProcReadProcessMemory.Call(
		uintptr(handle),
		addr,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&bytesRead)),
	)
	if ret == 0 {
		return fmt.Errorf("read failed")
	}
	return nil
}

func ReadU32(h windows.Handle, addr uintptr) uint32 {
	var v uint32
	ProcReadProcessMemory.Call(uintptr(h), addr, uintptr(unsafe.Pointer(&v)), 4, 0)
	return v
}

func ReadF32(h windows.Handle, addr uintptr) float32 {
	var v float32
	ProcReadProcessMemory.Call(uintptr(h), addr, uintptr(unsafe.Pointer(&v)), 4, 0)
	return v
}

func ReadString(handle windows.Handle, addr uintptr, maxLen int) string {
	buf := make([]byte, maxLen)
	var bytesRead uintptr
	ret, _, _ := ProcReadProcessMemory.Call(
		uintptr(handle),
		addr,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(maxLen),
		uintptr(unsafe.Pointer(&bytesRead)),
	)
	if ret == 0 {
		return ""
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

func BytesToUint32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func IsValidPtr(ptr uint32) bool {
	return ptr >= 0x10000000 && ptr < 0xF0000000
}

func IsValidCoord(val float32) bool {
	if math.IsNaN(float64(val)) || math.IsInf(float64(val), 0) {
		return false
	}
	return val > -100000 && val < 100000 && val != 0
}

func CalculateDistance(x1, y1, z1, x2, y2, z2 float32) float32 {
	dx := x2 - x1
	dy := y2 - y1
	dz := z2 - z1
	return float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
}