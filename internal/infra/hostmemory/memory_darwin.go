// INPUT: macOS 内核的 dispatch memory pressure 级别。
// OUTPUT: 正常压力时允许准入；读取失败或非正常级别不放行。
// POS: 无 CGO、无子进程的 macOS 内存探针。
package hostmemory

import "golang.org/x/sys/unix"

func memoryAvailable(_ uint64) (bool, error) {
	level, err := unix.SysctlUint32("kern.memorystatus_vm_pressure_level")
	// XNU 将此 sysctl 转为 dispatch 级别，NORMAL 为 0x1。
	// 不使用 free pages；缓存和压缩内存由内核压力判断统一处理。
	return level == 1, err
}
