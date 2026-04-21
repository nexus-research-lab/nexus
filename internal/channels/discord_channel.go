package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type discordChannel struct {
	token   string
	client  *http.Client
	baseURL string

	mu      sync.RWMutex
	ingress IngressAcceptor
	session *discordgo.Session
}

func newDiscordChannel(token string, client *http.Client) *discordChannel {
	if client == nil {
		client = http.DefaultClient
	}
	return &discordChannel{
		token:   strings.TrimSpace(token),
		client:  client,
		baseURL: "https://discord.com/api/v10",
	}
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

func (c *discordChannel) SendDeliveryText(ctx context.Context, target DeliveryTarget, text string) error {
	if strings.TrimSpace(c.token) == "" {
		return fmt.Errorf("discord channel is not configured")
	}
	targetID := firstNonEmpty(target.ThreadID, target.To)
	if targetID == "" {
		return fmt.Errorf("discord delivery target requires to or thread_id")
	}

	for _, chunk := range splitText(strings.TrimSpace(text), 1900) {
		payload, err := json.Marshal(map[string]any{
			"content": chunk,
		})
		if err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			strings.TrimRight(c.baseURL, "/")+"/channels/"+targetID+"/messages",
			bytes.NewReader(payload),
		)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bot "+c.token)
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
			Channel:  ChannelTypeDiscord,
			ChatType: chatType,
			Ref:      ref,
			Content:  content,
			Delivery: delivery,
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
		Channel:  ChannelTypeDiscord,
		ChatType: chatType,
		Ref:      ref,
		ThreadID: threadID,
		Content:  content,
		Delivery: delivery,
	}, nil
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

func expectSuccess(response *http.Response) error {
	defer response.Body.Close()
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("delivery request failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(body)))
}

func splitText(text string, limit int) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return []string{text}
	}

	result := make([]string, 0, len(runes)/limit+1)
	for start := 0; start < len(runes); start += limit {
		end := start + limit
		if end > len(runes) {
			end = len(runes)
		}
		result = append(result, string(runes[start:end]))
	}
	return result
}
