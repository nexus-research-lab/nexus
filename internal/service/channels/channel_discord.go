package channels

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

type discordChannel struct {
	token       string
	client      *http.Client
	baseURL     string
	ownerUserID string

	mu      sync.RWMutex
	ingress IngressAcceptor
	session *discordgo.Session
}

type discordSendMessageResponse struct {
	ID string `json:"id"`
}

func newDiscordChannel(token string, client *http.Client) *discordChannel {
	if client == nil {
		client = defaultChannelHTTPClient
	}
	return &discordChannel{
		token:   strings.TrimSpace(token),
		client:  client,
		baseURL: "https://discord.com/api/v10",
	}
}

func (c *discordChannel) WithOwner(ownerUserID string) *discordChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *discordChannel) ChannelType() string {
	return ChannelTypeDiscord
}

func (c *discordChannel) SetIngress(ingress IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *discordChannel) Start(context.Context) error {
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
	session.Dialer = newChannelWebsocketDialer()
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

func (c *discordChannel) Stop(context.Context) error {
	c.mu.Lock()
	session := c.session
	c.session = nil
	c.mu.Unlock()
	if session == nil {
		return nil
	}
	return session.Close()
}

func (c *discordChannel) SendDeliveryMessage(ctx context.Context, target DeliveryTarget, text string) (DeliveryResult, error) {
	normalized := target.Normalized()
	if strings.TrimSpace(c.token) == "" {
		return DeliveryResult{}, fmt.Errorf("discord channel is not configured")
	}
	targetID := firstNonEmpty(target.ThreadID, target.To)
	if targetID == "" {
		return DeliveryResult{}, fmt.Errorf("discord delivery target requires to or thread_id")
	}

	parts := make([]channelmessage.ReceiptPart, 0)
	for _, chunk := range splitText(strings.TrimSpace(text), 1900) {
		payload := map[string]any{
			"content": chunk,
			"allowed_mentions": map[string]any{
				"parse": []string{},
			},
		}
		var response discordSendMessageResponse
		if err := doChannelJSONExpectSuccessDecode(
			ctx,
			c.client,
			http.MethodPost,
			strings.TrimRight(c.baseURL, "/")+"/channels/"+targetID+"/messages",
			payload,
			map[string]string{"Authorization": "Bot " + c.token},
			&response,
		); err != nil {
			return DeliveryResult{}, err
		}
		if strings.TrimSpace(response.ID) != "" {
			parts = append(parts, channelmessage.TextPart(response.ID))
		}
	}
	return newDeliveryResult(normalized, channelmessage.NewReceipt(channelmessage.ReceiptParams{
		Channel:  ChannelTypeDiscord,
		Target:   targetID,
		ThreadID: target.ThreadID,
		Parts:    parts,
	})), nil
}

func (c *discordChannel) SendDeliveryTyping(ctx context.Context, target DeliveryTarget, active bool) error {
	if !active {
		return nil
	}
	if strings.TrimSpace(c.token) == "" {
		return fmt.Errorf("discord channel is not configured")
	}
	targetID := firstNonEmpty(target.ThreadID, target.To)
	if targetID == "" {
		return fmt.Errorf("discord typing target requires to or thread_id")
	}

	return doChannelJSONExpectSuccess(
		ctx,
		c.client,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/channels/"+targetID+"/typing",
		nil,
		map[string]string{"Authorization": "Bot " + c.token},
	)
}

func (c *discordChannel) handleMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
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
			"⚠️ Discord 消息路由失败: "+truncateChannelError(err),
		)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err = ingress.Accept(ctx, request); err != nil {
		if isPairingApprovalRequired(err) {
			if notice := pairingApprovalNoticeText(err); notice != "" {
				_, _ = session.ChannelMessageSend(message.ChannelID, notice)
			}
			return
		}
		_, _ = session.ChannelMessageSend(
			message.ChannelID,
			"⚠️ Discord 消息处理失败: "+truncateChannelError(err),
		)
	}
}

func (c *discordChannel) buildIngressRequest(
	session *discordgo.Session,
	message *discordgo.MessageCreate,
	content string,
) (IngressRequest, error) {
	chatType := "group"
	ref := strings.TrimSpace(message.ChannelID)
	threadID := ""
	delivery := &DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeDiscord,
		To:      strings.TrimSpace(message.ChannelID),
	}

	if strings.TrimSpace(message.GuildID) == "" {
		chatType = "dm"
		ref = strings.TrimSpace(message.Author.ID)
		return IngressRequest{
			Channel:     ChannelTypeDiscord,
			OwnerUserID: c.ownerUserID,
			AccountID:   channelAccountIDFromSecret("dg", c.token),
			ChatType:    chatType,
			Ref:         ref,
			Content:     content,
			RoundID:     strings.TrimSpace(message.ID),
			ReqID:       strings.TrimSpace(message.ID),
			Delivery:    delivery,
			Message: channelmessage.NewInbound(channelmessage.InboundParams{
				Channel:           ChannelTypeDiscord,
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

	return IngressRequest{
		Channel:     ChannelTypeDiscord,
		OwnerUserID: c.ownerUserID,
		AccountID:   channelAccountIDFromSecret("dg", c.token),
		ChatType:    chatType,
		Ref:         ref,
		ThreadID:    threadID,
		Content:     content,
		RoundID:     strings.TrimSpace(message.ID),
		ReqID:       strings.TrimSpace(message.ID),
		Delivery:    delivery,
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           ChannelTypeDiscord,
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

func (c *discordChannel) resolveDiscordThreadRoute(session *discordgo.Session, channelID string) (string, string) {
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

func (c *discordChannel) currentIngress() IngressAcceptor {
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
