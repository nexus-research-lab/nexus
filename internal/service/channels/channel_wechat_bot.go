package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

const (
	weComBotDefaultLongConnectionURL = "wss://openws.work.weixin.qq.com"
	weComBotSubscribeCommand         = "aibot_subscribe"
	weComBotPingCommand              = "ping"
	weComBotPongCommand              = "pong"
	weComBotMessageCallbackCommand   = "aibot_msg_callback"
	weComBotEventCallbackCommand     = "aibot_event_callback"
	weComBotResponseCommand          = "aibot_respond_msg"
)

type weComBotChannel struct {
	botID       string
	secret      string
	baseURL     string
	ownerUserID string
	dialer      weComBotDialer
	logger      *slog.Logger

	mu             sync.RWMutex
	writeMu        sync.Mutex
	pendingMu      sync.Mutex
	ingress        IngressAcceptor
	conn           weComBotSocket
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	connected      bool
	subscribeReqID string
	lastPingReqID  string
	pendingAcks    map[string]chan error
	ackTimeout     time.Duration
}

type weComBotDialer interface {
	DialContext(context.Context, string, http.Header) (weComBotSocket, *http.Response, error)
}

type gorillaWeComBotDialer struct {
	dialer *websocket.Dialer
}

func (d gorillaWeComBotDialer) DialContext(ctx context.Context, endpoint string, header http.Header) (weComBotSocket, *http.Response, error) {
	return d.dialer.DialContext(ctx, endpoint, header)
}

type weComBotSocket interface {
	ReadMessage() (int, []byte, error)
	WriteJSON(any) error
	Close() error
}

type weComBotHeaders struct {
	ReqID       string `json:"req_id,omitempty"`
	ReqIDCompat string `json:"reqId,omitempty"`
}

func (h weComBotHeaders) requestID() string {
	return firstNonEmpty(h.ReqID, h.ReqIDCompat)
}

type weComBotCommandFrame struct {
	Cmd     string          `json:"cmd,omitempty"`
	Headers weComBotHeaders `json:"headers,omitempty"`
	Body    any             `json:"body,omitempty"`
}

type weComBotIncomingFrame struct {
	Cmd     string          `json:"cmd,omitempty"`
	Headers weComBotHeaders `json:"headers,omitempty"`
	Body    json.RawMessage `json:"body,omitempty"`
	ErrCode *int            `json:"errcode,omitempty"`
	ErrMsg  string          `json:"errmsg,omitempty"`
}

type weComBotParsedMessage struct {
	Kind       string
	MsgType    string
	MsgID      string
	FromUser   string
	SenderName string
	ChatType   string
	ChatID     string
	Content    string
	ReqID      string
}

func newWeComBotChannel(botID string, secret string) *weComBotChannel {
	return &weComBotChannel{
		botID:       strings.TrimSpace(botID),
		secret:      strings.TrimSpace(secret),
		baseURL:     weComBotDefaultLongConnectionURL,
		dialer:      gorillaWeComBotDialer{dialer: newChannelWebsocketDialer()},
		logger:      logx.NewDiscardLogger(),
		pendingAcks: make(map[string]chan error),
		ackTimeout:  5 * time.Second,
	}
}

func (c *weComBotChannel) WithOwner(ownerUserID string) *weComBotChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *weComBotChannel) ChannelType() string {
	return ChannelTypeWeChat
}

func (c *weComBotChannel) SetIngress(ingress IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *weComBotChannel) SetLogger(logger *slog.Logger) {
	if logger == nil {
		logger = logx.NewDiscardLogger()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logger = logger
}

func (c *weComBotChannel) Start(ctx context.Context) error {
	if strings.TrimSpace(c.botID) == "" || strings.TrimSpace(c.secret) == "" {
		return fmt.Errorf("wechat bot channel is not configured")
	}

	c.mu.Lock()
	if c.cancel != nil {
		c.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.wg.Add(1)
	c.mu.Unlock()

	go c.run(runCtx)
	return nil
}

func (c *weComBotChannel) Stop(context.Context) error {
	c.mu.Lock()
	cancel := c.cancel
	conn := c.conn
	c.cancel = nil
	c.conn = nil
	c.connected = false
	c.subscribeReqID = ""
	c.lastPingReqID = ""
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close()
	}
	c.clearPendingAcks(errors.New("wechat bot long connection stopped"))
	c.wg.Wait()
	return nil
}

func (c *weComBotChannel) SendDeliveryMessage(ctx context.Context, target DeliveryTarget, text string) (DeliveryResult, error) {
	normalized := target.Normalized()
	if strings.TrimSpace(target.To) == "" {
		return DeliveryResult{}, fmt.Errorf("wechat bot delivery target requires to")
	}
	reqID := strings.TrimSpace(target.AccountID)
	if reqID == "" {
		return DeliveryResult{}, fmt.Errorf("wechat bot delivery requires callback req_id")
	}
	streamID := firstNonEmpty(target.ThreadID, target.To)
	if streamID == "" {
		return DeliveryResult{}, fmt.Errorf("wechat bot delivery requires stream id")
	}

	chunks := splitText(strings.TrimSpace(text), 3800)
	if len(chunks) == 0 {
		return newDeliveryResult(normalized, nil), nil
	}
	for index, chunk := range chunks {
		frame := weComBotStreamResponseFrame(reqID, streamID, chunk, index == len(chunks)-1)
		if err := c.writeReplyFrame(ctx, reqID, frame); err != nil {
			return DeliveryResult{}, err
		}
	}
	return newDeliveryResult(normalized, nil), nil
}

func (c *weComBotChannel) run(ctx context.Context) {
	defer c.wg.Done()
	delay := time.Second
	for ctx.Err() == nil {
		err := c.connectAndServe(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.loggerFor(ctx).Warn("企业微信智能机器人长连接断开，准备重连", "err", err)
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return
		}
		if delay < 30*time.Second {
			delay *= 2
		}
	}
}

func (c *weComBotChannel) connectAndServe(ctx context.Context) error {
	endpoint := strings.TrimSpace(c.baseURL)
	if endpoint == "" {
		endpoint = weComBotDefaultLongConnectionURL
	}
	conn, response, err := c.dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		if response != nil {
			return fmt.Errorf("connect wechat bot long connection failed: status=%s err=%w", response.Status, err)
		}
		return err
	}
	c.setSocket(conn, false)
	defer c.clearSocket(conn)

	subscribeReqID := newDeliveryID("aibot_subscribe")
	c.setSubscribeReqID(subscribeReqID)
	if err = c.writeFrame(ctx, weComBotCommandFrame{
		Cmd:     weComBotSubscribeCommand,
		Headers: weComBotHeaders{ReqID: subscribeReqID},
		Body: map[string]string{
			"bot_id": c.botID,
			"secret": c.secret,
		},
	}, false); err != nil {
		return err
	}

	pingCtx, cancelPing := context.WithCancel(ctx)
	defer cancelPing()
	go c.pingLoop(pingCtx)

	for {
		_, payload, readErr := conn.ReadMessage()
		if readErr != nil {
			return readErr
		}
		if handleErr := c.handleFrame(ctx, payload); handleErr != nil {
			c.loggerFor(ctx).Warn("企业微信智能机器人长连接消息处理失败", "err", handleErr)
		}
	}
}

func (c *weComBotChannel) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			reqID := newDeliveryID("ping")
			c.setLastPingReqID(reqID)
			if err := c.writeFrame(ctx, weComBotCommandFrame{
				Cmd:     weComBotPingCommand,
				Headers: weComBotHeaders{ReqID: reqID},
			}, true); err != nil {
				c.loggerFor(ctx).Debug("企业微信智能机器人心跳发送失败", "err", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *weComBotChannel) handleFrame(ctx context.Context, raw []byte) error {
	var frame weComBotIncomingFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return err
	}
	cmd := strings.ToLower(strings.TrimSpace(frame.Cmd))
	reqID := weComBotFrameRequestID(frame)
	if cmd == weComBotPongCommand {
		return nil
	}
	if errCode, errMsg, ok := weComBotFrameStatus(frame, cmd); ok {
		c.handleStatusFrame(ctx, reqID, errCode, errMsg)
		return nil
	}
	if cmd != weComBotMessageCallbackCommand && cmd != weComBotEventCallbackCommand {
		return nil
	}

	parsed, ignored, err := parseWeComBotInboundMessage(frame.Body, reqID)
	if err != nil || ignored != "" || parsed.Kind != "message" {
		return err
	}
	ingress := c.currentIngress()
	if ingress == nil {
		return nil
	}
	request := c.ingressRequestFromParsed(parsed)
	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err = ingress.Accept(requestCtx, request); err != nil {
		if isPairingApprovalRequired(err) {
			if request.Delivery != nil {
				if notice := pairingApprovalNoticeText(err); notice != "" {
					if _, sendErr := c.SendDeliveryMessage(requestCtx, *request.Delivery, notice); sendErr != nil {
						c.loggerFor(ctx).Warn("企业微信配对提醒发送失败", "err", sendErr)
					}
				}
			}
			return nil
		}
		return err
	}
	return nil
}

func (c *weComBotChannel) handleStatusFrame(ctx context.Context, reqID string, errCode int, errMsg string) {
	if c.completePendingAck(reqID, errCode, errMsg) {
		return
	}
	if errCode == 0 && c.isSubscribeReqID(reqID) {
		c.setConnected(true)
		c.loggerFor(ctx).Debug("企业微信智能机器人长连接订阅成功")
		return
	}
	if errCode == 0 && c.isLastPingReqID(reqID) {
		return
	}
	if errCode != 0 {
		c.loggerFor(ctx).Warn("企业微信智能机器人长连接命令失败",
			"req_id", reqID,
			"errcode", errCode,
			"errmsg", strings.TrimSpace(errMsg),
		)
	}
}

func (c *weComBotChannel) ingressRequestFromParsed(parsed weComBotParsedMessage) IngressRequest {
	chatType := "dm"
	ref := parsed.FromUser
	if parsed.ChatType == "group" || parsed.ChatID != "" {
		chatType = "group"
		ref = firstNonEmpty(parsed.ChatID, parsed.FromUser)
	}
	streamID := newDeliveryID("stream")
	metadata := map[string]string{
		"req_id":    parsed.ReqID,
		"stream_id": streamID,
		"msg_type":  parsed.MsgType,
	}
	if parsed.ChatID != "" {
		metadata["chat_id"] = parsed.ChatID
	}
	return IngressRequest{
		Channel:      ChannelTypeWeChat,
		OwnerUserID:  c.ownerUserID,
		AccountID:    strings.TrimSpace(c.botID),
		ChatType:     chatType,
		Ref:          ref,
		ExternalName: firstNonEmpty(parsed.SenderName, parsed.FromUser, parsed.ChatID),
		Content:      parsed.Content,
		RoundID:      parsed.MsgID,
		ReqID:        parsed.MsgID,
		Delivery: &DeliveryTarget{
			Mode:      DeliveryModeExplicit,
			Channel:   ChannelTypeWeChat,
			To:        ref,
			AccountID: parsed.ReqID,
			ThreadID:  streamID,
		},
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           ChannelTypeWeChat,
			Target:            ref,
			PlatformMessageID: parsed.MsgID,
			SenderID:          parsed.FromUser,
			SenderName:        parsed.SenderName,
			ChatType:          chatType,
			Text:              parsed.Content,
			Metadata:          metadata,
		}),
	}
}

func (c *weComBotChannel) writeFrame(ctx context.Context, frame weComBotCommandFrame, requireConnected bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("wechat bot long connection is not connected")
	}
	if requireConnected && !connected {
		return fmt.Errorf("wechat bot long connection is not ready")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteJSON(frame)
}

func (c *weComBotChannel) writeReplyFrame(ctx context.Context, reqID string, frame weComBotCommandFrame) error {
	ackTimeout := c.currentAckTimeout()
	if ackTimeout <= 0 {
		return c.writeFrame(ctx, frame, true)
	}
	ack := c.registerPendingAck(reqID)
	if err := c.writeFrame(ctx, frame, true); err != nil {
		c.removePendingAck(reqID)
		return err
	}
	timer := time.NewTimer(ackTimeout)
	defer timer.Stop()
	select {
	case err := <-ack:
		return err
	case <-timer.C:
		c.removePendingAck(reqID)
		return fmt.Errorf("wechat bot reply ack timeout")
	case <-ctx.Done():
		c.removePendingAck(reqID)
		return ctx.Err()
	}
}

func (c *weComBotChannel) currentAckTimeout() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ackTimeout
}

func (c *weComBotChannel) registerPendingAck(reqID string) chan error {
	reqID = strings.TrimSpace(reqID)
	ack := make(chan error, 1)
	if reqID == "" {
		ack <- fmt.Errorf("wechat bot reply requires req_id")
		return ack
	}
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	c.pendingAcks[reqID] = ack
	return ack
}

func (c *weComBotChannel) removePendingAck(reqID string) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	delete(c.pendingAcks, strings.TrimSpace(reqID))
}

func (c *weComBotChannel) completePendingAck(reqID string, errCode int, errMsg string) bool {
	reqID = strings.TrimSpace(reqID)
	if reqID == "" {
		return false
	}
	c.pendingMu.Lock()
	ack := c.pendingAcks[reqID]
	delete(c.pendingAcks, reqID)
	c.pendingMu.Unlock()
	if ack == nil {
		return false
	}
	if errCode == 0 {
		ack <- nil
		return true
	}
	ack <- fmt.Errorf("wechat bot reply rejected: errcode=%d errmsg=%s", errCode, strings.TrimSpace(errMsg))
	return true
}

func (c *weComBotChannel) clearPendingAcks(err error) {
	if err == nil {
		err = errors.New("wechat bot long connection closed")
	}
	c.pendingMu.Lock()
	acks := c.pendingAcks
	c.pendingAcks = make(map[string]chan error)
	c.pendingMu.Unlock()
	for _, ack := range acks {
		ack <- err
	}
}

func (c *weComBotChannel) currentIngress() IngressAcceptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ingress
}

func (c *weComBotChannel) setSocket(conn weComBotSocket, connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn = conn
	c.connected = connected
}

func (c *weComBotChannel) clearSocket(conn weComBotSocket) {
	c.mu.Lock()
	if c.conn == conn {
		c.conn = nil
		c.connected = false
		c.subscribeReqID = ""
		c.lastPingReqID = ""
	}
	c.mu.Unlock()
	c.clearPendingAcks(errors.New("wechat bot long connection closed"))
	_ = conn.Close()
}

func (c *weComBotChannel) setConnected(connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = connected
}

func (c *weComBotChannel) setSubscribeReqID(reqID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscribeReqID = strings.TrimSpace(reqID)
}

func (c *weComBotChannel) isSubscribeReqID(reqID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return strings.TrimSpace(reqID) != "" && strings.TrimSpace(reqID) == c.subscribeReqID
}

func (c *weComBotChannel) setLastPingReqID(reqID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPingReqID = strings.TrimSpace(reqID)
}

func (c *weComBotChannel) isLastPingReqID(reqID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return strings.TrimSpace(reqID) != "" && strings.TrimSpace(reqID) == c.lastPingReqID
}

func (c *weComBotChannel) loggerFor(context.Context) *slog.Logger {
	c.mu.RLock()
	logger := c.logger
	c.mu.RUnlock()
	if logger == nil {
		return logx.NewDiscardLogger()
	}
	return logger
}

func weComBotStreamResponseFrame(reqID string, streamID string, content string, finish bool) weComBotCommandFrame {
	return weComBotCommandFrame{
		Cmd:     weComBotResponseCommand,
		Headers: weComBotHeaders{ReqID: strings.TrimSpace(reqID)},
		Body: map[string]any{
			"msgtype": "stream",
			"stream": map[string]any{
				"id":      strings.TrimSpace(streamID),
				"content": content,
				"finish":  finish,
			},
		},
	}
}

func weComBotFrameRequestID(frame weComBotIncomingFrame) string {
	if reqID := frame.Headers.requestID(); reqID != "" {
		return reqID
	}
	var body map[string]any
	if err := json.Unmarshal(frame.Body, &body); err != nil {
		return ""
	}
	return firstNonEmpty(
		weComBotStringAt(body, "req_id"),
		weComBotStringAt(body, "reqId"),
		weComBotStringAt(body, "request_id"),
		weComBotStringAt(body, "requestId"),
	)
}

func weComBotFrameStatus(frame weComBotIncomingFrame, cmd string) (int, string, bool) {
	if frame.ErrCode != nil {
		return *frame.ErrCode, strings.TrimSpace(frame.ErrMsg), true
	}
	if cmd == weComBotMessageCallbackCommand || cmd == weComBotEventCallbackCommand {
		return 0, "", false
	}
	var body map[string]any
	if err := json.Unmarshal(frame.Body, &body); err != nil {
		return 0, "", false
	}
	errCode, ok := weComBotIntAt(body, "errcode")
	if !ok {
		return 0, "", false
	}
	return errCode, firstNonEmpty(
		weComBotStringAt(body, "errmsg"),
		weComBotStringAt(body, "err_msg"),
		weComBotStringAt(body, "message"),
	), true
}

func parseWeComBotInboundMessage(raw json.RawMessage, reqID string) (weComBotParsedMessage, string, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return weComBotParsedMessage{}, "", err
	}
	source := weComBotMessageSource(payload)
	msgType := strings.ToLower(firstNonEmpty(
		weComBotStringAt(source, "msgtype"),
		weComBotStringAt(source, "msg_type"),
		weComBotStringAt(source, "msgType"),
		weComBotStringAt(source, "message_type"),
		weComBotStringAt(source, "messageType"),
		weComBotStringAt(source, "type"),
	))
	if msgType == "" && weComBotTextContent(source) != "" {
		msgType = "text"
	}
	if msgType == "event" {
		return weComBotParsedMessage{Kind: "event", MsgType: msgType}, "event", nil
	}
	if msgType == "" {
		return weComBotParsedMessage{}, "empty_msg_type", nil
	}
	if msgType == "stream" {
		return weComBotParsedMessage{Kind: "stream-refresh", MsgType: msgType}, "stream_refresh", nil
	}

	content := ""
	switch msgType {
	case "text":
		content = weComBotTextContent(source)
	case "mixed":
		content = weComBotMixedText(source)
	default:
		return weComBotParsedMessage{Kind: "unsupported", MsgType: msgType}, "unsupported_msg_type", nil
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return weComBotParsedMessage{}, "empty_text", nil
	}

	fromUser := firstNonEmpty(
		weComBotStringAt(source, "from", "userid"),
		weComBotStringAt(source, "from", "user_id"),
		weComBotStringAt(source, "from", "userId"),
		weComBotStringAt(source, "sender", "userid"),
		weComBotStringAt(source, "sender", "user_id"),
		weComBotStringAt(source, "sender", "userId"),
		weComBotStringAt(source, "sender", "id"),
		weComBotStringAt(source, "userid"),
		weComBotStringAt(source, "user_id"),
		weComBotStringAt(source, "userId"),
	)
	if fromUser == "" {
		return weComBotParsedMessage{}, "empty_from_user", nil
	}

	msgID := firstNonEmpty(
		weComBotStringAt(source, "msgid"),
		weComBotStringAt(source, "msg_id"),
		weComBotStringAt(source, "msgId"),
		weComBotStringAt(source, "message_id"),
		weComBotStringAt(source, "messageId"),
		weComBotStringAt(source, "id"),
	)
	if msgID == "" {
		msgID = newDeliveryID("wecom_msg")
	}
	return weComBotParsedMessage{
		Kind:     "message",
		MsgType:  msgType,
		MsgID:    msgID,
		FromUser: fromUser,
		SenderName: firstNonEmpty(
			weComBotStringAt(source, "sender", "name"),
			weComBotStringAt(source, "from", "name"),
			weComBotStringAt(source, "sender_name"),
			weComBotStringAt(source, "senderName"),
			weComBotStringAt(source, "nickname"),
		),
		ChatType: strings.ToLower(firstNonEmpty(
			weComBotStringAt(source, "chattype"),
			weComBotStringAt(source, "chat_type"),
			weComBotStringAt(source, "chatType"),
			"single",
		)),
		ChatID: firstNonEmpty(
			weComBotStringAt(source, "chatid"),
			weComBotStringAt(source, "chat_id"),
			weComBotStringAt(source, "chatId"),
			weComBotStringAt(source, "conversation_id"),
			weComBotStringAt(source, "conversationId"),
		),
		Content: content,
		ReqID:   firstNonEmpty(reqID, msgID),
	}, "", nil
}

func weComBotMessageSource(payload map[string]any) map[string]any {
	candidates := []map[string]any{
		payload,
		weComBotMapAt(payload, "message"),
		weComBotMapAt(payload, "msg"),
		weComBotMapAt(payload, "data"),
		weComBotMapAt(payload, "event", "message"),
	}
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if firstNonEmpty(
			weComBotStringAt(candidate, "msgtype"),
			weComBotStringAt(candidate, "msg_type"),
			weComBotStringAt(candidate, "msgType"),
			weComBotStringAt(candidate, "message_type"),
			weComBotStringAt(candidate, "messageType"),
			weComBotStringAt(candidate, "type"),
			weComBotTextContent(candidate),
		) != "" {
			return candidate
		}
	}
	return payload
}

func weComBotTextContent(source map[string]any) string {
	return firstNonEmpty(
		weComBotStringAt(source, "text", "content"),
		weComBotStringAt(source, "text", "text"),
		weComBotStringAt(source, "message", "text", "content"),
		weComBotStringAt(source, "content"),
		weComBotStringAt(source, "text_content"),
		weComBotStringAt(source, "textContent"),
	)
}

func weComBotMixedText(source map[string]any) string {
	items := firstNonEmptySlice(
		weComBotSliceAt(source, "mixed", "msg_item"),
		weComBotSliceAt(source, "mixed", "msgItem"),
		weComBotSliceAt(source, "mixed", "items"),
		weComBotSliceAt(source, "items"),
		weComBotSliceAt(source, "attachments"),
		weComBotSliceAt(source, "message", "items"),
		weComBotSliceAt(source, "message", "msg_item"),
	)
	parts := make([]string, 0, len(items))
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		itemType := strings.ToLower(firstNonEmpty(
			weComBotStringAt(itemMap, "msgtype"),
			weComBotStringAt(itemMap, "msg_type"),
			weComBotStringAt(itemMap, "msgType"),
			weComBotStringAt(itemMap, "type"),
		))
		if itemType != "" && itemType != "text" {
			continue
		}
		text := firstNonEmpty(
			weComBotStringAt(itemMap, "text", "content"),
			weComBotStringAt(itemMap, "content"),
			weComBotStringAt(itemMap, "text"),
		)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func weComBotMapAt(source map[string]any, path ...string) map[string]any {
	value := weComBotValueAt(source, path...)
	result, _ := value.(map[string]any)
	return result
}

func weComBotSliceAt(source map[string]any, path ...string) []any {
	value := weComBotValueAt(source, path...)
	result, _ := value.([]any)
	return result
}

func weComBotStringAt(source map[string]any, path ...string) string {
	value := weComBotValueAt(source, path...)
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func weComBotIntAt(source map[string]any, path ...string) (int, bool) {
	value := weComBotValueAt(source, path...)
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		result, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(result), true
	case string:
		result, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, false
		}
		return result, true
	default:
		return 0, false
	}
}

func weComBotValueAt(source map[string]any, path ...string) any {
	if source == nil || len(path) == 0 {
		return nil
	}
	var current any = source
	for _, segment := range path {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = currentMap[strings.TrimSpace(segment)]
	}
	return current
}

func firstNonEmptySlice(values ...[]any) []any {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}
