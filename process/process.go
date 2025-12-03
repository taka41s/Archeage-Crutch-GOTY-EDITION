package process

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

func FindProcess(name string) (uint32, error) {
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

func GetModuleBase(pid uint32, name string) (uintptr, error) {
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
