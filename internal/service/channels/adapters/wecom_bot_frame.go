package adapters

import (
	"encoding/json"
	"strings"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
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

type weComBotHeaders struct {
	ReqID       string `json:"req_id,omitempty"`
	ReqIDCompat string `json:"reqId,omitempty"`
}

func (h weComBotHeaders) requestID() string {
	return channelcontract.FirstNonEmpty(h.ReqID, h.ReqIDCompat)
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
	return channelcontract.FirstNonEmpty(
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
	return errCode, channelcontract.FirstNonEmpty(
		weComBotStringAt(body, "errmsg"),
		weComBotStringAt(body, "err_msg"),
		weComBotStringAt(body, "message"),
	), true
}
