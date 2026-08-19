// INPUT: owner/Agent 身份、Manager 中已绑定的 DM/Room runtime session。
// OUTPUT: 持久 Agent 墓碑、全部匹配 session 的取消/断连，以及后续创建的 fail-closed 拒绝。
// POS: Agent 数据库身份删除提交后的 runtime 生命周期撤销边界。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ErrRuntimeAgentRevoked 表示 Agent 身份已被持久删除，runtime 不得重新创建。
var ErrRuntimeAgentRevoked = errors.New("runtime agent identity has been revoked")

type agentRuntimeIdentity struct {
	ownerUserID string
	agentID     string
}

func newAgentRuntimeIdentity(ownerUserID string, agentID string) agentRuntimeIdentity {
	return agentRuntimeIdentity{
		ownerUserID: strings.TrimSpace(ownerUserID),
		agentID:     strings.TrimSpace(agentID),
	}
}

func runtimeSessionAgentID(sessionKey string) string {
	parsed := protocol.ParseSessionKey(strings.TrimSpace(sessionKey))
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return ""
	}
	return strings.TrimSpace(parsed.AgentID)
}

func (m *Manager) runtimeAgentAdmissionErrorLocked(
	sessionKey string,
	ownerUserID string,
	agentID string,
) error {
	sessionKey = strings.TrimSpace(sessionKey)
	ownerUserID = strings.TrimSpace(ownerUserID)
	agentID = strings.TrimSpace(agentID)
	if _, deleting := m.sessionDeletionBlocks[runtimeSessionDeletionBlockKey(sessionKey)]; deleting {
		return fmt.Errorf("%w: session_key=%s", ErrRuntimeSessionDeleted, sessionKey)
	}
	if _, revoked := m.revokedSessionKeys[sessionKey]; revoked {
		return fmt.Errorf("%w: agent_id=%s", ErrRuntimeAgentRevoked, agentID)
	}
	if agentID == "" {
		return nil
	}
	if ownerUserID != "" {
		if _, revoked := m.revokedAgents[newAgentRuntimeIdentity(ownerUserID, agentID)]; revoked {
			return fmt.Errorf("%w: agent_id=%s", ErrRuntimeAgentRevoked, agentID)
		}
		return nil
	}
	// 身份墓碑存在但调用方没有 owner 时无法证明隔离边界，必须拒绝而不是
	// 允许同名 Agent 通过缺失 owner 的 options 绕过撤销。
	for identity := range m.revokedAgents {
		if identity.agentID == agentID {
			return fmt.Errorf("%w: agent_id=%s owner is required", ErrRuntimeAgentRevoked, agentID)
		}
	}
	return nil
}

// RevokeAgentSessions 原子写入 owner+Agent 墓碑，阻止并发/后续 runtime 创建，
// 并取消、断开该 Agent 当前所有 DM 与 Room session。其他 owner 或同 owner 的
// 其他 Agent 不受影响。
func (m *Manager) RevokeAgentSessions(
	ctx context.Context,
	ownerUserID string,
	agentID string,
) (int, error) {
	identity := newAgentRuntimeIdentity(ownerUserID, agentID)
	if identity.ownerUserID == "" || identity.agentID == "" {
		return 0, errors.New("owner_user_id and agent_id are required")
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, RoundIdleAbortTimeout)
		defer cancel()
	}

	targets := make([]*sessionCloseTarget, 0)
	waiting := make([]<-chan struct{}, 0)

	m.mu.Lock()
	m.revokedAgents[identity] = struct{}{}
	for sessionKey, state := range m.sessions {
		if state == nil || state.OwnerUserID != identity.ownerUserID {
			continue
		}
		sessionAgentID := strings.TrimSpace(state.AgentID)
		if sessionAgentID == "" {
			sessionAgentID = runtimeSessionAgentID(sessionKey)
		}
		if sessionAgentID != identity.agentID {
			continue
		}
		m.revokedSessionKeys[strings.TrimSpace(sessionKey)] = struct{}{}
		target, started, closeDone := m.beginSessionCloseLocked(sessionKey)
		if started {
			targets = append(targets, target)
		} else if closeDone != nil {
			waiting = append(waiting, closeDone)
		}
	}
	// 只有 owner 已无其他 Agent runtime 时才执行 owner 级进程树回收；
	// owner lifecycle fence 同时阻止新 startup 与当前撤销交错。
	reapPlan, reapFlight := m.beginOwnerReapLocked(identity.ownerUserID, nil, false)
	m.mu.Unlock()

	for _, target := range targets {
		cancelSessionCloseTarget(target)
	}
	m.startOwnerReap(reapPlan)

	errs := make([]error, 0, len(targets)+len(waiting)+1)
	for _, target := range targets {
		var closeErr error
		if target.client != nil {
			closeErr = target.client.Disconnect(ctx)
		}
		idleDrainErr := waitIdleMessageDrain(ctx, target.idleMessageDrain)
		backgroundErr := waitBackgroundTasks(ctx, target.backgroundDone)
		roundErr := waitRoundDoneForClose(ctx, target.roundDone)
		closeErr = errors.Join(closeErr, idleDrainErr, backgroundErr, roundErr)
		clientCleanupPending := errors.Is(closeErr, context.Canceled) ||
			errors.Is(closeErr, context.DeadlineExceeded)
		if clientCleanupPending || idleDrainErr != nil || backgroundErr != nil || roundErr != nil {
			m.finishSessionCloseWhenDone(target, clientCleanupPending)
		} else {
			m.finishSessionClose(target)
		}
		if closeErr != nil && !IsRuntimeTransportClosedError(closeErr) {
			errs = append(errs, fmt.Errorf(
				"close deleted Agent runtime session %s: %w",
				target.sessionKey,
				closeErr,
			))
		}
	}
	for _, closeDone := range waiting {
		if err := waitSessionClose(ctx, closeDone); err != nil {
			errs = append(errs, fmt.Errorf("wait deleted Agent runtime session close: %w", err))
		}
	}
	if reaperErr := waitOwnerReap(ctx, reapFlight); reaperErr != nil {
		errs = append(errs, fmt.Errorf("reap deleted Agent runtime processes: %w", reaperErr))
	}
	return len(targets) + len(waiting), errors.Join(errs...)
}
