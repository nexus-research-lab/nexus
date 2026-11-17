//go:build !darwin && !linux && !windows

// INPUT: 未支持的宿主平台。
// OUTPUT: 明确的探针不可用错误。
// POS: 不把无法读取内存当作内存充足。
package hostmemory

import "errors"

func memoryAvailable(uint64) (bool, error) {
	return false, errors.New("host memory admission is unsupported on this platform")
}
