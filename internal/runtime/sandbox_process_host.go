// INPUT: 宿主固定的 owner/session/generation、数据库接口及任务不可写的 app 目录句柄。
// OUTPUT: Bridge 启动回调到宿主持久阶段的精确绑定；文件清理失败保留启动栅栏。
// POS: 监督启动的数据库/文件适配；尚未接入默认 transport，不授予任务任何资源权限。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SandboxProcessStore 只持久化宿主启动事实；原生证据由 Bridge 提供。
type SandboxProcessStore interface {
	PrepareProcess(context.Context, protocol.SandboxProcessIntent) error
	RegisterProcess(context.Context, protocol.SandboxProcessKey, protocol.SandboxProcessRegistration) error
	ClaimProcessRelease(context.Context, protocol.SandboxProcessKey) error
	AbortPreparedProcess(context.Context, protocol.SandboxProcessKey) error
	ReapProcess(context.Context, protocol.SandboxProcessKey, protocol.SandboxProcessEvidence) error
	Process(context.Context, protocol.SandboxProcessKey) (protocol.SandboxProcessSnapshot, bool, error)
}

type sandboxProcessBinding struct {
	Owner, Session, RuntimeKind, LeaseID string
	Generation                           uint64
}

type sandboxProcessHost struct {
	mu      sync.Mutex
	store   SandboxProcessStore
	root    *confinedfs.Root
	binding sandboxProcessBinding
	intent  *protocol.SandboxProcessIntent
}

var _ supervision.Host = (*sandboxProcessHost)(nil)

// root 必须由宿主从受保护 app 根打开，并保持到回收完成；不能传用户根或 /tmp。
// 目录权限本身不隔离同 UID 任务，生产装配仍必须落实其文件沙箱拒绝规则。
func newSandboxProcessHost(store SandboxProcessStore, root *confinedfs.Root, binding sandboxProcessBinding) (*sandboxProcessHost, error) {
	if store == nil || root == nil || !filepath.IsAbs(root.Name()) || binding.Owner == "" || binding.Session == "" || binding.Generation == 0 || binding.Generation >= math.MaxInt64 || (binding.RuntimeKind != "nxs" && binding.RuntimeKind != "claude") {
		return nil, errors.New("supervision requires a protected host root and exact runtime binding")
	}
	return &sandboxProcessHost{store: store, root: root, binding: binding}, nil
}

func (h *sandboxProcessHost) Reserve(ctx context.Context, i supervision.Intent) (supervision.Paths, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	intent := h.boundIntent(i)
	if !validSandboxLaunchID(i.ID) || i.Version != 1 || i.JobLabel != "cn.nexus.runtime."+i.ID {
		return supervision.Paths{}, errors.New("invalid supervised launch identity")
	}
	if h.intent != nil && *h.intent != intent {
		return supervision.Paths{}, errors.New("supervised host cannot be rebound")
	}
	// 在持久预留之前拒绝超长路径；不转移到任务可写的共享临时目录。
	dir := filepath.Join(h.root.Name(), "p", i.ID)
	paths := supervision.Paths{JobFile: filepath.Join(dir, "job.plist"), Socket: filepath.Join(dir, "s")}
	if len(paths.Socket) > 103 {
		return supervision.Paths{}, errors.New("protected supervision socket path exceeds macOS limit")
	}
	// Prepare 响应可能丢失；先保留精确意图供 Finish 对账，不能生成第二个启动。
	h.intent = &intent
	if err := h.store.PrepareProcess(ctx, intent); err != nil {
		return supervision.Paths{}, err
	}
	child, err := h.root.OpenOrCreateRootNoSymlink("p/"+i.ID, 0700)
	if err != nil {
		return supervision.Paths{}, err
	}
	if err := child.Close(); err != nil {
		return supervision.Paths{}, err
	}
	return paths, nil
}

func (h *sandboxProcessHost) boundIntent(i supervision.Intent) protocol.SandboxProcessIntent {
	return protocol.SandboxProcessIntent{Key: protocol.SandboxProcessKey{OwnerUserID: h.binding.Owner, SessionKey: h.binding.Session, Generation: h.binding.Generation, LaunchID: i.ID}, Version: i.Version, RuntimeKind: h.binding.RuntimeKind, LeaseID: h.binding.LeaseID, BootID: i.BootID, OwnerUID: i.OwnerUID, JobLabel: i.JobLabel, HelperSHA256: i.HelperSHA256}
}

func validSandboxLaunchID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func (h *sandboxProcessHost) snapshot(ctx context.Context, i supervision.Intent) (protocol.SandboxProcessSnapshot, bool, error) {
	if h.intent == nil || *h.intent != h.boundIntent(i) {
		return protocol.SandboxProcessSnapshot{}, false, errors.New("supervised callback does not match reserved intent")
	}
	s, found, err := h.store.Process(ctx, h.intent.Key)
	if err == nil && found && s.Intent != *h.intent {
		err = errors.New("durable supervised intent changed")
	}
	return s, found, err
}

func (h *sandboxProcessHost) Publish(ctx context.Context, i supervision.Intent, data []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, found, err := h.snapshot(ctx, i)
	if err != nil {
		return err
	}
	if !found || s.Phase != protocol.SandboxProcessPrepared {
		return errors.New("supervised job publication requires prepared intent")
	}
	if err := h.root.WriteFileAtomic("p/"+i.ID+"/job.plist", data, 0600); err != nil {
		return err
	}
	dir, err := h.root.OpenRootNoSymlink("p/" + i.ID)
	if err != nil {
		return err
	}
	defer dir.Close()
	file, err := dir.Open(".")
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func (h *sandboxProcessHost) Register(ctx context.Context, i supervision.Intent, r supervision.Registration) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, found, err := h.snapshot(ctx, i)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("supervised intent missing")
	}
	return h.store.RegisterProcess(ctx, h.intent.Key, processRegistration(r))
}

func (h *sandboxProcessHost) ClaimRelease(ctx context.Context, i supervision.Intent) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, found, err := h.snapshot(ctx, i)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("supervised intent missing")
	}
	return h.store.ClaimProcessRelease(ctx, h.intent.Key)
}

func processRegistration(r supervision.Registration) protocol.SandboxProcessRegistration {
	return protocol.SandboxProcessRegistration{Version: r.Version, BootID: r.BootID, CoalitionID: r.CoalitionID, OwnerUID: r.OwnerUID}
}

func (h *sandboxProcessHost) Finish(ctx context.Context, i supervision.Intent, proof *supervision.Evidence) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	// Reserve 未通过本地检查，不存在该 Host 创建的路径或持久意图。
	if h.intent == nil {
		return nil
	}
	s, found, err := h.snapshot(ctx, i)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	var evidence protocol.SandboxProcessEvidence
	switch s.Phase {
	case protocol.SandboxProcessPrepared, protocol.SandboxProcessAborted:
		if s.Registration != nil || s.Evidence != nil {
			return errors.New("invalid unregistered process state")
		}
	case protocol.SandboxProcessRegistered, protocol.SandboxProcessReleased, protocol.SandboxProcessReaped:
		if proof == nil || s.Registration == nil {
			return errors.New("process retirement requires original native evidence")
		}
		evidence = protocol.SandboxProcessEvidence{Registration: processRegistration(proof.Registration), Reason: proof.Reason, ObservedBootID: proof.ObservedBootID}
		if evidence.Registration != *s.Registration || !((evidence.Reason == "coalition_reaped" && evidence.ObservedBootID == s.Registration.BootID) || (evidence.Reason == "boot_changed" && evidence.ObservedBootID != "" && evidence.ObservedBootID != s.Registration.BootID)) {
			return errors.New("process retirement evidence does not match original collection")
		}
	default:
		return fmt.Errorf("unknown supervised process phase %q", s.Phase)
	}
	// Bridge 只在 job 撤销及原集合回收后调用。先删除专属路径，再写终态；删除失败
	// 必须留下 durable 启动栅栏，不能借重复 Close 把清理错误变成成功。
	parent, err := h.root.OpenRootNoSymlink("p")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		removeErr := parent.RemoveAll(i.ID)
		closeErr := parent.Close()
		if err := errors.Join(removeErr, closeErr); err != nil {
			return err
		}
	}
	if s.Phase == protocol.SandboxProcessPrepared || s.Phase == protocol.SandboxProcessAborted {
		return h.store.AbortPreparedProcess(ctx, h.intent.Key)
	}
	return h.store.ReapProcess(ctx, h.intent.Key, evidence)
}
