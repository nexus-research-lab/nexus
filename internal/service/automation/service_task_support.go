package automation

import (
	"context"
	"fmt"
	"strings"

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

func (s *Service) cleanupIsolatedAutomationSessions(ctx context.Context, job protocol.CronJob) error {
	if strings.TrimSpace(job.SessionTarget.Kind) != protocol.SessionTargetIsolated {
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
