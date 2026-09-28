// INPUT: 宿主独占实例下的 exact process key、可信数据库及原生恢复器。
// OUTPUT: 仅该进程记录的撤销/回收终态，其他策略与 scratch 栅栏保持原样。
// POS: Manager 会话启动锁内的显式恢复入口；不自动执行任务或猜测副作用。
package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SandboxProcessRecoveryOwnership 必须持有真实宿主实例锁并在整个回调期间保持所有权。
// callback 获得经 inode 核验的 app 根；只读快照或 PID 不能实现此证明。
type SandboxProcessRecoveryOwnership interface {
	WithOwnership(func(appRoot string) error) error
}

type sandboxProcessRecoveryFunc func(context.Context, supervision.Recovery, supervision.RecoveryHost) error

// RecoverSandboxProcess 仅用于宿主已取得跨进程独占实例锁、确认旧宿主退出后的恢复。
// Manager 再以会话 gate 排除本实例活动 client。调用方仍须独立核对 policy 与 lease；
// 进程回收成功不授权重放工具，不会清除其他代次或声明整个会话恢复完成。
func (m *Manager) RecoverSandboxProcess(ctx context.Context, key protocol.SandboxProcessKey, ownership SandboxProcessRecoveryOwnership) (protocol.SandboxProcessSnapshot, error) {
	return m.recoverSandboxProcess(ctx, key, ownership, supervision.Recover)
}

func (m *Manager) recoverSandboxProcess(ctx context.Context, key protocol.SandboxProcessKey, ownership SandboxProcessRecoveryOwnership, recoverNative sandboxProcessRecoveryFunc) (protocol.SandboxProcessSnapshot, error) {
	if ownership == nil {
		return protocol.SandboxProcessSnapshot{}, errors.New("process recovery requires exclusive host ownership")
	}
	var result protocol.SandboxProcessSnapshot
	err := ownership.WithOwnership(func(appRoot string) error {
		var err error
		result, err = m.recoverSandboxProcessOwned(ctx, key, appRoot, recoverNative)
		return err
	})
	return result, err
}

func (m *Manager) recoverSandboxProcessOwned(ctx context.Context, key protocol.SandboxProcessKey, appRoot string, recoverNative sandboxProcessRecoveryFunc) (protocol.SandboxProcessSnapshot, error) {
	empty := protocol.SandboxProcessSnapshot{}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	startup, err := m.BeginClientStartup(ctx, key.SessionKey, key.OwnerUserID)
	if err != nil {
		return empty, err
	}
	defer startup.Close()
	m.mu.RLock()
	config := m.sandboxSupervisor
	store, ok := m.sandboxReceiptStore.(SandboxProcessStore)
	state := m.sessions[key.SessionKey]
	active := state != nil && (state.Client != nil || state.Closing)
	m.mu.RUnlock()
	if active {
		return empty, errors.New("cannot recover a process owned by an active runtime")
	}
	if config == nil || !ok {
		return empty, errors.New("process recovery requires configured trusted supervisor")
	}
	if err := verifyRecoveryRoot(appRoot, config.Root); err != nil {
		return empty, err
	}
	snapshot, found, err := store.Process(ctx, key)
	if err != nil {
		return empty, err
	}
	if !found || snapshot.Intent.Key != key {
		return empty, errors.New("exact process recovery record missing")
	}
	switch snapshot.Phase {
	case protocol.SandboxProcessAborted, protocol.SandboxProcessReaped:
		return snapshot, nil
	case protocol.SandboxProcessPrepared:
		if snapshot.Registration != nil || snapshot.Evidence != nil {
			return empty, errors.New("invalid unregistered recovery record")
		}
	case protocol.SandboxProcessRegistered, protocol.SandboxProcessReleased:
		if snapshot.Registration == nil || snapshot.Evidence != nil {
			return empty, errors.New("invalid registered recovery record")
		}
	default:
		return empty, errors.New("unknown process recovery phase")
	}
	i := snapshot.Intent
	host, err := newSandboxProcessHost(store, config.Root, sandboxProcessBinding{Owner: key.OwnerUserID, Session: key.SessionKey, Generation: key.Generation, RuntimeKind: i.RuntimeKind, LeaseID: i.LeaseID, Purpose: i.Purpose, Scratch: i.Scratch})
	if err != nil {
		return empty, err
	}
	// 绑定已存在的 exact 意图；恢复路径绝不调用 Reserve 或 ClaimRelease。
	host.intent = &i
	nativeIntent := supervision.Intent{Version: i.Version, ID: key.LaunchID, BootID: i.BootID, OwnerUID: i.OwnerUID, JobLabel: i.JobLabel, HelperSHA256: i.HelperSHA256}
	record := supervision.Recovery{Intent: nativeIntent}
	if r := snapshot.Registration; r != nil {
		record.Registration = &supervision.Registration{Version: r.Version, BootID: r.BootID, OwnerUID: r.OwnerUID, CoalitionID: r.CoalitionID}
	}
	if err := recoverNative(ctx, record, host); err != nil {
		return empty, err
	}
	result, found, err := store.Process(ctx, key)
	if err != nil {
		return empty, err
	}
	if !found || result.Intent != i || (result.Phase != protocol.SandboxProcessAborted && result.Phase != protocol.SandboxProcessReaped) {
		return empty, errors.New("process recovery did not persist exact terminal record")
	}
	return result, nil
}

// verifyRecoveryRoot 防止把另一状态根的实例锁借给当前进程记录。
func verifyRecoveryRoot(appRoot string, root *confinedfs.Root) error {
	if root == nil || !filepath.IsAbs(appRoot) {
		return errors.New("invalid recovery ownership root")
	}
	relative, err := filepath.Rel(appRoot, root.Name())
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return errors.New("process root is outside the owned app root")
	}
	app, err := confinedfs.Open(appRoot)
	if err != nil {
		return err
	}
	defer app.Close()
	current, err := app.OpenRootNoSymlink(relative)
	if err != nil {
		return err
	}
	defer current.Close()
	expectedInfo, err := root.Stat(".")
	if err != nil {
		return err
	}
	actualInfo, err := current.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(expectedInfo, actualInfo) {
		return errors.New("process root identity changed")
	}
	return nil
}
