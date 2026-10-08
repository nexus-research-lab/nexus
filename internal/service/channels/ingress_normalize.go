package channels

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelmanagement "github.com/nexus-research-lab/nexus/internal/service/channels/management"
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

	ownerUserID := normalizeChannelOwnerUserID(textutil.FirstNonEmpty(request.OwnerUserID, authctx.OwnerUserID(ctx)))
	ownerCtx := contextWithIngressOwner(ctx, ownerUserID)
	sessionKey, parsed, agentID, err := s.resolveSession(ownerCtx, request)
	if err != nil {
		return normalizedIngressRequest{}, err
	}

	channelStored := protocol.NormalizeStoredChannelType(parsed.Channel)
	accountID := strings.TrimSpace(parsed.AccountID)
	trustedExternalInteractive, err := s.validateResolvedExternalIngress(
		ownerCtx,
		ownerUserID,
		agentID,
		parsed,
	)
	if err != nil {
		return normalizedIngressRequest{}, err
	}
	rememberedTarget, err := s.resolveRememberedTarget(channelStored, parsed, request.Delivery)
	if err != nil {
		return normalizedIngressRequest{}, err
	}
	var pairing *pairingRow
	if s.control != nil && isExternalIngressChannel(channelStored) {
		pairing, err = s.control.ingressPairing(ownerCtx, ownerUserID, agentID, sessionKey)
		if err != nil {
			return normalizedIngressRequest{}, err
		}
		if rememberedTarget != nil {
			rememberedTarget.PairingID = pairing.PairingID
			rememberedTarget.BindingVersion = pairing.BindingVersion
		}
	}
	targetRoomType := ""
	if pairing != nil && pairing.TargetRoomID != "" {
		if err := s.control.validatePairingRoom(ownerCtx, agentID, channelmanagement.PairingSessionTarget{RoomID: pairing.TargetRoomID, ConversationID: pairing.TargetConversationID}); err != nil {
			return normalizedIngressRequest{}, err
		}
		target, err := s.control.rooms.GetConversationContext(ownerCtx, pairing.TargetConversationID)
		if err != nil {
			return normalizedIngressRequest{}, err
		}
		targetRoomType = target.Room.RoomType
	}
	roundID := textutil.FirstNonEmpty(request.RoundID, s.idFactory("ingress_round"))
	reqID := textutil.FirstNonEmpty(request.ReqID, request.RoundID, roundID)
	message := migrateIngressMessage(request, channelStored, parsed, content, reqID)

	return normalizedIngressRequest{
		pairing:                    pairing,
		targetRoomType:             targetRoomType,
		ownerUserID:                ownerUserID,
		channelStored:              channelStored,
		accountID:                  accountID,
		sessionKey:                 sessionKey,
		parsed:                     parsed,
		agentID:                    agentID,
		content:                    content,
		roundID:                    roundID,
		reqID:                      reqID,
		permissionMode:             sdkpermission.Mode(strings.TrimSpace(request.PermissionMode)),
		autoApproveAll:             request.AutoApproveAll,
		autoApproveTools:           s.resolveApprovedTools(channelStored, request.AutoApproveTools),
		trustedExternalInteractive: trustedExternalInteractive,
		rememberedTarget:           rememberedTarget,
		message:                    message,
	}, nil
}

func (s *IngressService) validateResolvedExternalIngress(
	ctx context.Context,
	ownerUserID string,
	agentID string,
	parsed protocol.SessionKey,
) (bool, error) {
	channel := normalizeIMChannelType(parsed.Channel)
	if channel == "" || channel == ChannelTypeInternal || channel == ChannelTypeWebSocket {
		return false, nil
	}
	if s.control == nil {
		return false, nil
	}
	if err := s.control.ValidateExternalSessionGrant(ctx, ownerUserID, agentID, parsed.Raw); err != nil {
		return false, err
	}
	return protocol.NormalizeSessionChatType(parsed.ChatType) == protocol.RoomTypeDM, nil
}

func contextWithIngressOwner(ctx context.Context, ownerUserID string) context.Context {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	if currentUserID, ok := authctx.CurrentUserID(ctx); ok && currentUserID == ownerUserID {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodLocal,
	})
}
