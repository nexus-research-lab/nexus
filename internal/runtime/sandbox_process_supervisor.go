// INPUT: 宿主初始化的可信 helper/目录、进程仓储与 factory 前冻结的会话代次。
// OUTPUT: 每次 runtime/probe 的独立 Host；同一代次的顺序和准入由数据库控制。
// POS: Manager 到 Bridge 的监督装配；不从请求环境解析可信路径或摘要。
package runtime

import (
	"context"
	"encoding/hex"
	"errors"
	"math"
	"path/filepath"
	"strings"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SandboxProcessSupervisor 由 App 启动装配提供，不能来自任务 settings 或环境。
// Root 必须是任务不可写的宿主目录；调用方在 Manager 完全关闭后才可关闭该句柄。
type SandboxProcessSupervisor struct {
	Root         *confinedfs.Root
	HelperPath   string
	HelperSHA256 string
}

// SetSandboxProcessSupervisor 仅允许在 Manager 使用前配置；不会自动选择临时目录。
func (m *Manager) SetSandboxProcessSupervisor(config SandboxProcessSupervisor) error {
	if config.Root == nil || !filepath.IsAbs(config.Root.Name()) || !filepath.IsAbs(config.HelperPath) || len(config.HelperSHA256) != 64 || strings.ToLower(config.HelperSHA256) != config.HelperSHA256 {
		return errors.New("invalid trusted process supervisor configuration")
	}
	if _, err := hex.DecodeString(config.HelperSHA256); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeStartups != 0 || len(m.sessions) != 0 || m.shutdownDone != nil {
		return errors.New("process supervisor must be configured before runtime startup")
	}
	if _, ok := m.sandboxReceiptStore.(SandboxProcessStore); !ok {
		return errors.New("process supervisor requires durable process store")
	}
	if _, ok := m.sandboxReceiptStore.(SandboxProcessReceiptReader); !ok {
		return errors.New("process supervisor requires durable startup reader")
	}
	if _, ok := m.sandboxReceiptStore.(sandboxRuntimeProcessReader); !ok {
		return errors.New("process supervisor requires exact runtime policy binding reader")
	}
	m.sandboxSupervisor = &config
	return nil
}

func (m *Manager) supervisedProcessOptions(options bridge.Options, owner, session string, floor uint64, lease *SandboxResourceLease) (bridge.Options, error) {
	m.mu.RLock()
	config := m.sandboxSupervisor
	store, ok := m.sandboxReceiptStore.(SandboxProcessStore)
	m.mu.RUnlock()
	if config == nil {
		return options, nil
	}
	if !ok || floor >= math.MaxInt64-1 {
		return bridge.Options{}, errors.New("supervised runtime identity unavailable or exhausted")
	}
	if options.ProcessSupervision != nil {
		return bridge.Options{}, errors.New("request cannot replace host process supervisor")
	}
	leaseID, err := supervisedLeaseIdentity(options, owner, session, lease)
	if err != nil {
		return bridge.Options{}, err
	}
	scratch, err := supervisedScratchBinding(lease)
	if err != nil {
		return bridge.Options{}, err
	}
	binding := sandboxProcessBinding{Owner: owner, Session: session, RuntimeKind: string(normalizedManagedRuntimeKind(options.Runtime.Kind)), Generation: floor + 1, LeaseID: leaseID, Scratch: scratch}
	if err := configureSupervisedLeaseCleanup(lease, config, store, binding); err != nil {
		return bridge.Options{}, err
	}
	if _, err := newSandboxProcessHost(store, config.Root, binding); err != nil {
		return bridge.Options{}, err
	}
	// binding 是本次 client 的不可变身份，warm reconfigure 不改写旧进程身份。
	options.ProcessSupervision = func(ctx context.Context, purpose supervision.Purpose) (supervision.Config, error) {
		if err := ctx.Err(); err != nil {
			return supervision.Config{}, err
		}
		currentLeaseID, err := supervisedLeaseIdentity(options, owner, session, lease)
		if err != nil {
			return supervision.Config{}, err
		}
		currentScratch, err := supervisedScratchBinding(lease)
		if err != nil {
			return supervision.Config{}, err
		}
		if currentLeaseID != binding.LeaseID || currentScratch != binding.Scratch {
			return supervision.Config{}, errors.New("supervised scratch identity changed before launch")
		}
		next := binding
		switch purpose {
		case supervision.Runtime:
			next.Purpose = protocol.SandboxProcessRuntime
		case supervision.ClaudeSandboxProbe:
			next.Purpose = protocol.SandboxProcessClaudeSandboxProbe
		case supervision.ClaudeRestrictedProbe:
			next.Purpose = protocol.SandboxProcessClaudeRestrictedProbe
		case supervision.VersionProbe:
			next.Purpose = protocol.SandboxProcessVersionProbe
		default:
			return supervision.Config{}, errors.New("unknown supervised launch purpose")
		}
		host, err := newSandboxProcessHost(store, config.Root, next)
		if err != nil {
			return supervision.Config{}, err
		}
		return supervision.Config{HelperPath: config.HelperPath, HelperSHA256: config.HelperSHA256, Host: host}, nil
	}
	return options, nil
}

// supervisedLeaseIdentity reads the supplied host handle, never a task-provided path.
func supervisedLeaseIdentity(options bridge.Options, owner, session string, lease *SandboxResourceLease) (string, error) {
	var resources *bridge.SandboxResourcePolicy
	if options.Sandbox != nil {
		resources = options.Sandbox.Resources
	}
	if resources == nil && lease == nil {
		return "", nil
	}
	if resources == nil || lease == nil || !lease.active() {
		return "", errors.New("supervised resource policy requires an active host lease")
	}
	if normalizedManagedRuntimeKind(options.Runtime.Kind) != bridge.RuntimeNXS {
		return "", errors.New("supervised scratch lease requires nxs runtime")
	}
	actual := lease.Resources()
	marker := lease.Marker()
	if actual == nil || *actual != *resources || marker == nil || marker.OwnerUserID != owner || marker.SessionKey != session || strings.TrimSpace(marker.LeaseID) == "" || marker.CleanupState == cleanupStateUnknown {
		return "", errors.New("supervised scratch lease identity or policy mismatch")
	}
	return marker.LeaseID, nil
}
