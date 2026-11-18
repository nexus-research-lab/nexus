// INPUT: macOS 内核的 dispatch memory pressure 级别与 hw.memsize。
// OUTPUT: 非 critical 压力时允许准入；物理内存总量。
// POS: 无 CGO、无子进程的 macOS 内存探针。
package hostmemory

import "golang.org/x/sys/unix"

// XNU 将此 sysctl 转为 dispatch 级别：NORMAL=0x1、WARN=0x2、CRITICAL=0x4。
// WARN 在 8–16 GiB 的 Mac 上很常见，且缓存与压缩内存仍可回收，因此只拒绝 CRITICAL。
const pressureCritical = 4

func memoryAvailable(_ uint64) (bool, error) {
	level, err := unix.SysctlUint32("kern.memorystatus_vm_pressure_level")
	if err != nil {
		return false, err
	}
	return level < pressureCritical, nil
}

func totalMemory() (uint64, error) {
	return unix.SysctlUint64("hw.memsize")
}
