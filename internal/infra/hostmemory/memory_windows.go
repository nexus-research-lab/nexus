// INPUT: Windows GlobalMemoryStatusEx 物理内存及可提交内存。
// OUTPUT: 两种余量均足够时允许准入；物理内存总量。
// POS: 无 shell、无额外监控进程的 Windows 内存探针。
package hostmemory

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var globalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

type memoryStatusEx struct {
	Length, MemoryLoad                               uint32
	TotalPhys, AvailPhys                             uint64
	TotalPageFile, AvailPageFile                     uint64
	TotalVirtual, AvailVirtual, AvailExtendedVirtual uint64
}

func readMemoryStatus() (memoryStatusEx, error) {
	status := memoryStatusEx{}
	status.Length = uint32(unsafe.Sizeof(status))
	ok, _, err := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if ok == 0 {
		return memoryStatusEx{}, fmt.Errorf("GlobalMemoryStatusEx: %w", err)
	}
	return status, nil
}

func totalMemory() (uint64, error) {
	status, err := readMemoryStatus()
	return status.TotalPhys, err
}

func memoryAvailable(reserveMiB uint64) (bool, error) {
	status, err := readMemoryStatus()
	if err != nil {
		return false, err
	}
	// 可提交余量可能先于物理内存耗尽；不能只看任务管理器的物理占比。
	return enoughMemory(status.TotalPhys, min(status.AvailPhys, status.AvailPageFile), reserveMiB)
}
