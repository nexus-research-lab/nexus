package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
)

type dingTalkAccessTokenEnvelope struct {
	AccessToken string `json:"accessToken"`
	ExpireIn    int    `json:"expireIn"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

type dingTalkAPIEnvelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestid"`
}

func (c *DingTalkChannel) SendDeliveryMessage(ctx context.Context, target channelcontract.DeliveryTarget, text string) (channelcontract.DeliveryResult, error) {
	normalized := target.Normalized()
	if strings.TrimSpace(target.To) == "" {
		return channelcontract.DeliveryResult{}, fmt.Errorf("dingtalk delivery target requires to")
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(target.To)), "http://") ||
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(target.To)), "https://") {
		if err := c.sendSessionWebhookText(ctx, target.To, text); err != nil {
			return channelcontract.DeliveryResult{}, err
		}
		return channelcontract.NewDeliveryResult(normalized, nil), nil
	}
	if strings.TrimSpace(c.robotCode) == "" {
		return channelcontract.DeliveryResult{}, fmt.Errorf("dingtalk delivery requires robot_code")
	}
	token, err := c.accessTokenForDelivery(ctx)
	if err != nil {
		return channelcontract.DeliveryResult{}, err
	}
	for _, chunk := range channeltransport.SplitText(strings.TrimSpace(text), 3800) {
		if err = c.sendGroupTextChunk(ctx, token, target.To, chunk); err != nil {
			c.clearAccessToken()
			return channelcontract.DeliveryResult{}, err
		}
	}
	return channelcontract.NewDeliveryResult(normalized, nil), nil
}

func (c *DingTalkChannel) accessTokenForDelivery(ctx context.Context) (string, error) {
	if c.clientID == "" || c.clientSecret == "" {
		return "", fmt.Errorf("dingtalk channel is not configured")
	}
	now := time.Now()
	c.mu.RLock()
	if c.accessToken != "" && now.Before(c.tokenExpiresAt) {
		token := c.accessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	result, err, _ := c.tokenFlight.Do("refresh", func() (any, error) {
		return c.refreshAccessToken(ctx, now)
	})
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

func (c *DingTalkChannel) refreshAccessToken(ctx context.Context, now time.Time) (string, error) {
	// singleflight 获胜方在真正刷新前再检查一次，避免等待期间已有 goroutine 写入新 token。
	c.mu.RLock()
	if c.accessToken != "" && now.Before(c.tokenExpiresAt) {
		token := c.accessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	payload, err := json.Marshal(map[string]string{
		"appKey":    c.clientID,
		"appSecret": c.clientSecret,
	})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/v1.0/oauth2/accessToken",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return "", err
	}
	var envelope dingTalkAccessTokenEnvelope
	if err = decodeDingTalkEnvelope(response, &envelope); err != nil {
		return "", err
	}
	if strings.TrimSpace(envelope.Code) != "" {
		return "", fmt.Errorf("dingtalk accessToken failed: code=%s message=%s", envelope.Code, strings.TrimSpace(envelope.Message))
	}
	token := strings.TrimSpace(envelope.AccessToken)
	if token == "" {
		return "", fmt.Errorf("dingtalk accessToken returned empty token")
	}
	expiresIn := envelope.ExpireIn
	if expiresIn <= 0 {
		expiresIn = 7200
	}
	if expiresIn > 600 {
		expiresIn -= 300
	}
	c.mu.Lock()
	c.accessToken = token
	c.tokenExpiresAt = now.Add(time.Duration(expiresIn) * time.Second)
	c.mu.Unlock()
	return token, nil
}

func (c *DingTalkChannel) sendGroupTextChunk(ctx context.Context, token string, conversationID string, text string) error {
	msgParam, err := json.Marshal(map[string]string{"content": text})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{
		"robotCode":          c.robotCode,
		"openConversationId": strings.TrimSpace(conversationID),
		"msgKey":             "sampleText",
		"msgParam":           string(msgParam),
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/v1.0/robot/groupMessages/send",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-acs-dingtalk-access-token", strings.TrimSpace(token))

	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	var envelope dingTalkAPIEnvelope
	if err = decodeDingTalkEnvelope(response, &envelope); err != nil {
		return err
	}
	if strings.TrimSpace(envelope.Code) != "" {
		return fmt.Errorf("dingtalk send message failed: code=%s message=%s", envelope.Code, strings.TrimSpace(envelope.Message))
	}
	return nil
}

func (c *DingTalkChannel) sendSessionWebhookText(ctx context.Context, webhook string, text string) error {
	for _, chunk := range channeltransport.SplitText(strings.TrimSpace(text), 3800) {
		payload, err := json.Marshal(map[string]any{
			"msgtype": "text",
			"text": map[string]string{
				"content": chunk,
			},
		})
		if err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(webhook), bytes.NewReader(payload))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := c.client.Do(request)
		if err != nil {
			return err
		}
		if err = channeltransport.ExpectSuccess(response); err != nil {
			return err
		}
	}
	return nil
}

func (c *DingTalkChannel) clearAccessToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = ""
	c.tokenExpiresAt = time.Time{}
}

func decodeDingTalkEnvelope(response *http.Response, target any) error {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("dingtalk request failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	if err = json.Unmarshal(body, target); err != nil {
		return err
	}
	return nil
}
