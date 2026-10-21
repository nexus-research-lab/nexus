// INPUT: 宿主启动恢复各页中的单项失败与持锁所有权。
// OUTPUT: owner-scoped 的失败清单；失败项仍由持久记录按会话阻断，不影响其他会话。
// POS: 启动恢复与设置页之间的只读投影；放弃动作见各平台实现。
package runtime

import (
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SandboxStartupRecoveryStage names the startup recovery phase that failed.
type SandboxStartupRecoveryStage string

const (
	// SandboxStartupRecoveryProcess means the original process record is still
	// prepared/registered/released; its process may be alive, so it cannot be
	// discarded and keeps blocking only its own session.
	SandboxStartupRecoveryProcess SandboxStartupRecoveryStage = "process"
	// SandboxStartupRecoveryLifecycle means the process is terminal but its
	// scratch or bound policy could not be closed automatically.
	SandboxStartupRecoveryLifecycle SandboxStartupRecoveryStage = "lifecycle"
)

const maxSandboxStartupRecoveryReasonBytes = 512

// SandboxStartupRecoveryFailure is one record that startup recovery left
// pending. The durable record still fences its exact owner/session.
type SandboxStartupRecoveryFailure struct {
	Key    protocol.SandboxProcessKey
	Stage  SandboxStartupRecoveryStage
	Reason string
}

// Discardable reports whether the owner may close this record explicitly.
func (f SandboxStartupRecoveryFailure) Discardable() bool {
	return f.Stage == SandboxStartupRecoveryLifecycle
}

type sandboxStartupRecoveryState struct {
	ownership SandboxProcessRecoveryOwnership
	failures  []SandboxStartupRecoveryFailure
}

// RecordSandboxStartupRecovery keeps the result of host startup recovery.
// Called once by App assembly after all recovery pages, before admission.
func (m *Manager) RecordSandboxStartupRecovery(ownership SandboxProcessRecoveryOwnership, failures []SandboxStartupRecoveryFailure) {
	recorded := make([]SandboxStartupRecoveryFailure, 0, len(failures))
	for _, failure := range failures {
		failure.Reason = boundedRecoveryReason(failure.Reason)
		recorded = append(recorded, failure)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sandboxRecovery = sandboxStartupRecoveryState{ownership: ownership, failures: recorded}
}

// SandboxStartupRecoveryFailures lists the given owner's unresolved records.
func (m *Manager) SandboxStartupRecoveryFailures(owner string) []SandboxStartupRecoveryFailure {
	owner = strings.TrimSpace(owner)
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := []SandboxStartupRecoveryFailure{}
	for _, failure := range m.sandboxRecovery.failures {
		if owner != "" && failure.Key.OwnerUserID == owner {
			result = append(result, failure)
		}
	}
	return result
}

func (m *Manager) sandboxStartupRecoveryFailure(owner, launchID string) (SandboxStartupRecoveryFailure, SandboxProcessRecoveryOwnership, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, failure := range m.sandboxRecovery.failures {
		if failure.Key.OwnerUserID == owner && failure.Key.LaunchID == launchID {
			return failure, m.sandboxRecovery.ownership, true
		}
	}
	return SandboxStartupRecoveryFailure{}, nil, false
}

func (m *Manager) forgetSandboxStartupRecoveryFailure(key protocol.SandboxProcessKey) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sandboxRecovery.failures = slices.DeleteFunc(m.sandboxRecovery.failures, func(failure SandboxStartupRecoveryFailure) bool {
		return failure.Key == key
	})
}

func boundedRecoveryReason(reason string) string {
	reason = strings.Join(strings.Fields(reason), " ")
	if len(reason) <= maxSandboxStartupRecoveryReasonBytes {
		return reason
	}
	cut := maxSandboxStartupRecoveryReasonBytes
	for cut > 0 && reason[cut]&0xC0 == 0x80 {
		cut--
	}
	return reason[:cut]
}
