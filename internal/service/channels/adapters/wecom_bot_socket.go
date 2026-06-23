package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
)

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

func (c *WeComBotChannel) run(ctx context.Context) {
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

func (c *WeComBotChannel) connectAndServe(ctx context.Context) error {
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

	subscribeReqID := channelcontract.NewID("aibot_subscribe")
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

func (c *WeComBotChannel) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			reqID := channelcontract.NewID("ping")
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

func (c *WeComBotChannel) handleFrame(ctx context.Context, raw []byte) error {
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
		if IsPairingApprovalRequired(err) {
			if request.Delivery != nil {
				if notice := PairingApprovalNoticeText(err); notice != "" {
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

func (c *WeComBotChannel) handleStatusFrame(ctx context.Context, reqID string, errCode int, errMsg string) {
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

func (c *WeComBotChannel) writeFrame(ctx context.Context, frame weComBotCommandFrame, requireConnected bool) error {
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

func (c *WeComBotChannel) writeReplyFrame(ctx context.Context, reqID string, frame weComBotCommandFrame) error {
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

func (c *WeComBotChannel) currentAckTimeout() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ackTimeout
}

func (c *WeComBotChannel) registerPendingAck(reqID string) chan error {
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

func (c *WeComBotChannel) removePendingAck(reqID string) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	delete(c.pendingAcks, strings.TrimSpace(reqID))
}

func (c *WeComBotChannel) completePendingAck(reqID string, errCode int, errMsg string) bool {
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

func (c *WeComBotChannel) clearPendingAcks(err error) {
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

func (c *WeComBotChannel) setSocket(conn weComBotSocket, connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn = conn
	c.connected = connected
}

func (c *WeComBotChannel) clearSocket(conn weComBotSocket) {
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

func (c *WeComBotChannel) setConnected(connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = connected
}

func (c *WeComBotChannel) setSubscribeReqID(reqID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscribeReqID = strings.TrimSpace(reqID)
}

func (c *WeComBotChannel) isSubscribeReqID(reqID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	reqID = strings.TrimSpace(reqID)
	return reqID != "" && reqID == c.subscribeReqID
}

func (c *WeComBotChannel) setLastPingReqID(reqID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPingReqID = strings.TrimSpace(reqID)
}

func (c *WeComBotChannel) isLastPingReqID(reqID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	reqID = strings.TrimSpace(reqID)
	return reqID != "" && reqID == c.lastPingReqID
}
