package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const automationSessionCloseTimeout = 3 * time.Second

func (s *Service) resolveTaskOwnerUserID(ctx context.Context, agentID string) (string, error) {
	if s.agents != nil && strings.TrimSpace(agentID) != "" {
		agentValue, err := s.requireAgent(ctx, agentID)
		if err != nil {
			return "", err
		}
		if agentValue != nil {
			if ownerUserID := strings.TrimSpace(agentValue.OwnerUserID); ownerUserID != "" {
				return ownerUserID, nil
			}
		}
	}
	return authctx.OwnerUserID(ctx), nil
}

func (s *Service) validateTaskExpiration(expiresAt *time.Time) error {
	if expiresAt == nil {
		return nil
	}
	if !expiresAt.UTC().After(s.nowFn().UTC()) {
		return fmt.Errorf("expires_at 必须晚于当前时间")
	}
	return nil
}

func (s *Service) validateTaskCapacity(ctx context.Context, ownerUserID string, enabling bool) error {
	if !enabling {
		return nil
	}
	limit := s.config.AutomationMaxEnabledTasksPerUser
	if limit <= 0 {
		limit = 100
	}
	count, err := s.repository.CountEnabledScheduledTasks(ctx, strings.TrimSpace(ownerUserID), "")
	if err != nil {
		return fmt.Errorf("统计已启用自动化任务: %w", err)
	}
	if count >= limit {
		return fmt.Errorf("每个用户启用的定时任务不能超过 %d 个", limit)
	}
	return nil
}

func (s *Service) cleanupIsolatedAutomationSessions(ctx context.Context, job automationdomain.ScheduledTask) error {
	if strings.TrimSpace(job.SessionTarget.Kind) != automationdomain.SessionTargetIsolated {
		return nil
	}
	workspacePath, err := s.resolveAutomationWorkspacePath(ctx, job.AgentID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(workspacePath) == "" {
		return nil
	}
	ownerUserID := strings.TrimSpace(job.OwnerUserID)
	if ownerUserID == "" {
		return errors.New("清理自动化会话缺少 owner_user_id")
	}
	prefix := fmt.Sprintf("agent:%s:automation:dm:scheduled-task:%s:", strings.TrimSpace(job.AgentID), strings.TrimSpace(job.JobID))
	files := workspacestore.NewSessionFileStore(s.config.WorkspacePath).ForOwner(ownerUserID)
	history := workspacestore.NewAgentHistoryStore(s.config.WorkspacePath).ForOwner(ownerUserID)
	sessions, err := files.ListSessions(workspacePath)
	if err != nil {
		return err
	}
	for _, item := range sessions {
		sessionKey := strings.TrimSpace(item.SessionKey)
		if !strings.HasPrefix(sessionKey, prefix) {
			continue
		}
		parsed := protocol.ParseSessionKey(sessionKey)
		if parsed.Kind != protocol.SessionKeyKindAgent || !parsed.IsStructured || parsed.Channel != "automation" {
			continue
		}
		if s.sessionCloser != nil {
			closeCtx, cancel := context.WithTimeout(context.Background(), automationSessionCloseTimeout)
			closeErr := s.sessionCloser.CloseSession(closeCtx, sessionKey)
			cancel()
			if closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
				return closeErr
			}
		}
		for _, transcriptSessionID := range protocol.SessionTranscriptIDs(item) {
			if _, deleteErr := history.DeleteTranscriptSession(workspacePath, transcriptSessionID); deleteErr != nil {
				return deleteErr
			}
		}
		if _, deleteErr := files.DeleteSession(workspacePath, sessionKey); deleteErr != nil {
			return deleteErr
		}
	}
	return nil
}

// DeleteTasksForSessions 删除目标或来源精确绑定到 Session 的定时任务。
func (s *Service) DeleteTasksForSessions(
	ctx context.Context,
	ownerUserID string,
	sessionKeys []string,
) error {
	keySet := make(map[string]struct{}, len(sessionKeys))
	for _, sessionKey := range sessionKeys {
		sessionKey = strings.TrimSpace(sessionKey)
		if sessionKey != "" {
			keySet[sessionKey] = struct{}{}
		}
	}
	if len(keySet) == 0 {
		return nil
	}
	items, err := s.repository.ListScheduledTasks(ctx, strings.TrimSpace(ownerUserID), "")
	if err != nil {
		return err
	}
	for _, item := range items {
		if !scheduledTaskReferencesSession(item, keySet) {
			continue
		}
		if _, err = s.DeleteTask(contextForJobOwner(ctx, item), item.JobID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteTasksForAgent 删除属于指定 Agent 的所有定时任务。
func (s *Service) DeleteTasksForAgent(
	ctx context.Context,
	ownerUserID string,
	agentID string,
) error {
	items, err := s.repository.ListScheduledTasks(
		ctx,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(agentID),
	)
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, err = s.DeleteTask(contextForJobOwner(ctx, item), item.JobID); err != nil {
			return err
		}
	}
	return nil
}

func scheduledTaskReferencesSession(
	item automationdomain.ScheduledTask,
	sessionKeys map[string]struct{},
) bool {
	for _, sessionKey := range []string{
		item.SessionTarget.BoundSessionKey,
		item.SessionTarget.NamedSessionKey,
		item.Source.SessionKey,
	} {
		if _, exists := sessionKeys[strings.TrimSpace(sessionKey)]; exists {
			return true
		}
	}
	return false
}

func (s *Service) resolveAutomationWorkspacePath(ctx context.Context, agentID string) (string, error) {
	if s.agents != nil && strings.TrimSpace(agentID) != "" {
		agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
		if err != nil {
			return "", err
		}
		if workspacePath := strings.TrimSpace(agentValue.WorkspacePath); workspacePath != "" {
			return workspacePath, nil
		}
	}
	return strings.TrimSpace(s.config.WorkspacePath), nil
}
