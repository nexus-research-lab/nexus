package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	dingtalkclient "github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
)

type dingTalkChannel struct {
	clientID     string
	clientSecret string
	ownerUserID  string
	httpClient   *http.Client

	mu      sync.RWMutex
	ingress IngressAcceptor
	client  *dingtalkclient.StreamClient
}

func newDingTalkChannel(clientID string, clientSecret string, httpClient *http.Client) *dingTalkChannel {
	if httpClient == nil {
		httpClient = defaultChannelHTTPClient
	}
	return &dingTalkChannel{
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		httpClient:   httpClient,
	}
}

func (c *dingTalkChannel) WithOwner(ownerUserID string) *dingTalkChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *dingTalkChannel) ChannelType() string {
	return ChannelTypeDingTalk
}

func (c *dingTalkChannel) SetIngress(ingress IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *dingTalkChannel) Start(ctx context.Context) error {
	if strings.TrimSpace(c.clientID) == "" || strings.TrimSpace(c.clientSecret) == "" {
		return nil
	}

	c.mu.Lock()
	if c.client != nil {
		c.mu.Unlock()
		return nil
	}
	client := dingtalkclient.NewStreamClient(
		dingtalkclient.WithAppCredential(dingtalkclient.NewAppCredentialConfig(c.clientID, c.clientSecret)),
	)
	client.RegisterChatBotCallbackRouter(c.handleBotMessage)
	c.client = client
	c.mu.Unlock()

	if err := client.Start(ctx); err != nil {
		c.mu.Lock()
		if c.client == client {
			c.client = nil
		}
		c.mu.Unlock()
		client.Close()
		return err
	}
	go func() {
		<-ctx.Done()
		_ = c.Stop(context.Background())
	}()
	return nil
}

func (c *dingTalkChannel) Stop(context.Context) error {
	c.mu.Lock()
	client := c.client
	c.client = nil
	c.mu.Unlock()
	if client != nil {
		client.Close()
	}
	return nil
}

func (c *dingTalkChannel) SendDeliveryText(ctx context.Context, target DeliveryTarget, text string) error {
	webhook := strings.TrimSpace(target.To)
	if webhook == "" {
		return errors.New("dingtalk delivery target requires session webhook")
	}
	for _, chunk := range splitText(strings.TrimSpace(text), 4000) {
		if err := c.sendSessionWebhook(ctx, webhook, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (c *dingTalkChannel) handleBotMessage(ctx context.Context, data *chatbot.BotCallbackDataModel) ([]byte, error) {
	if data == nil {
		return nil, nil
	}
	content := strings.TrimSpace(data.Text.Content)
	if content == "" {
		return nil, nil
	}

	ingress := c.currentIngress()
	if ingress == nil {
		return nil, nil
	}

	chatType := normalizeDingTalkChatType(data.ConversationType)
	ref := firstNonEmpty(data.ConversationId, data.SenderId, data.SenderStaffId)
	if ref == "" {
		return nil, nil
	}

	delivery := &DeliveryTarget{
		Mode:      DeliveryModeExplicit,
		Channel:   ChannelTypeDingTalk,
		To:        strings.TrimSpace(data.SessionWebhook),
		AccountID: strings.TrimSpace(data.ConversationId),
	}
	request := IngressRequest{
		Channel:      ChannelTypeDingTalk,
		OwnerUserID:  c.ownerUserID,
		ChatType:     chatType,
		Ref:          ref,
		ExternalName: firstNonEmpty(data.ConversationTitle, data.SenderNick),
		Content:      content,
		ReqID:        strings.TrimSpace(data.MsgId),
		Delivery:     delivery,
	}

	go func() {
		requestCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_, _ = ingress.Accept(requestCtx, request)
	}()

	return nil, nil
}

func (c *dingTalkChannel) currentIngress() IngressAcceptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ingress
}

func (c *dingTalkChannel) sendSessionWebhook(ctx context.Context, webhook string, text string) error {
	payload, err := json.Marshal(map[string]any{
		"msgtype": "text",
		"text": map[string]string{
			"content": text,
		},
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	if err = expectSuccess(response); err != nil {
		return fmt.Errorf("dingtalk session webhook failed: %w", err)
	}
	return nil
}

func normalizeDingTalkChatType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "single", "private", "p2p", "dm":
		return "dm"
	default:
		return "group"
	}
}
