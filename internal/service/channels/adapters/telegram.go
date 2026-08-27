package adapters

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
)

type telegramSendMessageResponse struct {
	OK          *bool  `json:"ok"`
	Description string `json:"description,omitempty"`
	Result      struct {
		MessageID int64 `json:"message_id"`
	} `json:"result"`
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
