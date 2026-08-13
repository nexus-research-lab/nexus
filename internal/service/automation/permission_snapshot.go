// INPUT: 新任务、执行 Session 与当前 Agent runtime 配置。
// OUTPUT: 创建时固化的具体 permission_mode 和完整工具权限快照。
// POS: automation 创建语义中的 copy-on-create 边界；后续 Agent 变更不回写任务。
package automation

import (
	"context"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type initialTaskPermissionSnapshot struct {
	Mode         string
	AgentOptions protocol.Options
}

func (s *Service) resolveInitialTaskPermissionSnapshot(
	ctx context.Context,
	job automationdomain.ScheduledTask,
) (initialTaskPermissionSnapshot, error) {
	var agentValue *protocol.Agent
	if s.agents != nil && strings.TrimSpace(job.AgentID) != "" {
		loaded, err := s.requireAgent(ctx, job.AgentID)
		if err != nil {
			return initialTaskPermissionSnapshot{}, err
		}
		agentValue = loaded
	}

	options := protocol.Options{}
	if agentValue != nil {
		options = agentValue.Options
		options.AllowedTools = append([]string(nil), agentValue.Options.AllowedTools...)
		options.DisallowedTools = append([]string(nil), agentValue.Options.DisallowedTools...)
	}

	mode := strings.TrimSpace(job.PermissionMode)
	if mode == "" {
		var err error
		mode, err = s.executionSessionPermissionMode(ctx, job, agentValue)
		if err != nil {
			return initialTaskPermissionSnapshot{}, err
		}
	}
	if mode == "" {
		mode = strings.TrimSpace(options.PermissionMode)
	}
	return initialTaskPermissionSnapshot{
		Mode:         concreteTaskPermissionMode(mode),
		AgentOptions: options,
	}, nil
}

func (s *Service) executionSessionPermissionMode(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	agentValue *protocol.Agent,
) (string, error) {
	if strings.TrimSpace(job.SessionTarget.Kind) != automationdomain.SessionTargetBound {
		return "", nil
	}
	sessionKey := strings.TrimSpace(job.SessionTarget.BoundSessionKey)
	if sessionKey == "" {
		return "", nil
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured {
		return "", nil
	}
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return s.roomSessionPermissionMode(ctx, parsed.ConversationID, job.AgentID)
	}
	if parsed.Kind != protocol.SessionKeyKindAgent ||
		strings.TrimSpace(parsed.AgentID) != strings.TrimSpace(job.AgentID) ||
		agentValue == nil {
		return "", nil
	}
	stored, _, err := workspacestore.NewSessionFileStore(s.config.WorkspacePath).
		ForOwner(job.OwnerUserID).
		FindSession([]string{agentValue.WorkspacePath}, sessionKey)
	if err != nil || stored == nil {
		return "", err
	}
	return protocol.SessionRuntimeSettingsFromOptions(stored.Options).PermissionMode, nil
}

func (s *Service) roomSessionPermissionMode(
	ctx context.Context,
	conversationID string,
	agentID string,
) (string, error) {
	if s.room == nil || strings.TrimSpace(conversationID) == "" {
		return "", nil
	}
	contextValue, err := s.room.GetConversationContext(ctx, strings.TrimSpace(conversationID))
	if err != nil || contextValue == nil {
		return "", err
	}
	var fallback *protocol.SessionRecord
	for index := range contextValue.Sessions {
		session := &contextValue.Sessions[index]
		if strings.TrimSpace(session.AgentID) != strings.TrimSpace(agentID) {
			continue
		}
		if session.IsPrimary {
			return protocol.SessionRuntimeSettingsFromOptions(session.Options).PermissionMode, nil
		}
		if fallback == nil {
			fallback = session
		}
	}
	if fallback == nil {
		return "", nil
	}
	return protocol.SessionRuntimeSettingsFromOptions(fallback.Options).PermissionMode, nil
}

func concreteTaskPermissionMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case automationdomain.PermissionModePlan:
		return automationdomain.PermissionModePlan
	case automationdomain.PermissionModeAcceptEdits:
		return automationdomain.PermissionModeAcceptEdits
	case automationdomain.PermissionModeBypassPermissions:
		return automationdomain.PermissionModeBypassPermissions
	case automationdomain.PermissionModeDontAsk:
		return automationdomain.PermissionModeDontAsk
	default:
		return automationdomain.PermissionModeDefault
	}
}
