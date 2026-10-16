//go:build !windows

// INPUT: 非Windows平台的Windows监督请求。
// OUTPUT: 拒绝伪造原生目录身份。
// POS: 平台边界，不制造跨平台冷恢复证明。
package runtime

import "errors"

func captureWindowsScratch(windowsSandboxBinding) (string, string, error) {
	return "", "", errors.New("Windows scratch identity requires native Windows")
}
