// INPUT: 企业微信智能机器人凭据、长连接生命周期、ingress 与主动投递请求。
// OUTPUT: 可重连的机器人通道、订阅状态和有界写入/心跳配置。
// POS: 企业微信 WebSocket 适配器运行态根。
package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
)

type WeComBotChannel struct {
	botID       string
	secret      string
	baseURL     string
	ownerUserID string
	dialer      weComBotDialer
	logger      *slog.Logger

	mu             sync.RWMutex
	writeMu        sync.Mutex
	pendingMu      sync.Mutex
	ingress        channelcontract.IngressAcceptor
	conn           weComBotSocket
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	connected      bool
	subscribeReqID string
	lastPingReqID  string
	missedPongs    int
	pendingAcks    map[string]chan error
	ackTimeout     time.Duration
	pingInterval   time.Duration
	writeTimeout   time.Duration
	maxMissedPongs int
}

func NewWeComBotChannel(botID string, secret string) *WeComBotChannel {
	return &WeComBotChannel{
		botID:          strings.TrimSpace(botID),
		secret:         strings.TrimSpace(secret),
		baseURL:        weComBotDefaultLongConnectionURL,
		dialer:         gorillaWeComBotDialer{dialer: channeltransport.NewWebsocketDialer()},
		logger:         logx.NewDiscardLogger(),
		pendingAcks:    make(map[string]chan error),
		ackTimeout:     5 * time.Second,
		pingInterval:   30 * time.Second,
		writeTimeout:   10 * time.Second,
		maxMissedPongs: 3,
	}
}

func (c *WeComBotChannel) WithOwner(ownerUserID string) *WeComBotChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *WeComBotChannel) WithBaseURL(baseURL string) *WeComBotChannel {
	if baseURL = strings.TrimSpace(baseURL); baseURL != "" {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
	return c
}

func (c *WeComBotChannel) BaseURL() string {
	return c.baseURL
}

func (c *WeComBotChannel) ChannelType() string {
	return channelcontract.ChannelTypeWeChat
}

func (c *WeComBotChannel) SetIngress(ingress channelcontract.IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *WeComBotChannel) SetLogger(logger *slog.Logger) {
	if logger == nil {
		logger = logx.NewDiscardLogger()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logger = logger
}

func (c *WeComBotChannel) Start(ctx context.Context) error {
	if c.botID == "" || c.secret == "" {
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

func (c *WeComBotChannel) Stop(context.Context) error {
	c.mu.Lock()
	cancel := c.cancel
	conn := c.conn
	c.cancel = nil
	c.conn = nil
	c.connected = false
	c.subscribeReqID = ""
	c.lastPingReqID = ""
	c.missedPongs = 0
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

func (c *WeComBotChannel) SendDeliveryMessage(ctx context.Context, target channelcontract.DeliveryTarget, text string) (channelcontract.DeliveryResult, error) {
	normalized := target.Normalized()
	if normalized.To == "" {
		return channelcontract.NewDeliveryResult(normalized, nil), fmt.Errorf("wechat bot delivery target requires to")
	}
	if accountID := normalized.AccountID; accountID != "" && accountID != c.botID {
		return channelcontract.NewDeliveryResult(normalized, nil), fmt.Errorf("wechat bot delivery account does not match configured bot")
	}
	chunks := channeltransport.SplitText(strings.TrimSpace(text), 3800)
	if len(chunks) == 0 {
		return channelcontract.NewDeliveryResult(normalized, nil), nil
	}
	parts := make([]channelmessage.ReceiptPart, 0, len(chunks))
	replyReqID := normalized.ReplyContextID
	streamID := normalized.StreamID
	for index, chunk := range chunks {
		reqID := replyReqID
		var frame weComBotCommandFrame
		if reqID != "" && streamID != "" {
			frame = weComBotStreamResponseFrame(reqID, streamID, chunk, index == len(chunks)-1)
		} else {
			reqID = channelcontract.NewID("aibot_send")
			frame = weComBotProactiveMessageFrame(reqID, normalized.To, chunk)
		}
		if err := c.writeReplyFrame(ctx, reqID, frame); err != nil {
			return channelcontract.NewDeliveryResult(normalized, nil), err
		}
		parts = append(parts, channelmessage.TextPart(reqID))
	}
	return channelcontract.NewDeliveryResult(normalized, channelmessage.NewReceipt(channelmessage.ReceiptParams{
		Channel:  channelcontract.ChannelTypeWeChat,
		Target:   normalized.To,
		ThreadID: normalized.ThreadID,
		Parts:    parts,
	})), nil
}

func (c *WeComBotChannel) currentIngress() channelcontract.IngressAcceptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ingress
}

func (c *WeComBotChannel) loggerFor(context.Context) *slog.Logger {
	c.mu.RLock()
	logger := c.logger
	c.mu.RUnlock()
	if logger == nil {
		return logx.NewDiscardLogger()
	}
	return logger
}
