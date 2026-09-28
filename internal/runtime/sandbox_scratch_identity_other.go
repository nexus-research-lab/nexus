//go:build !darwin

// INPUT: 非 macOS 的现有 lease。
// OUTPUT: 空的 macOS 恢复身份；不构造未验证的平台证据。
// POS: 保留既有非 macOS 监督接口，本实现不启用自动资源恢复。
package runtime

import "github.com/nexus-research-lab/nexus/internal/protocol"

func supervisedScratchBinding(*SandboxResourceLease) (protocol.SandboxProcessScratch, error) {
	return protocol.SandboxProcessScratch{}, nil
}
