// INPUT: owner 作用域 Session 列表、Channel pairing/account 实时状态与 Automation 引用计数。
// OUTPUT: 带账号短标识、当前/历史状态和可删除事实的外部 IM Session 目录投影。
// POS: Session 查询模型的外部传输身份聚合；删除规则复用同一真相源并保持 fail closed。
package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func isExternalAgentSession(parsed protocol.SessionKey) bool {
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent {
		return false
	}
	switch protocol.NormalizeStoredChannelType(parsed.Channel) {
	case protocol.SessionChannelDiscord,
		protocol.SessionChannelTelegram,
		protocol.SessionChannelDingTalk,
		protocol.SessionChannelWeChat,
		protocol.SessionChannelWeixinPersonal,
		protocol.SessionChannelFeishu:
		return true
	default:
		return false
	}
}

func (s *Service) projectExternalSessionIdentities(
	ctx context.Context,
	items []protocol.Session,
) ([]protocol.Session, error) {
	if len(items) == 0 || s.externalIdentity == nil {
		return items, nil
	}

	externalKeys := make([]string, 0, len(items))
	for _, item := range items {
		if isExternalAgentSession(protocol.ParseSessionKey(item.SessionKey)) {
			externalKeys = append(externalKeys, strings.TrimSpace(item.SessionKey))
		}
	}
	if len(externalKeys) == 0 {
		return items, nil
	}

	ownerUserID := authctx.OwnerUserID(ctx)
	referenceCounts := map[string]int{}
	if s.taskReferences != nil {
		counts, err := s.taskReferences.CountTasksReferencingSessions(
			ctx,
			ownerUserID,
			externalKeys,
		)
		if err != nil {
			return nil, fmt.Errorf("读取 IM Session 定时任务引用: %w", err)
		}
		referenceCounts = counts
	}

	for index := range items {
		sessionKey := strings.TrimSpace(items[index].SessionKey)
		if !isExternalAgentSession(protocol.ParseSessionKey(sessionKey)) {
			continue
		}
		identity, err := s.externalIdentity.ResolveExternalSessionIdentity(
			ctx,
			ownerUserID,
			sessionKey,
		)
		if err != nil {
			return nil, fmt.Errorf("读取 IM Session 身份 %s: %w", sessionKey, err)
		}
		if identity == nil {
			continue
		}
		identity.TaskReferenceCount = referenceCounts[sessionKey]
		identity.CanDelete = identity.CanDelete &&
			s.taskReferences != nil
		items[index].ExternalIdentity = identity
	}
	return items, nil
}

func (s *Service) projectExternalSessionIdentity(
	ctx context.Context,
	item protocol.Session,
) (protocol.Session, error) {
	items, err := s.projectExternalSessionIdentities(ctx, []protocol.Session{item})
	if err != nil {
		return protocol.Session{}, err
	}
	return items[0], nil
}

func (s *Service) validateExternalSessionDeletion(
	ctx context.Context,
	sessionKey string,
	parsed protocol.SessionKey,
) error {
	if !isExternalAgentSession(parsed) {
		return nil
	}
	if s.externalIdentity == nil || s.taskReferences == nil {
		return errorsNewExternalSessionDeletionUnavailable()
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	identity, err := s.externalIdentity.ResolveExternalSessionIdentity(
		ctx,
		ownerUserID,
		sessionKey,
	)
	if err != nil {
		return fmt.Errorf("确认 IM Session 配对状态: %w", err)
	}
	if identity == nil {
		return errorsNewExternalSessionDeletionUnavailable()
	}
	if identity.CurrentPairing {
		return fmt.Errorf(
			"%w: 当前 IM 会话仍在使用，请先解绑对应账号或会话",
			ErrExternalSessionPairingActive,
		)
	}
	return nil
}

func errorsNewExternalSessionDeletionUnavailable() error {
	return fmt.Errorf(
		"%w: 无法确认 IM 配对或任务生命周期状态",
		ErrSessionMutationUnsupported,
	)
}
