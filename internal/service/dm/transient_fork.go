// INPUT: 同 Agent 的源 DM/Room Session、可选 transcript 边界与宿主生成的隐藏目标 Session。
// OUTPUT: 继承模型上下文、但从普通目录隐藏且只展示新分支消息的临时 DM Session。
// POS: WorkGraph 等受限嵌入式编辑器复用标准 DM runtime 的 fork 边界。
package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// TransientForkRequest 只由宿主组合层构造；HTTP/WS 不能写入 Session 用途或 fork identity。
type TransientForkRequest struct {
	SourceSessionKey      string
	TargetSessionKey      string
	TargetRoundID         string
	Purpose               string
	Title                 string
	DisplayAfterUnixMilli int64
}

// CreateTransientFork 创建不进入普通目录的真实 DM transcript 分支。
func (s *Service) CreateTransientFork(
	ctx context.Context,
	request TransientForkRequest,
) (*protocol.Session, error) {
	if s == nil {
		return nil, errors.New("DM service is unavailable")
	}
	source, target, err := validateTransientForkKeys(
		request.SourceSessionKey,
		request.TargetSessionKey,
	)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.Purpose) == "" ||
		request.DisplayAfterUnixMilli <= 0 {
		return nil, errors.New("transient fork boundary is incomplete")
	}
	agentValue, err := s.agents.GetAgent(ctx, source.AgentID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(agentValue.OwnerUserID) != authctx.OwnerUserID(ctx) {
		return nil, errors.New("transient fork owner does not match Agent owner")
	}
	sourceSession, err := s.ensureSession(ctx, agentValue, source, source.Raw)
	if err != nil {
		return nil, err
	}
	targetRoundID := strings.TrimSpace(request.TargetRoundID)
	if targetRoundID == "" {
		targetRoundID, err = s.resolveLatestCompletedForkRound(
			ctx,
			agentValue,
			sourceSession,
			source.Raw,
		)
		if err != nil {
			return nil, err
		}
	}
	sourceSessionID, sourceMessageID, err := s.prepareConversationFork(
		ctx,
		source.Raw,
		targetRoundID,
		validateTransientForkSourceKey,
	)
	if err != nil {
		return nil, err
	}
	targetSession, err := s.ensureSession(ctx, agentValue, target, target.Raw)
	if err != nil {
		return nil, err
	}
	if targetSession.SessionID != nil && strings.TrimSpace(*targetSession.SessionID) != "" {
		return nil, errors.New("transient target Session already has a runtime transcript")
	}
	targetSession.Title = strings.TrimSpace(request.Title)
	if targetSession.Title == "" {
		targetSession.Title = "Temporary conversation"
	}
	targetSession.Options = protocol.WithSessionRuntimeSettings(
		targetSession.Options,
		protocol.SessionRuntimeSettingsFromOptions(sourceSession.Options),
	)
	targetSession.Options[protocol.OptionRuntimeForkSourceSessionID] = sourceSessionID
	targetSession.Options[protocol.OptionRuntimeForkMessageID] = sourceMessageID
	targetSession.Options[protocol.OptionSessionHiddenFromDirectory] = true
	targetSession.Options[protocol.OptionSessionPurpose] = strings.TrimSpace(request.Purpose)
	targetSession.Options[protocol.OptionSessionDisplayAfterUnixMilli] = request.DisplayAfterUnixMilli
	created, err := s.files.ForOwner(agentValue.OwnerUserID).UpsertSession(
		agentValue.WorkspacePath,
		targetSession,
	)
	if err != nil {
		return nil, err
	}
	if created != nil {
		targetSession = *created
	}
	if err = s.forkConversationSession(
		ctx,
		source.Raw,
		target.Raw,
		targetRoundID,
		sourceSessionID,
		sourceMessageID,
		validateTransientForkKeys,
	); err != nil {
		_, cleanupErr := s.files.ForOwner(agentValue.OwnerUserID).DeleteSession(
			agentValue.WorkspacePath,
			target.Raw,
		)
		return nil, errors.Join(err, cleanupErr)
	}
	result, err := s.ensureSession(ctx, agentValue, target, target.Raw)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Service) resolveLatestCompletedForkRound(
	ctx context.Context,
	agentValue *protocol.Agent,
	sourceSession protocol.Session,
	sourceSessionKey string,
) (string, error) {
	activeRoundIDs := s.runtime.GetRunningRoundIDs(sourceSessionKey)
	query := workspacestore.HistoryPageQuery{Limit: 32}
	for {
		page, err := s.history.ForOwner(agentValue.OwnerUserID).ReadMessagesPageContext(
			ctx,
			agentValue.WorkspacePath,
			sourceSession,
			activeRoundIDs,
			query,
		)
		if err != nil {
			return "", fmt.Errorf("读取 source conversation 最近轮次: %w", err)
		}
		if roundID := latestCompletedAssistantRound(page.Items, activeRoundIDs); roundID != "" {
			return roundID, nil
		}
		if !page.HasMore || page.NextBeforeRoundID == nil {
			break
		}
		query.BeforeRoundID = strings.TrimSpace(*page.NextBeforeRoundID)
		query.BeforeRoundTimestamp = 0
		if page.NextBeforeRoundTimestamp != nil {
			query.BeforeRoundTimestamp = *page.NextBeforeRoundTimestamp
		}
		if query.BeforeRoundID == "" {
			break
		}
	}
	return "", errors.New("source conversation 没有可分支的已完成助手回复")
}
