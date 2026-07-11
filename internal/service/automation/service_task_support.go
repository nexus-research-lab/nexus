package automation

import (
	"context"
	"fmt"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

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
	count, err := s.repository.CountEnabledCronJobs(ctx, strings.TrimSpace(ownerUserID), "")
	if err != nil {
		return fmt.Errorf("统计已启用自动化任务: %w", err)
	}
	if count >= limit {
		return fmt.Errorf("每个用户启用的定时任务不能超过 %d 个", limit)
	}
	return nil
}

func (s *Service) cleanupIsolatedAutomationSessions(ctx context.Context, job automationdomain.CronJob) error {
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
	prefix := fmt.Sprintf("agent:%s:automation:dm:cron:%s:", strings.TrimSpace(job.AgentID), strings.TrimSpace(job.JobID))
	files := workspacestore.NewSessionFileStore(s.config.WorkspacePath)
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
		if _, deleteErr := files.DeleteSession(workspacePath, sessionKey); deleteErr != nil {
			return deleteErr
		}
		if s.sessionCloser != nil {
			_ = s.sessionCloser.CloseSession(context.Background(), sessionKey)
		}
	}
	return nil
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
