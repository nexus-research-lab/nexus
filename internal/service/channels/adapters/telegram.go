package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
)

const telegramGeneralTopicID int64 = 1

type TelegramChannel struct {
	token       string
	client      *http.Client
	baseURL     string
	ownerUserID string
	logger      *slog.Logger

	mu      sync.RWMutex
	ingress channelcontract.IngressAcceptor
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

type telegramSendMessageResponse struct {
	OK          *bool  `json:"ok"`
	Description string `json:"description,omitempty"`
	Result      struct {
		MessageID int64 `json:"message_id"`
	} `json:"result"`
}

func NewTelegramChannel(token string, client *http.Client) *TelegramChannel {
	if client == nil {
		client = channeltransport.DefaultHTTPClient
	}
	return &TelegramChannel{
		token:   strings.TrimSpace(token),
		client:  client,
		baseURL: "https://api.telegram.org",
		logger:  logx.NewDiscardLogger(),
	}
}

func (c *TelegramChannel) WithOwner(ownerUserID string) *TelegramChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *TelegramChannel) WithBaseURL(baseURL string) *TelegramChannel {
	if baseURL = strings.TrimSpace(baseURL); baseURL != "" {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
	return c
}

func (c *TelegramChannel) BaseURL() string {
	return c.baseURL
}

func (c *TelegramChannel) ChannelType() string {
	return channelcontract.ChannelTypeTelegram
}

func (c *TelegramChannel) SetIngress(ingress channelcontract.IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *TelegramChannel) SetLogger(logger *slog.Logger) {
	if logger == nil {
		logger = logx.NewDiscardLogger()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logger = logger
}

func (c *TelegramChannel) Start(ctx context.Context) error {
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

func (c *TelegramChannel) Stop(context.Context) error {
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

func (c *TelegramChannel) SendDeliveryMessage(
	ctx context.Context,
	target channelcontract.DeliveryTarget,
	text string,
) (channelcontract.DeliveryResult, error) {
	normalized := target.Normalized()
	if strings.TrimSpace(c.token) == "" {
		return channelcontract.DeliveryResult{}, fmt.Errorf("telegram channel is not configured")
	}
	if strings.TrimSpace(target.To) == "" {
		return channelcontract.DeliveryResult{}, fmt.Errorf("telegram delivery target requires to")
	}

	parts := make([]channelmessage.ReceiptPart, 0)
	for _, chunk := range channeltransport.SplitText(strings.TrimSpace(text), 4000) {
		payload := map[string]any{
			"chat_id":                  target.To,
			"text":                     chunk,
			"disable_web_page_preview": true,
		}
		if err := applyTelegramSendThreadID(payload, target.ThreadID); err != nil {
			return channelcontract.DeliveryResult{}, err
		}
		var response telegramSendMessageResponse
		if err := channeltransport.DoJSONExpectSuccessDecode(
			ctx,
			c.client,
			http.MethodPost,
			strings.TrimRight(c.baseURL, "/")+"/bot"+c.token+"/sendMessage",
			payload,
			nil,
			&response,
		); err != nil {
			return channelcontract.DeliveryResult{}, c.redactError(err)
		}
		if response.OK != nil && !*response.OK {
			description := strings.TrimSpace(response.Description)
			if description == "" {
				description = "ok=false"
			}
			return channelcontract.DeliveryResult{}, fmt.Errorf("telegram sendMessage failed: %s", description)
		}
		if response.Result.MessageID != 0 {
			parts = append(parts, channelmessage.TextPart(strconv.FormatInt(response.Result.MessageID, 10)))
		}
	}
	return channelcontract.NewDeliveryResult(normalized, channelmessage.NewReceipt(channelmessage.ReceiptParams{
		Channel:  channelcontract.ChannelTypeTelegram,
		Target:   target.To,
		ThreadID: target.ThreadID,
		Parts:    parts,
	})), nil
}

func (c *TelegramChannel) SendDeliveryTyping(ctx context.Context, target channelcontract.DeliveryTarget, active bool) error {
	if !active {
		return nil
	}
	if strings.TrimSpace(c.token) == "" {
		return fmt.Errorf("telegram channel is not configured")
	}
	if strings.TrimSpace(target.To) == "" {
		return fmt.Errorf("telegram typing target requires to")
	}

	payload := map[string]any{
		"chat_id": target.To,
		"action":  "typing",
	}
	if err := applyTelegramTypingThreadID(payload, target.ThreadID); err != nil {
		return err
	}
	if err := channeltransport.DoJSONExpectSuccess(
		ctx,
		c.client,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/bot"+c.token+"/sendChatAction",
		payload,
		nil,
	); err != nil {
		return c.redactError(err)
	}
	return nil
}

func (c *TelegramChannel) pollUpdates(ctx context.Context) {
	defer c.wg.Done()

	offset := 0
	lastErrText := ""
	lastErrLoggedAt := time.Time{}
	for {
		if ctx.Err() != nil {
			return
		}
		updates, nextOffset, err := c.fetchUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			errText := strings.TrimSpace(err.Error())
			now := time.Now()
			if errText != lastErrText || now.Sub(lastErrLoggedAt) >= 30*time.Second {
				c.loggerFor(ctx).Warn("Telegram getUpdates 失败",
					"owner_user_id", c.ownerUserID,
					"err", c.redactError(err),
				)
				lastErrText = errText
				lastErrLoggedAt = now
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
		if lastErrText != "" {
			c.loggerFor(ctx).Info("Telegram getUpdates 已恢复",
				"owner_user_id", c.ownerUserID,
			)
			lastErrText = ""
			lastErrLoggedAt = time.Time{}
		}
		offset = nextOffset
		for _, update := range updates {
			c.handleUpdate(ctx, update)
		}
	}
}

func (c *TelegramChannel) fetchUpdates(ctx context.Context, offset int) ([]telegramUpdate, int, error) {
	payload := map[string]any{
		"offset":          offset,
		"timeout":         30,
		"allowed_updates": []string{"message", "edited_message"},
	}
	response, err := channeltransport.DoJSON(
		ctx,
		c.client,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/bot"+c.token+"/getUpdates",
		payload,
		nil,
	)
	if err != nil {
		return nil, offset, c.redactError(err)
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

func applyTelegramSendThreadID(payload map[string]any, rawThreadID string) error {
	return applyTelegramThreadID(payload, rawThreadID, false)
}

func applyTelegramTypingThreadID(payload map[string]any, rawThreadID string) error {
	return applyTelegramThreadID(payload, rawThreadID, true)
}

func applyTelegramThreadID(payload map[string]any, rawThreadID string, includeGeneralTopic bool) error {
	if strings.TrimSpace(rawThreadID) == "" {
		return nil
	}
	threadID, err := strconv.ParseInt(strings.TrimSpace(rawThreadID), 10, 64)
	if err != nil {
		return fmt.Errorf("telegram thread_id is invalid: %w", err)
	}
	if threadID == telegramGeneralTopicID && !includeGeneralTopic {
		return nil
	}
	payload["message_thread_id"] = threadID
	return nil
}

func (c *TelegramChannel) handleUpdate(ctx context.Context, update telegramUpdate) {
	message := update.Message
	edited := false
	if message == nil {
		message = update.EditedMessage
		edited = message != nil
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
		c.loggerFor(ctx).Warn("Telegram 入站消息缺少处理器",
			"owner_user_id", c.ownerUserID,
			"update_id", update.UpdateID,
		)
		return
	}

	chatType := "group"
	ref := strconv.FormatInt(message.Chat.ID, 10)
	threadID := ""
	delivery := &channelcontract.DeliveryTarget{
		Mode:    channelcontract.DeliveryModeExplicit,
		Channel: channelcontract.ChannelTypeTelegram,
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
	messageID := strconv.Itoa(message.MessageID)
	reqID := messageID
	if edited {
		reqID = telegramEditedMessageReqID(messageID, update.UpdateID)
	}
	c.loggerFor(ctx).Debug("收到 Telegram 入站消息",
		"owner_user_id", c.ownerUserID,
		"chat_type", chatType,
		"ref", ref,
		"thread_id", threadID,
		"message_id", messageID,
		"edited", edited,
		"chars", len([]rune(content)),
	)
	if _, err := ingress.Accept(requestCtx, channelcontract.IngressRequest{
		Channel:     channelcontract.ChannelTypeTelegram,
		OwnerUserID: c.ownerUserID,
		AccountID:   AccountIDFromSecret("tg", c.token),
		ChatType:    chatType,
		Ref:         ref,
		ThreadID:    threadID,
		Content:     content,
		ReqID:       reqID,
		Delivery:    delivery,
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           channelcontract.ChannelTypeTelegram,
			Target:            ref,
			PlatformMessageID: messageID,
			ThreadID:          threadID,
			SenderID:          strconv.FormatInt(message.From.ID, 10),
			ChatType:          chatType,
			Text:              content,
			Edited:            edited,
		}),
	}); err != nil {
		if IsPairingApprovalRequired(err) {
			if notice := PairingApprovalNoticeText(err); notice != "" {
				_, _ = c.SendDeliveryMessage(requestCtx, *delivery, notice)
			}
			return
		}
		c.loggerFor(ctx).Warn("Telegram 入站消息处理失败",
			"owner_user_id", c.ownerUserID,
			"chat_type", chatType,
			"ref", ref,
			"thread_id", threadID,
			"message_id", messageID,
			"err", err,
		)
		_, _ = c.SendDeliveryMessage(requestCtx, *delivery, "⚠️ Telegram 消息处理失败: "+TruncateError(err))
	}
}

func telegramEditedMessageReqID(messageID string, updateID int) string {
	trimmed := strings.TrimSpace(messageID)
	if updateID == 0 {
		return trimmed + ":edited"
	}
	return fmt.Sprintf("%s:edited:%d", trimmed, updateID)
}

func (c *TelegramChannel) redactError(err error) error {
	if err == nil {
		return nil
	}
	text := strings.TrimSpace(err.Error())
	token := strings.TrimSpace(c.token)
	if token != "" {
		text = strings.ReplaceAll(text, "/bot"+token+"/", "/bot<redacted>/")
		text = strings.ReplaceAll(text, "bot"+token, "bot<redacted>")
	}
	if text == "" {
		text = "telegram request failed"
	}
	return errors.New(text)
}

func (c *TelegramChannel) currentIngress() channelcontract.IngressAcceptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ingress
}

func (c *TelegramChannel) loggerFor(ctx context.Context) *slog.Logger {
	c.mu.RLock()
	logger := c.logger
	c.mu.RUnlock()
	return logx.Resolve(ctx, logger)
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
