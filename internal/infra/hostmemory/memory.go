// INPUT: 宿主内存快照与配置的最小保留 MiB。
// OUTPUT: 允许新增工作与否的轻量探针。
// POS: 平台内存读数到准入判断的叶子基础设施。
package hostmemory

import "errors"

// NewAdmissionProbe 为非正配置返回 nil；否则读取实时宿主状态。
// Windows/Linux 至少保留总物理内存的 15% 与指定 MiB 中的较大值；
// macOS 使用内核压力级别，避免把可回收缓存误判成内存耗尽。
func NewAdmissionProbe(reserveMiB int) func() (bool, error) {
	if reserveMiB <= 0 {
		return nil
	}
	return func() (bool, error) { return memoryAvailable(uint64(reserveMiB)) }
}

func enoughMemory(total, available, reserveMiB uint64) (bool, error) {
	if total == 0 || available > total {
		return false, errors.New("invalid host memory snapshot")
	}
	return available>>20 >= reserveMiB && available >= total/100*15, nil
}
