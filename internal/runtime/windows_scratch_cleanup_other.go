//go:build !windows

// INPUT: 非Windows平台的Windows scratch请求。
// OUTPUT: 拒绝伪造Windows原生删除证明。
// POS: 显式平台边界，不回退通用路径删除。
package runtime

import (
	"context"
	"errors"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func cleanupWindowsScratch(context.Context, *confinedfs.Root, WindowsSandboxStore, protocol.WindowsSandboxKey, *sandboxResource, *windowsScratchLiveProof) error {
	return errors.New("Windows scratch cleanup requires native Windows")
}
func (m *Manager) RecoverWindowsSandboxScratch(context.Context, protocol.WindowsSandboxKey, SandboxProcessRecoveryOwnership) (protocol.WindowsScratchRecovery, error) {
	return protocol.WindowsScratchRecovery{}, errors.New("Windows scratch recovery requires native Windows")
}
