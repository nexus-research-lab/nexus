package dm

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	memorysvc "github.com/nexus-research-lab/nexus/internal/workspace/memory"
)

func (s *Service) memoryOptions() memorysvc.MemoryOptions {
	return memorysvc.MemoryOptions{
		Enabled:        s.config.MemoryEnabled,
		AutoRecall:     s.config.MemoryAutoRecall,
		AutoExtract:    s.config.MemoryAutoExtract,
		MaxResults:     s.config.MemoryMaxResults,
		ScoreThreshold: s.config.MemoryScoreThreshold,
	}.Normalize()
}

func (s *Service) injectMemoryContext(
	ctx context.Context,
	agentValue *protocol.Agent,
	session protocol.Session,
	sessionKey string,
	content string,
	runtimeContent conversationsvc.RuntimeContent,
) conversationsvc.RuntimeContent {
	if agentValue == nil || runtimeContent.IsEmpty() {
		return runtimeContent
	}
	engine := memorysvc.NewEngine(agentValue.WorkspacePath, s.memoryOptions())
	injection, err := engine.BeforeRecall(ctx, memorysvc.MemoryScope{
		Kind:       memorysvc.ScopeKindDMSession,
		UserID:     authctx.OwnerUserID(ctx),
		AgentID:    agentValue.AgentID,
		SessionKey: sessionKey,
		SessionID:  sessionIDString(session),
	}, memorysvc.RecallRequest{
		Query:      content,
		MaxResults: s.config.MemoryMaxResults,
	})
	if err != nil {
		s.loggerFor(ctx).Warn("DM 动态记忆召回失败",
			"session_key", sessionKey,
			"agent_id", agentValue.AgentID,
			"err", err,
		)
		return runtimeContent
	}
	if strings.TrimSpace(injection.DynamicUserContext) == "" {
		return runtimeContent
	}
	return runtimeContent.PrependText(injection.DynamicUserContext)
}

func (r *roundRunner) commitMemoryTurn() {
	if r == nil || r.agent == nil {
		return
	}
	assistantText := memorysvc.ExtractMessageText(r.mapper.LastAssistantMessage())
	if strings.TrimSpace(assistantText) == "" {
		return
	}
	engine := memorysvc.NewEngine(r.workspacePath, r.service.memoryOptions())
	_, err := engine.CommitTurn(context.Background(), memorysvc.MemoryScope{
		Kind:       memorysvc.ScopeKindDMSession,
		UserID:     r.ownerUserID,
		AgentID:    r.agent.AgentID,
		SessionKey: r.sessionKey,
		SessionID:  sessionIDString(r.session),
	}, memorysvc.CommittedTurn{
		UserText:      r.content,
		AssistantText: assistantText,
		SessionKey:    r.sessionKey,
		SessionID:     sessionIDString(r.session),
		RoundID:       r.roundID,
		AgentID:       r.agent.AgentID,
	})
	if err != nil {
		r.service.loggerFor(context.Background()).Warn("DM 自动记忆提交失败",
			"session_key", r.sessionKey,
			"agent_id", r.agent.AgentID,
			"round_id", r.roundID,
			"err", err,
		)
	}
}

func sessionIDString(session protocol.Session) string {
	if session.SessionID == nil {
		return ""
	}
	return strings.TrimSpace(*session.SessionID)
}
