package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

type feishuChannel struct {
	appID       string
	appSecret   string
	ownerUserID string

	mu        sync.RWMutex
	ingress   IngressAcceptor
	apiClient *lark.Client
	cancel    context.CancelFunc
}

func newFeishuChannel(appID string, appSecret string) *feishuChannel {
	return &feishuChannel{
		appID:     strings.TrimSpace(appID),
		appSecret: strings.TrimSpace(appSecret),
		apiClient: lark.NewClient(
			strings.TrimSpace(appID),
			strings.TrimSpace(appSecret),
		),
	}
}

func (c *feishuChannel) WithOwner(ownerUserID string) *feishuChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *feishuChannel) ChannelType() string {
	return ChannelTypeFeishu
}

func (c *feishuChannel) SetIngress(ingress IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *feishuChannel) Start(ctx context.Context) error {
	if strings.TrimSpace(c.appID) == "" || strings.TrimSpace(c.appSecret) == "" {
		return nil
	}

	c.mu.Lock()
	if c.cancel != nil {
		c.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.mu.Unlock()

	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(c.handleMessageReceive)
	client := larkws.NewClient(
		c.appID,
		c.appSecret,
		larkws.WithEventHandler(eventHandler),
	)

	started := make(chan error, 1)
	go func() {
		started <- client.Start(runCtx)
	}()

	select {
	case err := <-started:
		c.mu.Lock()
		c.cancel = nil
		c.mu.Unlock()
		cancel()
		return err
	case <-time.After(2 * time.Second):
		return nil
	}
}

func (c *feishuChannel) Stop(context.Context) error {
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (c *feishuChannel) SendDeliveryText(ctx context.Context, target DeliveryTarget, text string) error {
	if c.apiClient == nil {
		return errors.New("feishu channel is not configured")
	}
	to := strings.TrimSpace(target.To)
	if to == "" {
		return errors.New("feishu delivery target requires to")
	}
	receiveIDType := normalizeFeishuReceiveIDType(target.AccountID, to)

	for _, chunk := range splitText(strings.TrimSpace(text), 4000) {
		content, err := json.Marshal(map[string]string{"text": chunk})
		if err != nil {
			return err
		}
		req := larkim.NewCreateMessageReqBuilder().
			ReceiveIdType(receiveIDType).
			Body(larkim.NewCreateMessageReqBodyBuilder().
				ReceiveId(to).
				MsgType("text").
				Content(string(content)).
				Uuid(newDeliveryID("feishu_msg")).
				Build()).
			Build()
		resp, err := c.apiClient.Im.Message.Create(ctx, req)
		if err != nil {
			return err
		}
		if resp == nil || !resp.Success() {
			if resp == nil {
				return errors.New("feishu create message returned empty response")
			}
			return fmt.Errorf("feishu create message failed: code=%d msg=%s", resp.Code, strings.TrimSpace(resp.Msg))
		}
	}
	return nil
}

func (c *feishuChannel) handleMessageReceive(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil
	}
	message := event.Event.Message
	content := extractFeishuText(message.MessageType, message.Content)
	if content == "" {
		return nil
	}

	ingress := c.currentIngress()
	if ingress == nil {
		return nil
	}

	chatType := "group"
	ref := stringPointerValue(message.ChatId)
	receiveIDType := feishuReceiveIDTypeChatID
	if strings.EqualFold(stringPointerValue(message.ChatType), "p2p") {
		chatType = "dm"
		ref = feishuSenderOpenID(event.Event.Sender)
		receiveIDType = feishuReceiveIDTypeOpenID
	}
	if ref == "" {
		return nil
	}

	delivery := &DeliveryTarget{
		Mode:      DeliveryModeExplicit,
		Channel:   ChannelTypeFeishu,
		To:        ref,
		AccountID: receiveIDType,
		ThreadID:  stringPointerValue(message.ThreadId),
	}
	request := IngressRequest{
		Channel:      ChannelTypeFeishu,
		OwnerUserID:  c.ownerUserID,
		ChatType:     chatType,
		Ref:          ref,
		ThreadID:     stringPointerValue(message.ThreadId),
		ExternalName: ref,
		Content:      content,
		ReqID:        stringPointerValue(message.MessageId),
		Delivery:     delivery,
	}

	go func() {
		requestCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_, _ = ingress.Accept(requestCtx, request)
	}()
	return nil
}

func (c *feishuChannel) currentIngress() IngressAcceptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ingress
}

func extractFeishuText(messageType *string, raw *string) string {
	if !strings.EqualFold(stringPointerValue(messageType), "text") {
		return ""
	}
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(stringPointerValue(raw)), &payload); err != nil {
		return strings.TrimSpace(stringPointerValue(raw))
	}
	return strings.TrimSpace(payload.Text)
}

func feishuSenderOpenID(sender *larkim.EventSender) string {
	if sender == nil || sender.SenderId == nil {
		return ""
	}
	return stringPointerValue(sender.SenderId.OpenId)
}

func normalizeFeishuReceiveIDType(raw string, to string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case feishuReceiveIDTypeOpenID, "open":
		return feishuReceiveIDTypeOpenID
	case feishuReceiveIDTypeChatID, "chat":
		return feishuReceiveIDTypeChatID
	default:
		if strings.HasPrefix(strings.TrimSpace(to), "ou_") {
			return feishuReceiveIDTypeOpenID
		}
		return feishuReceiveIDTypeChatID
	}
}
