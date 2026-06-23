package adapters

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
)

type PersonalWeixinChannel struct {
	token       string
	accountID   string
	userID      string
	ownerUserID string
	client      *PersonalWeixinIlinkClient

	mu      sync.RWMutex
	ingress channelcontract.IngressAcceptor
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewPersonalWeixinChannel(config PersonalWeixinClientConfig, client *http.Client) *PersonalWeixinChannel {
	ilinkClient := NewPersonalWeixinIlinkClient(config, client)
	return &PersonalWeixinChannel{
		token:     strings.TrimSpace(config.Token),
		accountID: strings.TrimSpace(config.AccountID),
		userID:    strings.TrimSpace(config.UserID),
		client:    ilinkClient,
	}
}

func (c *PersonalWeixinChannel) WithOwner(ownerUserID string) *PersonalWeixinChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *PersonalWeixinChannel) ChannelType() string {
	return channelcontract.ChannelTypeWeixinPersonal
}

func (c *PersonalWeixinChannel) SetIngress(ingress channelcontract.IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *PersonalWeixinChannel) Start(ctx context.Context) error {
	if strings.TrimSpace(c.token) == "" {
		return nil
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

	go c.pollUpdates(runCtx)
	return nil
}

func (c *PersonalWeixinChannel) Stop(context.Context) error {
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.wg.Wait()
	return nil
}

func (c *PersonalWeixinChannel) SendDeliveryMessage(ctx context.Context, target channelcontract.DeliveryTarget, text string) (channelcontract.DeliveryResult, error) {
	normalized := target.Normalized()
	if strings.TrimSpace(c.token) == "" {
		return channelcontract.DeliveryResult{}, fmt.Errorf("personal weixin channel is not configured")
	}
	if strings.TrimSpace(target.To) == "" {
		return channelcontract.DeliveryResult{}, fmt.Errorf("personal weixin delivery target requires to")
	}
	parts := make([]channelmessage.ReceiptPart, 0)
	for _, chunk := range channeltransport.SplitText(strings.TrimSpace(text), 4000) {
		clientID := channelcontract.NewID("weixin")
		request := map[string]any{
			"base_info": c.client.baseInfo(),
			"msg": personalWeixinMessage{
				FromUserID:   "",
				ToUserID:     strings.TrimSpace(target.To),
				ClientID:     clientID,
				MessageType:  personalWeixinMessageTypeBot,
				MessageState: personalWeixinMessageStateEnd,
				ContextToken: strings.TrimSpace(target.ThreadID),
				ItemList: []personalWeixinMessageItem{{
					Type: personalWeixinItemTypeText,
					TextItem: personalWeixinTextItem{
						Text: chunk,
					},
				}},
			},
		}
		if err := c.client.post(ctx, "ilink/bot/sendmessage", request, nil); err != nil {
			return channelcontract.DeliveryResult{}, err
		}
		parts = append(parts, channelmessage.TextPart(clientID))
	}
	return channelcontract.NewDeliveryResult(normalized, channelmessage.NewReceipt(channelmessage.ReceiptParams{
		Channel:  channelcontract.ChannelTypeWeixinPersonal,
		Target:   target.To,
		ThreadID: target.ThreadID,
		Parts:    parts,
	})), nil
}

func (c *PersonalWeixinChannel) SendDeliveryTyping(ctx context.Context, target channelcontract.DeliveryTarget, active bool) error {
	if strings.TrimSpace(c.token) == "" {
		return fmt.Errorf("personal weixin channel is not configured")
	}
	to := strings.TrimSpace(target.To)
	if to == "" {
		return fmt.Errorf("personal weixin typing target requires to")
	}
	ticket, err := c.client.TypingTicket(ctx, to, strings.TrimSpace(target.ThreadID))
	if err != nil {
		return err
	}
	if strings.TrimSpace(ticket) == "" {
		return nil
	}
	return c.client.SendTyping(ctx, to, ticket, active)
}

func (c *PersonalWeixinChannel) pollUpdates(ctx context.Context) {
	defer c.wg.Done()
	getUpdatesBuf := ""
	nextTimeout := 35 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		response, err := c.client.GetUpdates(ctx, getUpdatesBuf, nextTimeout)
		if err != nil {
			if waitPersonalWeixinRetry(ctx, 2*time.Second) {
				continue
			}
			return
		}
		if response.LongPollingTimeoutMS > 0 {
			nextTimeout = time.Duration(response.LongPollingTimeoutMS) * time.Millisecond
		}
		if response.Ret != 0 || response.ErrCode != 0 {
			if waitPersonalWeixinRetry(ctx, 5*time.Second) {
				continue
			}
			return
		}
		if strings.TrimSpace(response.GetUpdatesBuf) != "" {
			getUpdatesBuf = response.GetUpdatesBuf
		}
		for _, message := range response.Messages {
			c.handleMessage(ctx, message)
		}
	}
}

func (c *PersonalWeixinChannel) handleMessage(ctx context.Context, message personalWeixinMessage) {
	if message.MessageType == personalWeixinMessageTypeBot {
		return
	}
	fromUserID := strings.TrimSpace(message.FromUserID)
	if fromUserID == "" {
		return
	}
	content := personalWeixinTextContent(message)
	if content == "" {
		return
	}
	ingress := c.currentIngress()
	if ingress == nil {
		return
	}
	delivery := &channelcontract.DeliveryTarget{
		Mode:      channelcontract.DeliveryModeExplicit,
		Channel:   channelcontract.ChannelTypeWeixinPersonal,
		To:        fromUserID,
		AccountID: c.accountID,
		ThreadID:  strings.TrimSpace(message.ContextToken),
	}
	messageID := personalWeixinInboundMessageID(message)
	receivedAt := time.Time{}
	if message.CreateTimeMS > 0 {
		receivedAt = time.UnixMilli(message.CreateTimeMS).UTC()
	}
	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := ingress.Accept(requestCtx, channelcontract.IngressRequest{
		Channel:      channelcontract.ChannelTypeWeixinPersonal,
		OwnerUserID:  c.ownerUserID,
		AccountID:    c.accountID,
		ChatType:     "dm",
		Ref:          fromUserID,
		ExternalName: fromUserID,
		Content:      content,
		RoundID:      messageID,
		ReqID:        messageID,
		Delivery:     delivery,
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           channelcontract.ChannelTypeWeixinPersonal,
			Target:            fromUserID,
			PlatformMessageID: messageID,
			ThreadID:          strings.TrimSpace(message.ContextToken),
			SenderID:          fromUserID,
			SenderName:        fromUserID,
			ChatType:          "dm",
			Text:              content,
			ReceivedAt:        receivedAt,
			Metadata: map[string]string{
				"client_id":  strings.TrimSpace(message.ClientID),
				"session_id": strings.TrimSpace(message.SessionID),
				"group_id":   strings.TrimSpace(message.GroupID),
			},
		}),
	}); err != nil {
		if IsPairingApprovalRequired(err) {
			if notice := PairingApprovalNoticeText(err); notice != "" {
				_, _ = c.SendDeliveryMessage(requestCtx, *delivery, notice)
			}
			return
		}
		_, _ = c.SendDeliveryMessage(requestCtx, *delivery, "⚠️ 微信消息处理失败: "+TruncateError(err))
	}
}

func personalWeixinInboundMessageID(message personalWeixinMessage) string {
	if message.MessageID > 0 {
		return strconv.FormatInt(message.MessageID, 10)
	}
	if clientID := strings.TrimSpace(message.ClientID); clientID != "" {
		return clientID
	}
	if message.Seq > 0 {
		return strconv.FormatInt(message.Seq, 10)
	}
	return ""
}

func (c *PersonalWeixinChannel) currentIngress() channelcontract.IngressAcceptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ingress
}
