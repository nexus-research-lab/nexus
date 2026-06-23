package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type feishuTenantTokenEnvelope struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"`
}

func (c *FeishuChannel) tenantAccessToken(ctx context.Context) (string, error) {
	if c.appID == "" || c.appSecret == "" {
		return "", fmt.Errorf("feishu channel is not configured")
	}
	now := time.Now()
	c.mu.Lock()
	if c.tenantToken != "" && now.Before(c.tokenExpiresAt) {
		token := c.tenantToken
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	token, expiresAt, err := c.fetchTenantAccessToken(ctx, now)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.tenantToken = token
	c.tokenExpiresAt = expiresAt
	c.mu.Unlock()
	return token, nil
}

func (c *FeishuChannel) fetchTenantAccessToken(ctx context.Context, now time.Time) (string, time.Time, error) {
	payload, err := json.Marshal(map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	})
	if err != nil {
		return "", time.Time{}, err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/open-apis/auth/v3/tenant_access_token/internal",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", time.Time{}, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return "", time.Time{}, err
	}
	var envelope feishuTenantTokenEnvelope
	if err = decodeFeishuEnvelope(response, &envelope); err != nil {
		return "", time.Time{}, err
	}
	if envelope.Code != 0 {
		return "", time.Time{}, fmt.Errorf("feishu tenant_access_token failed: code=%d msg=%s", envelope.Code, strings.TrimSpace(envelope.Msg))
	}
	token := strings.TrimSpace(envelope.TenantAccessToken)
	if token == "" {
		return "", time.Time{}, fmt.Errorf("feishu tenant_access_token returned empty token")
	}
	expiresIn := envelope.Expire
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	if expiresIn > 600 {
		expiresIn -= 300
	}
	return token, now.Add(time.Duration(expiresIn) * time.Second), nil
}

func (c *FeishuChannel) clearTenantAccessToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tenantToken = ""
	c.tokenExpiresAt = time.Time{}
}
