package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
)

type FeishuChannel struct {
	appID             string
	appSecret         string
	client            *http.Client
	baseURL           string
	ownerUserID       string
	verificationToken string
	encryptKey        string
	connectionMode    string
	replyInThread     bool

	mu             sync.Mutex
	tenantToken    string
	tokenExpiresAt time.Time
	ingress        channelcontract.IngressAcceptor
	cancel         context.CancelFunc
	eventClient    feishuEventClient
	eventFactory   feishuEventClientFactory
	typingReacts   map[string]string
}

type feishuEventClient interface {
	Start(context.Context) error
	Close()
}

type feishuEventClientFactory func(feishuEventClientConfig) feishuEventClient

type feishuEventClientConfig struct {
	AppID             string
	AppSecret         string
	BaseURL           string
	VerificationToken string
	EncryptKey        string
	OnReady           func()
	OnError           func(error)
	OnMessage         func(context.Context, *larkim.P2MessageReceiveV1) error
	OnReaction        func(context.Context, *larkim.P2MessageReactionCreatedV1) error
}

func NewFeishuChannel(appID string, appSecret string, client *http.Client) *FeishuChannel {
	if client == nil {
		client = channeltransport.DefaultHTTPClient
	}
	return &FeishuChannel{
		appID:          strings.TrimSpace(appID),
		appSecret:      strings.TrimSpace(appSecret),
		client:         client,
		baseURL:        "https://open.feishu.cn",
		connectionMode: "websocket",
		eventFactory:   newFeishuSDKEventClient,
	}
}

func (c *FeishuChannel) WithOwner(ownerUserID string) *FeishuChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *FeishuChannel) WithEventSecurity(verificationToken string, encryptKey string) *FeishuChannel {
	c.verificationToken = strings.TrimSpace(verificationToken)
	c.encryptKey = strings.TrimSpace(encryptKey)
	return c
}

func (c *FeishuChannel) WithBaseURL(baseURL string) *FeishuChannel {
	if baseURL = strings.TrimSpace(baseURL); baseURL != "" {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
	return c
}

func (c *FeishuChannel) WithConnectionMode(mode string) *FeishuChannel {
	c.connectionMode = normalizeFeishuConnectionMode(mode)
	return c
}

func (c *FeishuChannel) WithReplyInThread(value string) *FeishuChannel {
	c.replyInThread = normalizeFeishuReplyInThread(value)
	return c
}

func (c *FeishuChannel) BaseURL() string {
	return c.baseURL
}

func (c *FeishuChannel) ConnectionMode() string {
	return c.connectionMode
}

func (c *FeishuChannel) ReplyInThread() bool {
	return c.replyInThread
}

func (c *FeishuChannel) ChannelType() string {
	return channelcontract.ChannelTypeFeishu
}

func (c *FeishuChannel) SetIngress(ingress channelcontract.IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *FeishuChannel) Start(ctx context.Context) error {
	if c.appID == "" || c.appSecret == "" {
		return fmt.Errorf("feishu channel is not configured")
	}
	if normalizeFeishuConnectionMode(c.connectionMode) == "webhook" {
		return nil
	}

	ready := make(chan struct{}, 1)
	startErr := make(chan error, 1)
	runCtx, cancel := context.WithCancel(ctx)
	client := c.eventFactory(feishuEventClientConfig{
		AppID:             c.appID,
		AppSecret:         c.appSecret,
		BaseURL:           c.baseURL,
		VerificationToken: c.verificationToken,
		EncryptKey:        c.encryptKey,
		OnReady: func() {
			select {
			case ready <- struct{}{}:
			default:
			}
		},
		OnError: func(err error) {
			if err == nil {
				return
			}
			select {
			case startErr <- err:
			default:
			}
		},
		OnMessage:  c.handleSDKMessage,
		OnReaction: c.handleSDKReaction,
	})

	c.mu.Lock()
	if c.eventClient != nil {
		cancel()
		c.mu.Unlock()
		return nil
	}
	c.cancel = cancel
	c.eventClient = client
	c.mu.Unlock()

	go func() {
		if err := client.Start(runCtx); err != nil {
			select {
			case startErr <- err:
			default:
			}
		}
	}()

	select {
	case <-ready:
		return nil
	case err := <-startErr:
		c.clearEventClient(client)
		client.Close()
		cancel()
		return err
	case <-time.After(8 * time.Second):
		return nil
	case <-ctx.Done():
		c.clearEventClient(client)
		client.Close()
		cancel()
		return ctx.Err()
	}
}

func (c *FeishuChannel) clearEventClient(client feishuEventClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.eventClient == client {
		c.eventClient = nil
		c.cancel = nil
	}
}

func (c *FeishuChannel) Stop(context.Context) error {
	c.mu.Lock()
	cancel := c.cancel
	client := c.eventClient
	c.cancel = nil
	c.eventClient = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if client != nil {
		client.Close()
	}
	return nil
}

func (c *FeishuChannel) currentIngress() channelcontract.IngressAcceptor {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ingress
}

func newFeishuSDKEventClient(config feishuEventClientConfig) feishuEventClient {
	sdkLogger := feishuSDKLogger{}
	dispatcher := larkdispatcher.NewEventDispatcher(config.VerificationToken, config.EncryptKey)
	dispatcher.InitConfig(larkevent.WithLogger(sdkLogger), larkevent.WithLogLevel(larkcore.LogLevelWarn))
	dispatcher.OnP2MessageReceiveV1(config.OnMessage)
	if config.OnReaction != nil {
		dispatcher.OnP2MessageReactionCreatedV1(config.OnReaction)
	}
	options := []larkws.ClientOption{
		larkws.WithEventHandler(dispatcher),
		larkws.WithLogLevel(larkcore.LogLevelWarn),
		larkws.WithLogger(sdkLogger),
		larkws.WithOnReady(config.OnReady),
		larkws.WithOnError(config.OnError),
	}
	if baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/"); baseURL != "" {
		options = append(options, larkws.WithDomain(baseURL))
	}
	return larkws.NewClient(config.AppID, config.AppSecret, options...)
}

func normalizeFeishuConnectionMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "webhook", "http", "callback":
		return "webhook"
	default:
		return "websocket"
	}
}

func normalizeFeishuReplyInThread(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "enabled", "enable":
		return true
	default:
		return false
	}
}

type feishuSDKLogger struct{}

func (feishuSDKLogger) Debug(context.Context, ...interface{}) {}

func (feishuSDKLogger) Info(context.Context, ...interface{}) {}

func (feishuSDKLogger) Warn(ctx context.Context, args ...interface{}) {
	detail := formatFeishuSDKLog(args...)
	if detail == "" || isFeishuSDKExpectedCloseLog(detail) {
		return
	}
	slog.Default().WarnContext(ctx, "飞书 SDK 长连接警告", "detail", detail)
}

func (feishuSDKLogger) Error(ctx context.Context, args ...interface{}) {
	detail := formatFeishuSDKLog(args...)
	if detail == "" || isFeishuSDKExpectedCloseLog(detail) {
		return
	}
	slog.Default().ErrorContext(ctx, "飞书 SDK 长连接错误", "detail", detail)
}

func formatFeishuSDKLog(args ...interface{}) string {
	return strings.TrimSpace(fmt.Sprint(args...))
}

func isFeishuSDKExpectedCloseLog(detail string) bool {
	normalized := strings.ToLower(strings.TrimSpace(detail))
	return strings.Contains(normalized, "use of closed network connection") ||
		strings.Contains(normalized, "connection is closed, receive message loop exit") ||
		strings.Contains(normalized, "websocket: close 1000")
}
