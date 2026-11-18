// INPUT: 宿主内存快照与配置的最小保留 MiB。
// OUTPUT: 允许新增工作与否的轻量探针，以及按物理内存推算的默认会话额度。
// POS: 平台内存读数到准入判断的叶子基础设施。
package hostmemory

import (
	"errors"
	"sync/atomic"
)

const (
	// 大内存机器按比例保留会过严：64 GiB 时 15% 已近 10 GiB。
	maxProportionalReserve = 2 << 30
	minDefaultSessions     = 4
	maxDefaultSessions     = 12
)

// NewAdmissionProbe 为非正配置返回 nil；否则读取实时宿主状态。
// Windows/Linux 至少保留指定 MiB，以及总物理内存 15%（上限 2 GiB）中的较大值；
// macOS 只在内核报告 critical 压力时拒绝。
// 探针读取失败时放行并经 onError 报告：这是软准入，读不到读数不能让产品停摆。
// onError 只在从成功转为失败时调用一次，避免每次准入都写日志。
func NewAdmissionProbe(reserveMiB int, onError func(error)) func() bool {
	if reserveMiB <= 0 {
		return nil
	}
	var failing atomic.Bool
	return func() bool {
		available, err := memoryAvailable(uint64(reserveMiB))
		if err != nil {
			if !failing.Swap(true) && onError != nil {
				onError(err)
			}
			return true
		}
		failing.Store(false)
		return available
	}
}

// DefaultSessionLimit 每 2 GiB 物理内存一个会话，限制在 [4, 12]；读不到总量时取 4。
func DefaultSessionLimit() int {
	total, err := totalMemory()
	if err != nil || total == 0 {
		return minDefaultSessions
	}
	return min(max(int(total>>31), minDefaultSessions), maxDefaultSessions)
}

func enoughMemory(total, available, reserveMiB uint64) (bool, error) {
	if total == 0 || available > total {
		return false, errors.New("invalid host memory snapshot")
	}
	return available>>20 >= reserveMiB && available >= min(total/100*15, maxProportionalReserve), nil
}
