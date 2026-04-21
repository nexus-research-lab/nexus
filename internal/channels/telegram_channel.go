package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type telegramChannel struct {
	token   string
	client  *http.Client
	baseURL string

	mu      sync.RWMutex
	ingress IngressAcceptor
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newTelegramChannel(token string, client *http.Client) *telegramChannel {
	if client == nil {
		client = http.DefaultClient
	}
	return &telegramChannel{
		token:   strings.TrimSpace(token),
		client:  client,
		baseURL: "https://api.telegram.org",
	}
}

func (c *telegramChannel) ChannelType() string {
	return ChannelTypeTelegram
}

func (c *telegramChannel) SetIngress(ingress IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *telegramChannel) Start(ctx context.Context) error {
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

func (c *telegramChannel) Stop(context.Context) error {
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

func (c *telegramChannel) SendDeliveryText(ctx context.Context, target DeliveryTarget, text string) error {
	if strings.TrimSpace(c.token) == "" {
		return fmt.Errorf("telegram channel is not configured")
	}
	if strings.TrimSpace(target.To) == "" {
		return fmt.Errorf("telegram delivery target requires to")
	}

	for _, chunk := range splitText(strings.TrimSpace(text), 4000) {
		payload := map[string]any{
			"chat_id": target.To,
			"text":    chunk,
		}
		if strings.TrimSpace(target.ThreadID) != "" {
			threadID, err := strconv.ParseInt(strings.TrimSpace(target.ThreadID), 10, 64)
			if err != nil {
				return fmt.Errorf("telegram thread_id is invalid: %w", err)
			}
			payload["message_thread_id"] = threadID
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			strings.TrimRight(c.baseURL, "/")+"/bot"+c.token+"/sendMessage",
			bytes.NewReader(body),
		)
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")

		response, err := c.client.Do(request)
		if err != nil {
			return err
		}
		if err = expectSuccess(response); err != nil {
			return err
		}
	}
	return nil
}

func (c *telegramChannel) pollUpdates(ctx context.Context) {
	defer c.wg.Done()

	offset := 0
	for {
		if ctx.Err() != nil {
			return
		}
		updates, nextOffset, err := c.fetchUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}
		offset = nextOffset
		for _, update := range updates {
			c.handleUpdate(ctx, update)
		}
	}
}

func (c *telegramChannel) fetchUpdates(ctx context.Context, offset int) ([]telegramUpdate, int, error) {
	payload, err := json.Marshal(map[string]any{
		"offset":          offset,
		"timeout":         30,
		"allowed_updates": []string{"message"},
	})
	if err != nil {
		return nil, offset, err
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/bot"+c.token+"/getUpdates",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, offset, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return nil, offset, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, offset, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, offset, fmt.Errorf(
			"telegram getUpdates failed: status=%d body=%s",
			response.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var envelope telegramUpdatesEnvelope
	if err = json.Unmarshal(body, &envelope); err != nil {
		return nil, offset, err
	}
	if !envelope.OK {
		return nil, offset, fmt.Errorf("telegram getUpdates returned not ok: %s", strings.TrimSpace(envelope.Description))
	}

	nextOffset := offset
	for _, update := range envelope.Result {
		if update.UpdateID >= nextOffset {
			nextOffset = update.UpdateID + 1
		}
	}
	return envelope.Result, nextOffset, nil
}

func (c *telegramChannel) handleUpdate(ctx context.Context, update telegramUpdate) {
	message := update.Message
	if message == nil {
		message = update.EditedMessage
	}
	if message == nil || message.From == nil || message.From.IsBot {
		return
	}

	content := strings.TrimSpace(message.Text)
	if content == "" {
		content = strings.TrimSpace(message.Caption)
	}
	if content == "" {
		return
	}

	ingress := c.currentIngress()
	if ingress == nil {
		return
	}

	chatType := "group"
	ref := strconv.FormatInt(message.Chat.ID, 10)
	threadID := ""
	delivery := &DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeTelegram,
		To:      strconv.FormatInt(message.Chat.ID, 10),
	}
	if strings.EqualFold(message.Chat.Type, "private") {
		chatType = "dm"
		ref = strconv.FormatInt(message.From.ID, 10)
	}
	if message.MessageThreadID != 0 {
		threadID = strconv.Itoa(message.MessageThreadID)
		delivery.ThreadID = threadID
	}

	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := ingress.Accept(requestCtx, IngressRequest{
		Channel:  ChannelTypeTelegram,
		ChatType: chatType,
		Ref:      ref,
		ThreadID: threadID,
		Content:  content,
		Delivery: delivery,
	}); err != nil {
		_ = c.SendDeliveryText(requestCtx, *delivery, "⚠️ Telegram 消息处理失败: "+truncateChannelError(err))
	}
}

func (c *telegramChannel) currentIngress() IngressAcceptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ingress
}

type telegramUpdatesEnvelope struct {
	OK          bool             `json:"ok"`
	Description string           `json:"description,omitempty"`
	Result      []telegramUpdate `json:"result,omitempty"`
}

type telegramUpdate struct {
	UpdateID      int              `json:"update_id"`
	Message       *telegramMessage `json:"message,omitempty"`
	EditedMessage *telegramMessage `json:"edited_message,omitempty"`
}

type telegramMessage struct {
	MessageID       int           `json:"message_id"`
	MessageThreadID int           `json:"message_thread_id,omitempty"`
	Text            string        `json:"text,omitempty"`
	Caption         string        `json:"caption,omitempty"`
	From            *telegramUser `json:"from,omitempty"`
	Chat            telegramChat  `json:"chat"`
}

type telegramUser struct {
	ID    int64 `json:"id"`
	IsBot bool  `json:"is_bot"`
}

type telegramChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}
