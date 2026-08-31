// INPUT: authenticated WebSocket sender, exact business-session binding, and human-only QR/code payload.
// OUTPUT: principal/session-targeted native presentation plus a secret-free submission ACK.
// POS: Channel authorization human transport; verification material never enters model events or logs.
package websocket

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authorizationsvc "github.com/nexus-research-lab/nexus/internal/service/channelauthorization"
)

const (
	channelAuthorizationCodeMaxRunes = 256
	channelAuthorizationQRMaxBytes   = 512 << 10
)

// ChannelAuthorizationController 是原生人类输入边界唯一需要的服务能力。
type ChannelAuthorizationController interface {
	SubmitHumanVerificationCode(
		context.Context,
		authorizationsvc.HumanVerificationCodeSubmission,
	) (*authorizationsvc.View, error)
	Cancel(
		context.Context,
		authorizationsvc.Actor,
		string,
	) (*authorizationsvc.View, error)
}

type channelAuthorizationSender interface {
	Key() string
	IsClosed() bool
	SendEvent(context.Context, protocol.EventMessage) error
}

type authenticatedChannelAuthorizationSender struct {
	principalUserID string
	principalRole   string
	authMethod      string
	authSessionID   string
	localSingleUser bool
	sender          channelAuthorizationSender
}

type channelAuthorizationRoute struct {
	flowID                 string
	presentationToken      string
	principalUserID        string
	principalAuthMethod    string
	principalAuthSessionID string
	agentID                string
	businessSessionKey     string
	rootRoundID            string
	runtimeLeaseSessionKey string
	runtimeLeaseRoundID    string
	expiresAt              time.Time
	senderKeys             map[string]struct{}
}

type channelAuthorizationTransport struct {
	mu         sync.Mutex
	controller ChannelAuthorizationController
	senders    map[string]authenticatedChannelAuthorizationSender
	routes     map[string]channelAuthorizationRoute
}

func newChannelAuthorizationTransport() *channelAuthorizationTransport {
	return &channelAuthorizationTransport{
		senders: make(map[string]authenticatedChannelAuthorizationSender),
		routes:  make(map[string]channelAuthorizationRoute),
	}
}

// SetChannelAuthorizationController 注入验证码消费服务；presenter 与 controller
// 分开装配，避免服务层持有 WebSocket 或把人类输入回流到 MCP。
func (h *Handler) SetChannelAuthorizationController(controller ChannelAuthorizationController) {
	if h == nil {
		return
	}
	transport := h.ensureChannelAuthorizationTransport()
	transport.mu.Lock()
	transport.controller = controller
	transport.mu.Unlock()
}

// PresentChannelAuthorization 实现 service/channelauthorization.HumanPresenter。
// 仅同 principal 且已绑定原始 business session 的认证连接可收到材料。
func (h *Handler) PresentChannelAuthorization(
	ctx context.Context,
	presentation authorizationsvc.HumanPresentation,
) error {
	if h == nil || h.permission == nil {
		return errors.New("Channel 授权缺少可信 WebSocket 会话路由")
	}
	transport := h.ensureChannelAuthorizationTransport()
	return transport.present(ctx, h.permission, presentation)
}

func (h *Handler) ensureChannelAuthorizationTransport() *channelAuthorizationTransport {
	if h.channelAuthorization != nil {
		return h.channelAuthorization
	}
	// Handler 在装配后不再并发替换整个 transport；测试或旧构造路径可按需补齐。
	h.channelAuthorization = newChannelAuthorizationTransport()
	return h.channelAuthorization
}

func (t *channelAuthorizationTransport) registerAuthenticatedSender(
	ctx context.Context,
	sender channelAuthorizationSender,
) {
	if t == nil || sender == nil || sender.IsClosed() {
		return
	}
	principalUserID, ok := authctx.CurrentUserID(ctx)
	localSingleUser := authctx.IsLocalSingleUserControlPlane(ctx, authctx.SystemUserID)
	principalRole := ""
	authMethod := ""
	authSessionID := ""
	if principal := authctx.PrincipalFromContext(ctx); principal != nil {
		principalRole = strings.TrimSpace(principal.Role)
		authMethod = strings.TrimSpace(principal.AuthMethod)
		if principal.SessionID != nil {
			authSessionID = strings.TrimSpace(*principal.SessionID)
		}
	}
	if !ok && localSingleUser {
		principalUserID = authctx.SystemUserID
		principalRole = authctx.RoleOwner
		authMethod = authctx.AuthMethodLocal
		ok = true
	}
	switch authMethod {
	case authctx.AuthMethodPassword:
		if authSessionID == "" {
			return
		}
	case authctx.AuthMethodLocal:
		evidence, hasEvidence := authctx.InteractiveHumanEvidenceFromContext(ctx)
		if !localSingleUser ||
			!hasEvidence ||
			evidence.Source != "desktop_session_token" {
			return
		}
	default:
		return
	}
	if !ok || strings.TrimSpace(principalUserID) == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pruneLocked(time.Now().UTC())
	t.senders[sender.Key()] = authenticatedChannelAuthorizationSender{
		principalUserID: strings.TrimSpace(principalUserID),
		principalRole:   principalRole,
		authMethod:      authMethod,
		authSessionID:   authSessionID,
		localSingleUser: localSingleUser,
		sender:          sender,
	}
}

func (t *channelAuthorizationTransport) unregisterSender(sender channelAuthorizationSender) {
	if t == nil || sender == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.senders, sender.Key())
	for token, route := range t.routes {
		delete(route.senderKeys, sender.Key())
		if len(route.senderKeys) == 0 {
			delete(t.routes, token)
			continue
		}
		t.routes[token] = route
	}
}

func (t *channelAuthorizationTransport) present(
	ctx context.Context,
	permission *permissionctx.Context,
	presentation authorizationsvc.HumanPresentation,
) error {
	flowID := strings.TrimSpace(presentation.FlowID)
	token := strings.TrimSpace(presentation.PresentationToken)
	principalUserID := strings.TrimSpace(presentation.PrincipalUserID)
	principalAuthMethod := strings.TrimSpace(presentation.PrincipalAuthMethod)
	principalAuthSessionID := strings.TrimSpace(presentation.PrincipalAuthSessionID)
	businessSessionKey := strings.TrimSpace(presentation.BusinessSessionKey)
	kind := strings.TrimSpace(presentation.Kind)
	qrPayloadType := strings.ToLower(strings.TrimSpace(presentation.QRPayloadType))
	if flowID == "" || token == "" || principalUserID == "" ||
		principalAuthMethod == "" ||
		businessSessionKey == "" ||
		strings.TrimSpace(presentation.AgentID) == "" ||
		strings.TrimSpace(presentation.RootRoundID) == "" ||
		strings.TrimSpace(presentation.RuntimeLeaseSessionKey) == "" ||
		strings.TrimSpace(presentation.RuntimeLeaseRoundID) == "" ||
		(kind != authorizationsvc.PresentationKindQRCode &&
			kind != authorizationsvc.PresentationKindVerificationCode) ||
		(kind == authorizationsvc.PresentationKindQRCode &&
			(strings.TrimSpace(presentation.QRPayload) == "" ||
				!utf8.ValidString(presentation.QRPayload) ||
				len(presentation.QRPayload) > channelAuthorizationQRMaxBytes ||
				(qrPayloadType != "text" && qrPayloadType != "url"))) ||
		!time.Now().UTC().Before(presentation.ExpiresAt.UTC()) {
		return errors.New("Channel 授权展示路由无效或已过期")
	}

	candidates := permission.ResolveSessionSenders(businessSessionKey)
	eligible := make([]channelAuthorizationSender, 0, len(candidates))
	t.mu.Lock()
	t.pruneLocked(time.Now().UTC())
	for _, candidate := range candidates {
		if candidate == nil || candidate.IsClosed() {
			continue
		}
		registered, ok := t.senders[candidate.Key()]
		if !ok || registered.sender == nil || registered.sender.IsClosed() ||
			registered.principalUserID != principalUserID ||
			registered.authMethod != principalAuthMethod ||
			registered.authSessionID != principalAuthSessionID {
			continue
		}
		eligible = append(eligible, registered.sender)
	}
	if len(eligible) == 0 {
		t.mu.Unlock()
		return errors.New("当前认证用户没有绑定原始 Channel 授权会话")
	}
	route := channelAuthorizationRoute{
		flowID:                 flowID,
		presentationToken:      token,
		principalUserID:        principalUserID,
		principalAuthMethod:    principalAuthMethod,
		principalAuthSessionID: principalAuthSessionID,
		agentID:                strings.TrimSpace(presentation.AgentID),
		businessSessionKey:     businessSessionKey,
		rootRoundID:            strings.TrimSpace(presentation.RootRoundID),
		runtimeLeaseSessionKey: strings.TrimSpace(presentation.RuntimeLeaseSessionKey),
		runtimeLeaseRoundID:    strings.TrimSpace(presentation.RuntimeLeaseRoundID),
		expiresAt:              presentation.ExpiresAt.UTC(),
		senderKeys:             make(map[string]struct{}, len(eligible)),
	}
	for _, sender := range eligible {
		route.senderKeys[sender.Key()] = struct{}{}
	}
	t.routes[token] = route
	t.mu.Unlock()

	event := protocol.NewChannelAuthorizationEvent(
		businessSessionKey,
		protocol.ChannelAuthorizationData{
			FlowID:            flowID,
			PresentationToken: token,
			Kind:              kind,
			ChannelType:       strings.TrimSpace(presentation.ChannelType),
			AccountBinding:    strings.TrimSpace(presentation.AccountBinding),
			QRPayload:         presentation.QRPayload,
			QRPayloadType:     qrPayloadType,
			Prompt:            strings.TrimSpace(presentation.Prompt),
			ExpiresAt:         presentation.ExpiresAt.UTC(),
		},
	)

	successful := make(map[string]struct{}, len(eligible))
	for _, sender := range eligible {
		if err := sender.SendEvent(ctx, event); err == nil {
			successful[sender.Key()] = struct{}{}
		}
	}
	t.mu.Lock()
	if len(successful) == 0 {
		delete(t.routes, token)
		t.mu.Unlock()
		return errors.New("无法向原始认证会话安全展示 Channel 授权")
	}
	current, exists := t.routes[token]
	if exists && current.flowID == flowID && current.presentationToken == token {
		current.senderKeys = successful
		t.routes[token] = current
	}
	t.mu.Unlock()
	return nil
}

func (h *Handler) handleChannelAuthorizationCode(
	ctx context.Context,
	sender channelAuthorizationSender,
	inbound map[string]any,
) {
	flowID := stringWireValue(inbound["flow_id"])
	token := stringWireValue(inbound["presentation_token"])
	code := stringWireValue(inbound["code"])
	// 尽早从通用 wire map 移除，后续错误路径或调试输出不得意外携带验证码。
	delete(inbound, "code")

	transport := h.ensureChannelAuthorizationTransport()
	submission, controller, err := transport.buildSubmission(
		h.permission,
		sender,
		flowID,
		token,
		code,
	)
	code = ""
	if err != nil {
		h.sendChannelAuthorizationResult(
			ctx,
			sender,
			flowID,
			false,
			"",
			"验证码提交已拒绝，请从当前授权会话重新打开安全输入卡。",
		)
		return
	}
	view, submitErr := controller.SubmitHumanVerificationCode(ctx, submission)
	submission.Code = ""
	if submitErr != nil {
		h.sendChannelAuthorizationResult(
			ctx,
			sender,
			flowID,
			false,
			"",
			"验证码未被当前授权会话接受，请核对后重试。",
		)
		return
	}
	transport.consumeRoute(token)
	status := ""
	message := "验证码已安全提交，正在等待平台确认。"
	if view != nil {
		status = strings.TrimSpace(view.Status)
		if strings.TrimSpace(view.Message) != "" {
			message = strings.TrimSpace(view.Message)
		}
	}
	h.sendChannelAuthorizationResult(ctx, sender, flowID, true, status, message)
}

func (h *Handler) handleChannelAuthorizationCancel(
	ctx context.Context,
	sender channelAuthorizationSender,
	inbound map[string]any,
) {
	flowID := stringWireValue(inbound["flow_id"])
	token := stringWireValue(inbound["presentation_token"])
	transport := h.ensureChannelAuthorizationTransport()
	actor, controller, err := transport.buildCancellation(
		h.permission,
		sender,
		flowID,
		token,
	)
	if err != nil {
		h.sendChannelAuthorizationResult(
			ctx,
			sender,
			flowID,
			false,
			"",
			"取消请求已拒绝，请从原始授权会话重试。",
		)
		return
	}
	view, cancelErr := controller.Cancel(ctx, actor, flowID)
	if cancelErr != nil {
		h.sendChannelAuthorizationResult(
			ctx,
			sender,
			flowID,
			false,
			"",
			"当前授权会话无法取消，状态待核对；请刷新频道状态后再操作。",
		)
		return
	}
	transport.consumeRoute(token)
	status := "cancelled"
	message := "Channel 授权已取消。"
	if view != nil {
		if strings.TrimSpace(view.Status) != "" {
			status = strings.TrimSpace(view.Status)
		}
		if strings.TrimSpace(view.Message) != "" {
			message = strings.TrimSpace(view.Message)
		}
	}
	h.sendChannelAuthorizationResult(ctx, sender, flowID, true, status, message)
}

func stringWireValue(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func (t *channelAuthorizationTransport) buildSubmission(
	permission *permissionctx.Context,
	sender channelAuthorizationSender,
	flowID string,
	token string,
	code string,
) (authorizationsvc.HumanVerificationCodeSubmission, ChannelAuthorizationController, error) {
	if t == nil || permission == nil || sender == nil || sender.IsClosed() {
		return authorizationsvc.HumanVerificationCodeSubmission{}, nil, errors.New("invalid sender")
	}
	flowID = strings.TrimSpace(flowID)
	token = strings.TrimSpace(token)
	code = strings.TrimSpace(code)
	if flowID == "" || token == "" || code == "" ||
		utf8.RuneCountInString(code) > channelAuthorizationCodeMaxRunes {
		return authorizationsvc.HumanVerificationCodeSubmission{}, nil, errors.New("invalid submission")
	}

	route, _, controller, err := t.resolveRoute(
		permission,
		sender,
		flowID,
		token,
	)
	if err != nil {
		return authorizationsvc.HumanVerificationCodeSubmission{}, nil, err
	}
	return authorizationsvc.HumanVerificationCodeSubmission{
		FlowID:                 route.flowID,
		PresentationToken:      route.presentationToken,
		OwnerUserID:            route.principalUserID,
		PrincipalUserID:        route.principalUserID,
		PrincipalAuthSessionID: route.principalAuthSessionID,
		AgentID:                route.agentID,
		BusinessSessionKey:     route.businessSessionKey,
		RootRoundID:            route.rootRoundID,
		RuntimeLeaseSessionKey: route.runtimeLeaseSessionKey,
		RuntimeLeaseRoundID:    route.runtimeLeaseRoundID,
		Code:                   code,
	}, controller, nil
}

func (t *channelAuthorizationTransport) buildCancellation(
	permission *permissionctx.Context,
	sender channelAuthorizationSender,
	flowID string,
	token string,
) (authorizationsvc.Actor, ChannelAuthorizationController, error) {
	route, authenticated, controller, err := t.resolveRoute(
		permission,
		sender,
		strings.TrimSpace(flowID),
		strings.TrimSpace(token),
	)
	if err != nil {
		return authorizationsvc.Actor{}, nil, err
	}
	return authorizationsvc.Actor{
		OwnerUserID:        route.principalUserID,
		AgentID:            route.agentID,
		SessionKey:         route.businessSessionKey,
		RoundID:            route.rootRoundID,
		LeaseSessionKey:    route.runtimeLeaseSessionKey,
		LeaseRoundID:       route.runtimeLeaseRoundID,
		ContextKind:        "agent",
		ContextID:          route.agentID,
		IsMainAgent:        true,
		PrincipalRole:      authenticated.principalRole,
		AuthMethod:         authenticated.authMethod,
		AuthSessionID:      authenticated.authSessionID,
		LocalSingleUser:    authenticated.localSingleUser,
		RoundLeaseRequired: false,
	}, controller, nil
}

func (t *channelAuthorizationTransport) resolveRoute(
	permission *permissionctx.Context,
	sender channelAuthorizationSender,
	flowID string,
	token string,
) (
	channelAuthorizationRoute,
	authenticatedChannelAuthorizationSender,
	ChannelAuthorizationController,
	error,
) {
	if t == nil || permission == nil || sender == nil || sender.IsClosed() ||
		strings.TrimSpace(flowID) == "" || strings.TrimSpace(token) == "" {
		return channelAuthorizationRoute{}, authenticatedChannelAuthorizationSender{}, nil, errors.New("invalid route")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.pruneLocked(now)
	authenticated, ok := t.senders[sender.Key()]
	if !ok || authenticated.sender == nil ||
		authenticated.principalUserID == "" {
		return channelAuthorizationRoute{}, authenticatedChannelAuthorizationSender{}, nil, errors.New("sender is not authenticated")
	}
	route, ok := t.routes[token]
	if !ok || route.flowID != flowID || route.presentationToken != token ||
		!now.Before(route.expiresAt) ||
		authenticated.principalUserID != route.principalUserID {
		return channelAuthorizationRoute{}, authenticatedChannelAuthorizationSender{}, nil, errors.New("presentation principal mismatch")
	}
	if authenticated.authMethod != route.principalAuthMethod ||
		authenticated.authSessionID != route.principalAuthSessionID {
		return channelAuthorizationRoute{}, authenticatedChannelAuthorizationSender{}, nil, errors.New("presentation route mismatch")
	}
	if _, ok = route.senderKeys[sender.Key()]; !ok ||
		!permission.IsBound(route.businessSessionKey, sender) {
		return channelAuthorizationRoute{}, authenticatedChannelAuthorizationSender{}, nil, errors.New("business session is no longer bound")
	}
	if t.controller == nil {
		return channelAuthorizationRoute{}, authenticatedChannelAuthorizationSender{}, nil, errors.New("controller is unavailable")
	}
	return route, authenticated, t.controller, nil
}

func (t *channelAuthorizationTransport) consumeRoute(token string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	delete(t.routes, strings.TrimSpace(token))
	t.mu.Unlock()
}

func (t *channelAuthorizationTransport) pruneLocked(now time.Time) {
	for token, route := range t.routes {
		if !now.Before(route.expiresAt) {
			delete(t.routes, token)
		}
	}
	for key, sender := range t.senders {
		if sender.sender == nil || sender.sender.IsClosed() {
			delete(t.senders, key)
		}
	}
}

func (h *Handler) sendChannelAuthorizationResult(
	ctx context.Context,
	sender channelAuthorizationSender,
	flowID string,
	accepted bool,
	status string,
	message string,
) {
	if sender == nil || sender.IsClosed() {
		return
	}
	_ = sender.SendEvent(ctx, protocol.NewChannelAuthorizationResultEvent(
		"",
		protocol.ChannelAuthorizationResultData{
			FlowID:   strings.TrimSpace(flowID),
			Accepted: accepted,
			Status:   strings.TrimSpace(status),
			Message:  strings.TrimSpace(message),
		},
	))
}
