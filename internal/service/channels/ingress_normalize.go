package channels

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func migrateIngressMessage(
	request IngressRequest,
	channelStored string,
	parsed protocol.SessionKey,
	content string,
	reqID string,
) *channelmessage.Inbound {
	return channelmessage.NormalizeInbound(request.Message, channelmessage.InboundParams{
		Channel:           channelStored,
		Target:            parsed.Ref,
		PlatformMessageID: reqID,
		ThreadID:          parsed.ThreadID,
		SenderName:        request.ExternalName,
		ChatType:          parsed.ChatType,
		Text:              content,
	})
}

func (s *IngressService) normalizeRequest(ctx context.Context, request IngressRequest) (normalizedIngressRequest, error) {
	content := strings.TrimSpace(request.Content)
	if content == "" {
		return normalizedIngressRequest{}, errors.New("content is required")
	}

	ownerUserID := normalizeChannelOwnerUserID(firstNonEmptyIngress(request.OwnerUserID, authctx.OwnerUserID(ctx)))
	ownerCtx := contextWithIngressOwner(ctx, ownerUserID)
	sessionKey, parsed, agentID, err := s.resolveSession(ownerCtx, request)
	if err != nil {
		return normalizedIngressRequest{}, err
	}

	channelStored := protocol.NormalizeStoredChannelType(parsed.Channel)
	accountID := strings.TrimSpace(parsed.AccountID)
	rememberedTarget, err := s.resolveRememberedTarget(channelStored, parsed, request.Delivery)
	if err != nil {
		return normalizedIngressRequest{}, err
	}
	roundID := firstNonEmptyIngress(request.RoundID, s.idFactory("ingress_round"))
	reqID := firstNonEmptyIngress(request.ReqID, request.RoundID, roundID)
	message := migrateIngressMessage(request, channelStored, parsed, content, reqID)

	return normalizedIngressRequest{
		ownerUserID:      ownerUserID,
		channelStored:    channelStored,
		accountID:        accountID,
		sessionKey:       sessionKey,
		parsed:           parsed,
		agentID:          agentID,
		content:          content,
		roundID:          roundID,
		reqID:            reqID,
		permissionMode:   sdkpermission.Mode(strings.TrimSpace(request.PermissionMode)),
		autoApproveAll:   request.AutoApproveAll,
		autoApproveTools: s.resolveApprovedTools(channelStored, request.AutoApproveTools),
		rememberedTarget: rememberedTarget,
		message:          message,
	}, nil
}

func contextWithIngressOwner(ctx context.Context, ownerUserID string) context.Context {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	if currentUserID, ok := authctx.CurrentUserID(ctx); ok && strings.TrimSpace(currentUserID) == ownerUserID {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodLocal,
	})
}

func firstNonEmptyIngress(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
