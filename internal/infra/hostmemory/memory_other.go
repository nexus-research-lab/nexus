//go:build !darwin && !linux && !windows

// INPUT: 未支持的宿主平台。
// OUTPUT: 明确的探针不可用错误；调用方据此放行并取默认额度。
// POS: 不伪造读数；放行策略由 NewAdmissionProbe 统一决定。
package hostmemory

import "errors"

func memoryAvailable(uint64) (bool, error) {
	return false, errors.New("host memory admission is unsupported on this platform")
}

func totalMemory() (uint64, error) {
	return 0, errors.New("host memory total is unsupported on this platform")
}
