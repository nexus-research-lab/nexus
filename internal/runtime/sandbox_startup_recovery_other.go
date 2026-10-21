//go:build !darwin

// INPUT: owner 与 launch ID。
// OUTPUT: 与 macOS 相同的查找/可放弃判定；非 macOS 没有监督启动记录可收口。
// POS: 平台边界；不在其他平台推测恢复事实。
package runtime

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrSandboxRecoveryNotFound       = errors.New("sandbox startup recovery record not found")
	ErrSandboxRecoveryNotDiscardable = errors.New("sandbox startup recovery record cannot be discarded")
)

// DiscardSandboxStartupRecovery is only meaningful for the supervised macOS App.
func (m *Manager) DiscardSandboxStartupRecovery(_ context.Context, owner, launchID, _ string) error {
	failure, _, found := m.sandboxStartupRecoveryFailure(strings.TrimSpace(owner), strings.TrimSpace(launchID))
	if !found {
		return ErrSandboxRecoveryNotFound
	}
	if !failure.Discardable() {
		return ErrSandboxRecoveryNotDiscardable
	}
	return errors.New("sandbox startup recovery discard is only supported on the macOS App")
}
