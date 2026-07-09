package types

import (
	"errors"
	"strings"
	"time"
)

// HeartbeatConfig 表示持久化 heartbeat 配置。
type HeartbeatConfig struct {
	AgentID      string `json:"agent_id"`
	Enabled      bool   `json:"enabled"`
	EverySeconds int    `json:"every_seconds"`
	TargetMode   string `json:"target_mode"`
	AckMaxChars  int    `json:"ack_max_chars"`
}

// Validate 校验配置。
func (c HeartbeatConfig) Validate() error {
	if strings.TrimSpace(c.AgentID) == "" {
		return errors.New("agent_id is required")
	}
	if c.EverySeconds <= 0 {
		return errors.New("every_seconds must be greater than 0")
	}
	if c.AckMaxChars < 0 {
		return errors.New("ack_max_chars must be greater than or equal to 0")
	}
	switch strings.TrimSpace(c.TargetMode) {
	case "", HeartbeatTargetNone, HeartbeatTargetLast, HeartbeatTargetExplicit:
		return nil
	default:
		return ErrHeartbeatConfigInvalid
	}
}

// Normalized 返回带默认值的配置副本。
func (c HeartbeatConfig) Normalized() HeartbeatConfig {
	result := c
	result.AgentID = strings.TrimSpace(result.AgentID)
	if result.EverySeconds <= 0 {
		result.EverySeconds = 1800
	}
	if result.AckMaxChars < 0 {
		result.AckMaxChars = 300
	}
	result.TargetMode = strings.TrimSpace(result.TargetMode)
	if result.TargetMode == "" {
		result.TargetMode = HeartbeatTargetNone
	}
	return result
}

// DefaultHeartbeatConfig 返回默认 heartbeat 配置。
func DefaultHeartbeatConfig(agentID string) HeartbeatConfig {
	return HeartbeatConfig{
		AgentID:      strings.TrimSpace(agentID),
		Enabled:      false,
		EverySeconds: 1800,
		TargetMode:   HeartbeatTargetNone,
		AckMaxChars:  300,
	}
}

// HeartbeatStatus 表示运行态和配置快照。
type HeartbeatStatus struct {
	AgentID         string     `json:"agent_id"`
	Enabled         bool       `json:"enabled"`
	EverySeconds    int        `json:"every_seconds"`
	TargetMode      string     `json:"target_mode"`
	AckMaxChars     int        `json:"ack_max_chars"`
	Running         bool       `json:"running"`
	PendingWake     bool       `json:"pending_wake"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	LastAckAt       *time.Time `json:"last_ack_at,omitempty"`
	DeliveryError   *string    `json:"delivery_error,omitempty"`
}

// HeartbeatWakeResult 表示手动唤醒返回。
type HeartbeatWakeResult struct {
	AgentID   string `json:"agent_id"`
	Mode      string `json:"mode"`
	Scheduled bool   `json:"scheduled"`
}

// HeartbeatUpdateInput 表示 heartbeat 配置更新请求。
type HeartbeatUpdateInput struct {
	Enabled      bool   `json:"enabled"`
	EverySeconds int    `json:"every_seconds"`
	TargetMode   string `json:"target_mode"`
	AckMaxChars  int    `json:"ack_max_chars"`
}

// HeartbeatWakeInput 表示唤醒请求。
type HeartbeatWakeInput struct {
	Mode string  `json:"mode"`
	Text *string `json:"text,omitempty"`
}

// SystemEvent 表示 heartbeat/main-session 消费的系统事件。
type SystemEvent struct {
	EventID    string
	EventType  string
	SourceType string
	SourceID   string
	Payload    string
	Status     string
	CreatedAt  time.Time
}
