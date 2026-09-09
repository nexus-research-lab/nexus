// INPUT: 已认证 Nexus Principal、同源浏览器请求、消息正文与同步游标。
// OUTPUT: Nexus Team JSON envelope、WSS 水位或换代提示；内部以固定短令牌访问 Relay。
// POS: 浏览器到多人 Team 的唯一 HTTP/WSS gateway；不接受客户端身份或 Relay token。
package team

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	teamsvc "github.com/nexus-research-lab/nexus/internal/service/team"
)

const (
	// 64 KiB 正文在 JSON Unicode 转义的最坏情况下约为 384 KiB。
	maxRequestBodyBytes = 512 * 1024
	maxResourceIDBytes  = 128
	maxIdempotencyBytes = 128
	maxPageSize         = 100
)

type relayTokenExchanger interface {
	ExchangeRelayUserToken(context.Context, *authsvc.Principal) (string, error)
}

type relayStream interface {
	Watch(context.Context, string, string, string, func(relaycontract.StreamUpdated) error) error
}

type streamResetRequired struct {
	Type     string `json:"type"`
	StreamID string `json:"stream_id"`
	Reason   string `json:"reason"`
}

// Handlers 封装 Team gateway 的 HTTP 边界。
type Handlers struct {
	api    *handlershared.API
	tokens relayTokenExchanger
	relay  relayStream
	team   *teamsvc.Service
}

// New 创建 Team gateway handlers。
func New(
	api *handlershared.API,
	tokens relayTokenExchanger,
	service *teamsvc.Service,
	relay relayStream,
) *Handlers {
	return &Handlers{api: api, tokens: tokens, relay: relay, team: service}
}

// HandleStream 把 Relay 的提交水位提示转发给同源浏览器；消息正文仍由 Difference 获取。
func (h *Handlers) HandleStream(writer http.ResponseWriter, request *http.Request) {
	h.noStore(writer)
	if !h.requireMutationOrigin(writer, request) {
		return
	}
	streamID := strings.TrimSpace(request.URL.Query().Get("stream_id"))
	streamEpoch := strings.TrimSpace(request.URL.Query().Get("stream_epoch"))
	if !validResourceID(streamID) || !validResourceID(streamEpoch) {
		h.writeRequestError(writer, request, "team.stream_invalid", "同步流参数无效", false)
		return
	}
	token, ok := h.exchangeToken(writer, request, false)
	if !ok {
		return
	}
	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		// Origin 已按外部 scheme 与 Host 完整校验。
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer connection.CloseNow()
	connection.SetReadLimit(1024)
	ctx := connection.CloseRead(request.Context())
	err = h.relay.Watch(ctx, token, streamID, streamEpoch, func(update relaycontract.StreamUpdated) error {
		writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return wsjson.Write(writeCtx, connection, update)
	})
	if ctx.Err() != nil {
		return
	}
	var remote *relaycontract.RemoteError
	if errors.As(err, &remote) && remote.Code == "full_snapshot_required" {
		writeCtx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
		defer cancel()
		if writeErr := wsjson.Write(writeCtx, connection, streamResetRequired{
			Type: "stream.reset_required", StreamID: streamID, Reason: remote.Code,
		}); writeErr != nil {
			h.api.BaseLogger().Warn("Team WSS 换代提示失败", "stream_id", streamID, "err", writeErr)
			return
		}
		_ = connection.Close(websocket.StatusPolicyViolation, "team stream reset required")
		return
	}
	h.api.BaseLogger().Warn("Team WSS 中断", "stream_id", streamID, "err", err)
	_ = connection.Close(websocket.StatusInternalError, "team stream interrupted")
}

// HandleBootstrap 获取当前登录用户的默认 Team 空间。
func (h *Handlers) HandleBootstrap(writer http.ResponseWriter, request *http.Request) {
	h.noStore(writer)
	if !h.requireMutationOrigin(writer, request) || !h.requireEmptyBody(writer, request) {
		return
	}
	token, ok := h.exchangeToken(writer, request, true)
	if !ok {
		return
	}
	result, err := h.team.Bootstrap(request.Context(), teamAccess(request, token))
	if err != nil {
		h.writeRelayError(writer, request, err, true)
		return
	}
	h.api.WriteSuccess(writer, result)
}

func teamAccess(request *http.Request, token string) teamsvc.Access {
	principal := authsvc.PrincipalFromContext(request.Context())
	return teamsvc.Access{OwnerUserID: principal.UserID, DeploymentID: principal.DeploymentID, Token: token}
}

// HandlePostMessage 向 Team Conversation 幂等提交一条真人消息。
func (h *Handlers) HandlePostMessage(writer http.ResponseWriter, request *http.Request) {
	h.noStore(writer)
	if !h.requireMutationOrigin(writer, request) {
		return
	}
	conversationID := strings.TrimSpace(chi.URLParam(request, "conversation_id"))
	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if !validResourceID(conversationID) || !validIdempotencyKey(idempotencyKey) {
		h.writeRequestError(writer, request, "team.message_invalid", "消息请求参数无效", true)
		return
	}
	var input relaycontract.CreateMessageInput
	if err := decodeStrictJSON(writer, request, &input); err != nil {
		h.writeRequestError(writer, request, "team.message_invalid", "消息正文无效", true)
		return
	}
	token, ok := h.exchangeToken(writer, request, true)
	if !ok {
		return
	}
	result, err := h.team.PostMessage(
		request.Context(), teamAccess(request, token), conversationID, idempotencyKey, input,
	)
	if err != nil {
		h.writeRelayError(writer, request, err, true)
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleSnapshot 读取一页固定上界的 Team Conversation 快照。
func (h *Handlers) HandleSnapshot(writer http.ResponseWriter, request *http.Request) {
	h.noStore(writer)
	conversationID := strings.TrimSpace(chi.URLParam(request, "conversation_id"))
	options, err := snapshotOptions(request.URL.Query())
	if !validResourceID(conversationID) || err != nil {
		h.writeRequestError(writer, request, "team.snapshot_invalid", "快照请求参数无效", false)
		return
	}
	token, ok := h.exchangeToken(writer, request, false)
	if !ok {
		return
	}
	result, err := h.team.Snapshot(request.Context(), teamAccess(request, token), conversationID, options)
	if err != nil {
		h.writeRelayError(writer, request, err, false)
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleDifference 读取一页连续 Team sync event。
func (h *Handlers) HandleDifference(writer http.ResponseWriter, request *http.Request) {
	h.noStore(writer)
	streamID := strings.TrimSpace(chi.URLParam(request, "stream_id"))
	options, err := differenceOptions(request.URL.Query())
	if !validResourceID(streamID) || err != nil {
		h.writeRequestError(writer, request, "team.difference_invalid", "增量请求参数无效", false)
		return
	}
	token, ok := h.exchangeToken(writer, request, false)
	if !ok {
		return
	}
	result, err := h.team.Difference(request.Context(), teamAccess(request, token), streamID, options)
	if err != nil {
		h.writeRelayError(writer, request, err, false)
		return
	}
	h.api.WriteSuccess(writer, result)
}

func (h *Handlers) writeProjectionError(
	writer http.ResponseWriter,
	request *http.Request,
	err error,
	mutation bool,
) {
	h.api.BaseLogger().Error("Team 本地投影失败", "err", err)
	h.api.WriteError(writer, request, http.StatusServiceUnavailable, handlershared.FailureSpec{
		Code:     "team.local_projection_failed",
		Category: protocol.FailureCategoryUnavailable,
		Effect:   requestEffect(mutation, false),
		Detail:   "团队消息正在同步，请稍后重试",
		Cause:    err,
	})
}

func (h *Handlers) noStore(writer http.ResponseWriter) {
	writer.Header().Set("Cache-Control", "no-store")
}

func (h *Handlers) requireMutationOrigin(writer http.ResponseWriter, request *http.Request) bool {
	if sameOrigin(request) {
		return true
	}
	h.api.WriteError(writer, request, http.StatusForbidden, handlershared.FailureSpec{
		Code:     "team.origin_invalid",
		Category: protocol.FailureCategoryAuthorization,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "请求来源无效",
	})
	return false
}

func (h *Handlers) requireEmptyBody(writer http.ResponseWriter, request *http.Request) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, maxRequestBodyBytes))
	var value any
	if err := decoder.Decode(&value); errors.Is(err, io.EOF) {
		return true
	}
	h.writeRequestError(writer, request, "team.bootstrap_invalid", "Bootstrap 请求不能包含正文", true)
	return false
}

func (h *Handlers) exchangeToken(
	writer http.ResponseWriter,
	request *http.Request,
	mutation bool,
) (string, bool) {
	principal := authsvc.PrincipalFromContext(request.Context())
	if principal == nil {
		h.api.WriteError(writer, request, http.StatusUnauthorized, handlershared.FailureSpec{
			Code:     "auth.authentication_required",
			Category: protocol.FailureCategoryAuthentication,
			Effect:   requestEffect(mutation, true),
			Detail:   "未登录或登录状态已过期",
		})
		return "", false
	}
	token, err := h.tokens.ExchangeRelayUserToken(request.Context(), principal)
	if err != nil {
		h.api.WriteError(writer, request, http.StatusBadGateway, handlershared.FailureSpec{
			Code:     "team.identity_exchange_failed",
			Category: protocol.FailureCategoryUnavailable,
			Effect:   requestEffect(mutation, true),
			Detail:   "团队身份暂时无法验证",
			Cause:    err,
		})
		return "", false
	}
	return token, true
}

func (h *Handlers) writeRequestError(
	writer http.ResponseWriter,
	request *http.Request,
	code string,
	detail string,
	mutation bool,
) {
	h.api.WriteError(writer, request, http.StatusBadRequest, handlershared.FailureSpec{
		Code:     code,
		Category: protocol.FailureCategoryValidation,
		Effect:   requestEffect(mutation, true),
		Detail:   detail,
	})
}

func (h *Handlers) writeRelayError(
	writer http.ResponseWriter,
	request *http.Request,
	err error,
	mutation bool,
) {
	if errors.Is(err, teamsvc.ErrProjection) {
		h.writeProjectionError(writer, request, err, mutation)
		return
	}
	status, failure := relayFailure(err, mutation)
	h.api.WriteError(writer, request, status, failure)
}

func relayFailure(err error, mutation bool) (int, handlershared.FailureSpec) {
	spec := handlershared.FailureSpec{
		Code:     "team.upstream_failed",
		Category: protocol.FailureCategoryUnavailable,
		Effect:   requestEffect(mutation, false),
		Detail:   "团队服务暂时不可用",
		Cause:    err,
	}
	var remote *relaycontract.RemoteError
	if !errors.As(err, &remote) {
		return http.StatusBadGateway, spec
	}

	switch remote.Code {
	case "request_invalid":
		spec.Code = "team.request_invalid"
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = requestEffect(mutation, true)
		spec.Detail = "团队请求无效"
		return http.StatusBadRequest, spec
	case "message_too_large":
		spec.Code = "team.message_too_large"
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = requestEffect(mutation, true)
		spec.Detail = "消息正文超过大小限制"
		return http.StatusRequestEntityTooLarge, spec
	case "idempotency_key_required":
		spec.Code = "team.idempotency_key_required"
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = requestEffect(mutation, true)
		spec.Detail = "消息请求缺少有效的幂等标识"
		return http.StatusBadRequest, spec
	case "resource_not_found":
		spec.Code = "team.resource_not_found"
		spec.Category = protocol.FailureCategoryNotFound
		spec.Effect = requestEffect(mutation, true)
		spec.Detail = "团队资源不存在"
		return http.StatusNotFound, spec
	case "idempotency_conflict":
		spec.Code = "team.idempotency_conflict"
		spec.Category = protocol.FailureCategoryConflict
		spec.Effect = requestEffect(mutation, true)
		spec.Detail = "幂等标识已用于不同的消息"
		return http.StatusConflict, spec
	case "cursor_ahead":
		spec.Code = "team.cursor_ahead"
		spec.Category = protocol.FailureCategoryConflict
		spec.Detail = "同步位置超过服务端水位"
		return http.StatusConflict, spec
	case "full_snapshot_required":
		spec.Code = "team.full_snapshot_required"
		spec.Category = protocol.FailureCategoryConflict
		spec.Detail = "增量历史已裁剪，请重新获取快照"
		return http.StatusConflict, spec
	case "principal_invalid":
		spec.Code = "team.identity_rejected"
		spec.Effect = requestEffect(mutation, true)
		spec.Detail = "团队身份暂时无法验证"
		return http.StatusBadGateway, spec
	case "relay_unavailable":
		spec.Code = "team.unavailable"
		return http.StatusServiceUnavailable, spec
	default:
		return http.StatusBadGateway, spec
	}
}

func requestEffect(mutation bool, notApplied bool) protocol.FailureEffect {
	if !mutation {
		return protocol.FailureEffectNotApplicable
	}
	if notApplied {
		return protocol.FailureEffectNotApplied
	}
	return protocol.FailureEffectUnknown
}

func sameOrigin(request *http.Request) bool {
	if request == nil {
		return false
	}
	origin, err := url.Parse(strings.TrimSpace(request.Header.Get("Origin")))
	if err != nil || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" ||
		(origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" ||
		(origin.Path != "" && origin.Path != "/") {
		return false
	}
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-Proto"), ",")[0]); forwarded != "" {
		scheme = strings.ToLower(forwarded)
	}
	return strings.EqualFold(origin.Scheme, scheme) && strings.EqualFold(origin.Host, request.Host)
}

func decodeStrictJSON(writer http.ResponseWriter, request *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, maxRequestBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("请求正文只能包含一个 JSON 值")
	}
	return nil
}

func snapshotOptions(query url.Values) (relaycontract.SnapshotOptions, error) {
	after, err := queryInt64(query, "after_message_seq", 0)
	if err != nil {
		return relaycontract.SnapshotOptions{}, err
	}
	limit, err := queryLimit(query)
	if err != nil {
		return relaycontract.SnapshotOptions{}, err
	}
	result := relaycontract.SnapshotOptions{
		AfterMessageSeq: after,
		Limit:           limit,
		StreamEpoch:     strings.TrimSpace(query.Get("stream_epoch")),
	}
	if result.StreamEpoch != "" && !validResourceID(result.StreamEpoch) {
		return relaycontract.SnapshotOptions{}, errors.New("stream epoch 无效")
	}
	throughRaw := strings.TrimSpace(query.Get("through_message_seq"))
	snapshotRaw := strings.TrimSpace(query.Get("snapshot_seq"))
	if (throughRaw == "") != (snapshotRaw == "") {
		return relaycontract.SnapshotOptions{}, errors.New("续页游标不完整")
	}
	if throughRaw == "" {
		if after > 0 && result.StreamEpoch == "" {
			return relaycontract.SnapshotOptions{}, errors.New("续页缺少 stream epoch")
		}
		return result, nil
	}
	through, err := parseNonnegativeInt64(throughRaw)
	if err != nil {
		return relaycontract.SnapshotOptions{}, err
	}
	snapshot, err := parseNonnegativeInt64(snapshotRaw)
	if err != nil {
		return relaycontract.SnapshotOptions{}, err
	}
	result.ThroughMessageSeq = &through
	result.SnapshotSeq = &snapshot
	if result.StreamEpoch == "" {
		return relaycontract.SnapshotOptions{}, errors.New("续页缺少 stream epoch")
	}
	return result, nil
}

func differenceOptions(query url.Values) (relaycontract.DifferenceOptions, error) {
	after, err := queryInt64(query, "after_seq", 0)
	if err != nil {
		return relaycontract.DifferenceOptions{}, err
	}
	limit, err := queryLimit(query)
	if err != nil {
		return relaycontract.DifferenceOptions{}, err
	}
	streamEpoch := strings.TrimSpace(query.Get("stream_epoch"))
	if streamEpoch != "" && !validResourceID(streamEpoch) {
		return relaycontract.DifferenceOptions{}, errors.New("stream epoch 无效")
	}
	if after > 0 && streamEpoch == "" {
		return relaycontract.DifferenceOptions{}, errors.New("增量请求缺少 stream epoch")
	}
	return relaycontract.DifferenceOptions{
		AfterSeq: after, Limit: limit, StreamEpoch: streamEpoch,
	}, nil
}

func queryInt64(query url.Values, name string, fallback int64) (int64, error) {
	raw := strings.TrimSpace(query.Get(name))
	if raw == "" {
		return fallback, nil
	}
	return parseNonnegativeInt64(raw)
}

func parseNonnegativeInt64(raw string) (int64, error) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, errors.New("游标无效")
	}
	return value, nil
}

func queryLimit(query url.Values) (int, error) {
	raw := strings.TrimSpace(query.Get("limit"))
	if raw == "" {
		return maxPageSize, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 || value > maxPageSize {
		return 0, errors.New("分页大小无效")
	}
	return value, nil
}

func validResourceID(value string) bool {
	return value != "" && len(value) <= maxResourceIDBytes
}

func validIdempotencyKey(value string) bool {
	if value == "" || len(value) > maxIdempotencyBytes {
		return false
	}
	for _, char := range []byte(value) {
		if char < 0x21 || char > 0x7e {
			return false
		}
	}
	return true
}
