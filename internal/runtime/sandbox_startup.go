// INPUT: exact owner/session、持久策略回执与固定 scratch 父目录句柄。
// OUTPUT: 新 runtime 的代次下界与跨重启保留的清理失败准入栅栏。
// POS: 进程创建前的宿主恢复边界；历史回执不授予权限或证明后代终态。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ErrSandboxCleanupPending 表示旧执行尚未证明收口，不能用新目录或后端绕过。
var ErrSandboxCleanupPending = errors.New("previous sandbox runtime cleanup is unresolved")

// sandboxStartupGeneration 在创建进程之前读取最新持久代次；只允许已收口的
// generation 后继。相同宿主正在复用的 client 不进入这条重建路径。
func (m *Manager) sandboxStartupGeneration(ctx context.Context, owner, session string) (uint64, error) {
	m.mu.RLock()
	store := m.sandboxReceiptStore
	m.mu.RUnlock()
	if store == nil || owner == "" {
		return 0, nil
	}
	reader, ok := store.(SandboxPolicyReceiptReader)
	if !ok {
		return 0, errors.New("sandbox receipt store cannot verify previous runtime cleanup")
	}
	processFloor, err := sandboxProcessStartupGeneration(ctx, store, owner, session)
	if err != nil {
		return 0, err
	}
	windowsFloor, err := windowsSandboxStartupGeneration(ctx, store, owner, session)
	if err != nil {
		return 0, err
	}
	processFloor = max(processFloor, windowsFloor)
	snapshot, found, err := reader.Latest(ctx, owner, session)
	if err != nil {
		return 0, fmt.Errorf("read previous sandbox runtime receipt: %w", err)
	}
	if !found {
		return processFloor, nil
	}
	if snapshot.OwnerUserID != owner || snapshot.SessionKey != session {
		return 0, errors.New("previous sandbox runtime receipt scope mismatch")
	}
	if _, err := sandboxPolicyReceiptFromSnapshot(snapshot); err != nil {
		return 0, err
	}
	if snapshot.Generation >= math.MaxInt64 {
		return 0, errors.New("sandbox runtime generation is exhausted")
	}
	switch snapshot.Phase {
	case protocol.SandboxPolicyReceiptRetired, protocol.SandboxPolicyReceiptReconciled:
		return max(snapshot.Generation, processFloor), nil
	default:
		return 0, ErrSandboxCleanupPending
	}
}

// checkSandboxScratchAdmission 只读取固定父目录下的 marker；任何旧路径上的
// exact scope 清理失败都阻断新 Acquire，包括历史生成的 stale 后缀目录。
func checkSandboxScratchAdmission(ctx context.Context, base *confinedfs.Root, root, owner, session, name string) error {
	entries, err := fs.ReadDir(base.FS(), ".")
	if err != nil {
		return fmt.Errorf("inspect previous sandbox scratch: %w", err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !strings.HasPrefix(entry.Name(), ".scratch-") {
			continue
		}
		samePathScope := entry.Name() == name || strings.HasPrefix(entry.Name(), name+"-stale-")
		leaf, err := base.OpenRootNoSymlink(entry.Name())
		if err != nil {
			if samePathScope {
				return fmt.Errorf("verify previous sandbox scratch: %w", errors.Join(ErrSandboxCleanupPending, err))
			}
			continue
		}
		marker, readErr := readSandboxLeaseMarkerInRoot(leaf)
		closeErr := leaf.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			if samePathScope {
				return fmt.Errorf("verify previous sandbox marker: %w", errors.Join(ErrSandboxCleanupPending, err))
			}
			continue
		}
		if marker.OwnerUserID != owner || marker.SessionKey != session {
			if samePathScope {
				return ErrSandboxCleanupPending
			}
			continue
		}
		marker, err = validateSandboxLeaseMarker(marker, root, owner)
		if err != nil {
			return fmt.Errorf("verify previous sandbox marker: %w", errors.Join(ErrSandboxCleanupPending, err))
		}
		if marker.CleanupState == cleanupStateUnknown {
			return ErrSandboxCleanupPending
		}
	}
	return nil
}
