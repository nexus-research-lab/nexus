// INPUT: WebSocket 请求、固定 Nexus 扩展 Origin、browser.result 回执与 browser.event 生命周期事件。
// OUTPUT: 已认证的 Browser 连接注册、协议不兼容诊断、回执投递及扩展事件转交。
// POS: Browser transport 信任边界；不解释浏览器动作或保存 Session 状态。
package browser

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	browsersvc "github.com/nexus-research-lab/nexus/internal/service/browser"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	readyTimeout = 5 * time.Second
	writeTimeout = 10 * time.Second
	pingEvery    = 20 * time.Second
	readLimit    = 64 << 20
)

type wireMessage struct {
	Type              string         `json:"type"`
	ID                string         `json:"id,omitempty"`
	ProtocolVersion   string         `json:"protocol_version,omitempty"`
	ExtensionVersion  string         `json:"extension_version,omitempty"`
	BrowserName       string         `json:"browser_name,omitempty"`
	BrowserInstance   string         `json:"browser_instance_id,omitempty"`
	BrowserGeneration string         `json:"browser_generation,omitempty"`
	Event             string         `json:"event,omitempty"`
	Data              map[string]any `json:"data,omitempty"`
	Result            map[string]any `json:"result,omitempty"`
	Error             string         `json:"error,omitempty"`
}

// Handler 处理 Nexus 浏览器扩展连接。
type Handler struct {
	api     *handlershared.API
	service *browsersvc.Service
}

// New 创建 Browser handler。
func New(api *handlershared.API, service *browsersvc.Service) *Handler {
	if api == nil || service == nil {
		return nil
	}
	return &Handler{api: api, service: service}
}

// HandleStatus 返回扩展连接状态，不包含浏览数据。
func (h *Handler) HandleStatus(writer http.ResponseWriter, request *http.Request) {
	if h == nil || h.api == nil || h.service == nil {
		http.NotFound(writer, request)
		return
	}
	h.api.WriteSuccess(writer, h.service.Status())
}

// HandleWebSocket 校验固定扩展身份并处理连接生命周期。
func (h *Handler) HandleWebSocket(writer http.ResponseWriter, request *http.Request) {
	if h == nil || h.service == nil {
		http.NotFound(writer, request)
		return
	}
	if !trustedRequest(request) {
		http.Error(writer, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		// Origin 已在 trustedRequest 中按完整 scheme 和扩展 ID 精确校验。
		InsecureSkipVerify: true,
		Subprotocols:       []string{browsersvc.WebSocketSubprotocol},
	})
	if err != nil {
		return
	}
	connection.SetReadLimit(readLimit)
	defer connection.CloseNow()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	readyCtx, readyCancel := context.WithTimeout(ctx, readyTimeout)
	var ready wireMessage
	err = wsjson.Read(readyCtx, connection, &ready)
	readyCancel()
	if err == nil && ready.Type == "browser.ready" && ready.ProtocolVersion != browsersvc.ProtocolVersion {
		h.service.ObserveIncompatibleHandshake(ready.ExtensionVersion, ready.ProtocolVersion)
	}
	if err != nil || ready.Type != "browser.ready" ||
		ready.ProtocolVersion != browsersvc.ProtocolVersion ||
		strings.TrimSpace(ready.BrowserInstance) == "" ||
		strings.TrimSpace(ready.BrowserGeneration) == "" {
		_ = connection.Close(websocket.StatusPolicyViolation, "invalid Browser handshake")
		return
	}

	var writeMu sync.Mutex
	send := func(parent context.Context, payload any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		writeCtx, writeCancel := context.WithTimeout(parent, writeTimeout)
		defer writeCancel()
		return wsjson.Write(writeCtx, connection, payload)
	}
	clientID, detach := h.service.Attach(
		ready.ExtensionVersion,
		ready.BrowserName,
		ready.BrowserInstance,
		ready.BrowserGeneration,
		send,
		func() { _ = connection.CloseNow() },
	)
	if clientID == 0 {
		return
	}
	defer detach()
	if err = send(ctx, map[string]any{
		"type":             "browser.accepted",
		"protocol_version": browsersvc.ProtocolVersion,
	}); err != nil {
		return
	}
	go keepAlive(ctx, cancel, connection, send)

	for {
		var inbound wireMessage
		if err = wsjson.Read(ctx, connection, &inbound); err != nil {
			return
		}
		switch inbound.Type {
		case "browser.result":
			h.service.Resolve(inbound.ID, inbound.Result, inbound.Error)
		case "browser.event":
			h.service.ObserveEvent(inbound.Event, inbound.Data)
		}
	}
}

func keepAlive(
	ctx context.Context,
	cancel context.CancelFunc,
	connection *websocket.Conn,
	send func(context.Context, any) error,
) {
	ticker := time.NewTicker(pingEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := send(ctx, map[string]any{"type": "browser.ping"}); err != nil {
				cancel()
				_ = connection.CloseNow()
				return
			}
		}
	}
}

func trustedRequest(request *http.Request) bool {
	if request == nil || request.Method != http.MethodGet {
		return false
	}
	origin, err := url.Parse(strings.TrimSpace(request.Header.Get("Origin")))
	if err != nil || origin.Scheme != "chrome-extension" ||
		origin.Host != browsersvc.BrowserExtensionID ||
		(origin.Path != "" && origin.Path != "/") ||
		origin.RawQuery != "" || origin.User != nil {
		return false
	}
	for _, item := range strings.Split(request.Header.Get("Sec-WebSocket-Protocol"), ",") {
		if strings.TrimSpace(item) == browsersvc.WebSocketSubprotocol {
			return true
		}
	}
	return false
}
