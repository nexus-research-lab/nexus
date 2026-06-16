package adapters

import (
	"context"
	"fmt"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
)

type DiscordChannel struct {
	token       string
	client      *http.Client
	baseURL     string
	ownerUserID string

	mu      sync.RWMutex
	ingress channelcontract.IngressAcceptor
	session *discordgo.Session
}

type discordSendMessageResponse struct {
	ID string `json:"id"`
}

func NewDiscordChannel(token string, client *http.Client) *DiscordChannel {
	if client == nil {
		client = channeltransport.DefaultHTTPClient
	}
	return &DiscordChannel{
		token:   strings.TrimSpace(token),
		client:  client,
		baseURL: "https://discord.com/api/v10",
	}
}

func (c *DiscordChannel) WithOwner(ownerUserID string) *DiscordChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *DiscordChannel) WithBaseURL(baseURL string) *DiscordChannel {
	if baseURL = strings.TrimSpace(baseURL); baseURL != "" {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
	return c
}

func (c *DiscordChannel) BaseURL() string {
	return c.baseURL
}

func (c *DiscordChannel) ChannelType() string {
	return channelcontract.ChannelTypeDiscord
}

func (c *DiscordChannel) SetIngress(ingress channelcontract.IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *DiscordChannel) Start(context.Context) error {
	if strings.TrimSpace(c.token) == "" {
		return nil
	}

	c.mu.Lock()
	if c.session != nil {
		c.mu.Unlock()
		return nil
	}
	session, err := discordgo.New("Bot " + c.token)
	if err != nil {
		c.mu.Unlock()
		return err
	}
	session.Client = c.client
	session.Dialer = channeltransport.NewWebsocketDialer()
	session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent
	session.AddHandler(c.handleMessageCreate)
	c.session = session
	c.mu.Unlock()

	if err = session.Open(); err != nil {
		c.mu.Lock()
		if c.session == session {
			c.session = nil
		}
		c.mu.Unlock()
		_ = session.Close()
		return err
	}
	return nil
}

func (c *DiscordChannel) Stop(context.Context) error {
	c.mu.Lock()
	session := c.session
	c.session = nil
	c.mu.Unlock()
	if session == nil {
		return nil
	}
	return session.Close()
}

func (c *DiscordChannel) SendDeliveryMessage(ctx context.Context, target channelcontract.DeliveryTarget, text string) (channelcontract.DeliveryResult, error) {
	normalized := target.Normalized()
	if strings.TrimSpace(c.token) == "" {
		return channelcontract.DeliveryResult{}, fmt.Errorf("discord channel is not configured")
	}
	targetID := channelcontract.FirstNonEmpty(target.ThreadID, target.To)
	if targetID == "" {
		return channelcontract.DeliveryResult{}, fmt.Errorf("discord delivery target requires to or thread_id")
	}

	parts := make([]channelmessage.ReceiptPart, 0)
	for _, chunk := range channeltransport.SplitText(strings.TrimSpace(text), 1900) {
		payload := map[string]any{
			"content": chunk,
			"allowed_mentions": map[string]any{
				"parse": []string{},
			},
		}
		var response discordSendMessageResponse
		if err := channeltransport.DoJSONExpectSuccessDecode(
			ctx,
			c.client,
			http.MethodPost,
			strings.TrimRight(c.baseURL, "/")+"/channels/"+targetID+"/messages",
			payload,
			map[string]string{"Authorization": "Bot " + c.token},
			&response,
		); err != nil {
			return channelcontract.DeliveryResult{}, err
		}
		if strings.TrimSpace(response.ID) != "" {
			parts = append(parts, channelmessage.TextPart(response.ID))
		}
	}
	return channelcontract.NewDeliveryResult(normalized, channelmessage.NewReceipt(channelmessage.ReceiptParams{
		Channel:  channelcontract.ChannelTypeDiscord,
		Target:   targetID,
		ThreadID: target.ThreadID,
		Parts:    parts,
	})), nil
}

func (c *DiscordChannel) SendDeliveryTyping(ctx context.Context, target channelcontract.DeliveryTarget, active bool) error {
	if !active {
		return nil
	}
	if strings.TrimSpace(c.token) == "" {
		return fmt.Errorf("discord channel is not configured")
	}
	targetID := channelcontract.FirstNonEmpty(target.ThreadID, target.To)
	if targetID == "" {
		return fmt.Errorf("discord typing target requires to or thread_id")
	}

	return channeltransport.DoJSONExpectSuccess(
		ctx,
		c.client,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/channels/"+targetID+"/typing",
		nil,
		map[string]string{"Authorization": "Bot " + c.token},
	)
}

func (c *DiscordChannel) handleMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	if session == nil || message == nil || message.Author == nil || message.Author.Bot {
		return
	}
	content := strings.TrimSpace(message.Content)
	if content == "" {
		return
	}

	ingress := c.currentIngress()
	if ingress == nil {
		return
	}

	request, err := c.buildIngressRequest(session, message, content)
	if err != nil {
		_, _ = session.ChannelMessageSend(
			message.ChannelID,
			"⚠️ Discord 消息路由失败: "+TruncateError(err),
		)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err = ingress.Accept(ctx, request); err != nil {
		if IsPairingApprovalRequired(err) {
			if notice := PairingApprovalNoticeText(err); notice != "" {
				_, _ = session.ChannelMessageSend(message.ChannelID, notice)
			}
			return
		}
		_, _ = session.ChannelMessageSend(
			message.ChannelID,
			"⚠️ Discord 消息处理失败: "+TruncateError(err),
		)
	}
}

func (c *DiscordChannel) buildIngressRequest(
	session *discordgo.Session,
	message *discordgo.MessageCreate,
	content string,
) (channelcontract.IngressRequest, error) {
	chatType := "group"
	ref := strings.TrimSpace(message.ChannelID)
	threadID := ""
	delivery := &channelcontract.DeliveryTarget{
		Mode:    channelcontract.DeliveryModeExplicit,
		Channel: channelcontract.ChannelTypeDiscord,
		To:      strings.TrimSpace(message.ChannelID),
	}

	if strings.TrimSpace(message.GuildID) == "" {
		chatType = "dm"
		ref = strings.TrimSpace(message.Author.ID)
		return channelcontract.IngressRequest{
			Channel:     channelcontract.ChannelTypeDiscord,
			OwnerUserID: c.ownerUserID,
			AccountID:   AccountIDFromSecret("dg", c.token),
			ChatType:    chatType,
			Ref:         ref,
			Content:     content,
			RoundID:     strings.TrimSpace(message.ID),
			ReqID:       strings.TrimSpace(message.ID),
			Delivery:    delivery,
			Message: channelmessage.NewInbound(channelmessage.InboundParams{
				Channel:           channelcontract.ChannelTypeDiscord,
				Target:            ref,
				PlatformMessageID: strings.TrimSpace(message.ID),
				ThreadID:          threadID,
				ReplyToID:         discordReplyToID(message),
				SenderID:          strings.TrimSpace(message.Author.ID),
				SenderName:        strings.TrimSpace(message.Author.Username),
				ChatType:          chatType,
				Text:              content,
			}),
		}, nil
	}

	channelID := strings.TrimSpace(message.ChannelID)
	if parentID, resolvedThreadID := c.resolveDiscordThreadRoute(session, channelID); resolvedThreadID != "" {
		threadID = resolvedThreadID
		channelID = parentID
		delivery.ThreadID = resolvedThreadID
	}
	delivery.To = channelID
	delivery.AccountID = strings.TrimSpace(message.GuildID)
	ref = joinDiscordRoute(strings.TrimSpace(message.GuildID), channelID)

	return channelcontract.IngressRequest{
		Channel:     channelcontract.ChannelTypeDiscord,
		OwnerUserID: c.ownerUserID,
		AccountID:   AccountIDFromSecret("dg", c.token),
		ChatType:    chatType,
		Ref:         ref,
		ThreadID:    threadID,
		Content:     content,
		RoundID:     strings.TrimSpace(message.ID),
		ReqID:       strings.TrimSpace(message.ID),
		Delivery:    delivery,
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           channelcontract.ChannelTypeDiscord,
			Target:            ref,
			PlatformMessageID: strings.TrimSpace(message.ID),
			ThreadID:          threadID,
			ReplyToID:         discordReplyToID(message),
			SenderID:          strings.TrimSpace(message.Author.ID),
			SenderName:        strings.TrimSpace(message.Author.Username),
			ChatType:          chatType,
			Text:              content,
		}),
	}, nil
}

func discordReplyToID(message *discordgo.MessageCreate) string {
	if message == nil || message.ReferencedMessage == nil {
		return ""
	}
	return strings.TrimSpace(message.ReferencedMessage.ID)
}

func (c *DiscordChannel) resolveDiscordThreadRoute(session *discordgo.Session, channelID string) (string, string) {
	channel, err := session.State.Channel(strings.TrimSpace(channelID))
	if err != nil || channel == nil {
		channel, err = session.Channel(strings.TrimSpace(channelID))
		if err != nil || channel == nil {
			return strings.TrimSpace(channelID), ""
		}
	}
	if !isDiscordThreadType(channel.Type) {
		return strings.TrimSpace(channel.ID), ""
	}
	parentID := strings.TrimSpace(channel.ParentID)
	if parentID == "" {
		parentID = strings.TrimSpace(channel.ID)
	}
	return parentID, strings.TrimSpace(channel.ID)
}

func (c *DiscordChannel) currentIngress() channelcontract.IngressAcceptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ingress
}

func joinDiscordRoute(guildID string, channelID string) string {
	if strings.TrimSpace(guildID) == "" {
		return strings.TrimSpace(channelID)
	}
	return strings.TrimSpace(guildID) + ":" + strings.TrimSpace(channelID)
}

func isDiscordThreadType(channelType discordgo.ChannelType) bool {
	switch channelType {
	case discordgo.ChannelTypeGuildPublicThread,
		discordgo.ChannelTypeGuildPrivateThread,
		discordgo.ChannelTypeGuildNewsThread:
		return true
	default:
		return false
	}
}
