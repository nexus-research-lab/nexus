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
	m.sandboxSupervisor = &config
	return nil
}

func (m *Manager) supervisedProcessOptions(options bridge.Options, owner, session string, floor uint64) (bridge.Options, error) {
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
	binding := sandboxProcessBinding{Owner: owner, Session: session, RuntimeKind: string(normalizedManagedRuntimeKind(options.Runtime.Kind)), Generation: floor + 1}
	if _, err := newSandboxProcessHost(store, config.Root, binding); err != nil {
		return bridge.Options{}, err
	}
	// binding 是本次 client 的不可变身份，warm reconfigure 不改写旧进程身份。
	options.ProcessSupervision = func(ctx context.Context, purpose supervision.Purpose) (supervision.Config, error) {
		if err := ctx.Err(); err != nil {
			return supervision.Config{}, err
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
