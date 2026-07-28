// INPUT: 平台官方应用注册端点、二维码设备码与轮询响应。
// OUTPUT: 供 IM 与 Connector 复用的统一注册开始、状态和凭据模型。
// POS: appregistration 的跨平台协议边界，不承载业务存储或运行时装配。
package appregistration

import (
	"context"
	"net/http"
	"time"
)

// Status is the normalized state returned by a platform registration poll.
type Status string

const (
	StatusPending   Status = "pending"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusExpired   Status = "expired"
	StatusSlowDown  Status = "slow_down"
)

// StartResult contains the URL rendered as a QR code and the opaque polling key.
type StartResult struct {
	DeviceCode              string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	ExpiresIn               int
	Interval                int
}

// PollResult contains normalized completion state and platform credentials.
type PollResult struct {
	Status      Status
	Credentials map[string]string
	UserID      string
	Message     string
}

// Client is implemented by each official QR registration protocol.
type Client interface {
	Start(context.Context) (*StartResult, error)
	Poll(context.Context, string) (*PollResult, error)
}

func effectiveHTTPClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: 20 * time.Second}
}
