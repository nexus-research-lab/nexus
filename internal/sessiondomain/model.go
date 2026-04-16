// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：model.go
// @Date   ：2026/04/10 23:58:00
// @Author ：leemysw
// 2026/04/10 23:58:00   Create
// =====================================================

package sessiondomain

import "time"

// Session 表示对外暴露的统一会话模型。
type Session struct {
	SessionKey     string         `json:"session_key"`
	AgentID        string         `json:"agent_id"`
	SessionID      *string        `json:"session_id"`
	RoomSessionID  *string        `json:"room_session_id"`
	RoomID         *string        `json:"room_id"`
	ConversationID *string        `json:"conversation_id"`
	ChannelType    string         `json:"channel_type"`
	ChatType       string         `json:"chat_type"`
	Status         string         `json:"status"`
	CreatedAt      time.Time      `json:"created_at"`
	LastActivity   time.Time      `json:"last_activity"`
	Title          string         `json:"title"`
	MessageCount   int            `json:"message_count"`
	Options        map[string]any `json:"options"`
	IsActive       bool           `json:"is_active"`
}

// Message 表示历史消息行。
type Message map[string]any

// CostSummary 表示 Session 维度成本汇总。
type CostSummary struct {
	AgentID                        string    `json:"agent_id"`
	SessionKey                     string    `json:"session_key"`
	SessionID                      string    `json:"session_id"`
	TotalInputTokens               int       `json:"total_input_tokens"`
	TotalOutputTokens              int       `json:"total_output_tokens"`
	TotalTokens                    int       `json:"total_tokens"`
	TotalCacheCreationInputTokens  int       `json:"total_cache_creation_input_tokens"`
	TotalCacheReadInputTokens      int       `json:"total_cache_read_input_tokens"`
	TotalCostUSD                   float64   `json:"total_cost_usd"`
	CompletedRounds                int       `json:"completed_rounds"`
	ErrorRounds                    int       `json:"error_rounds"`
	LastRoundID                    *string   `json:"last_round_id"`
	LastRunDurationMS              *int      `json:"last_run_duration_ms"`
	LastRunCostUSD                 *float64  `json:"last_run_cost_usd"`
	UpdatedAt                      time.Time `json:"updated_at"`
}

// AgentCostSummary 表示 Agent 维度成本汇总。
type AgentCostSummary struct {
	AgentID                       string    `json:"agent_id"`
	TotalInputTokens              int       `json:"total_input_tokens"`
	TotalOutputTokens             int       `json:"total_output_tokens"`
	TotalTokens                   int       `json:"total_tokens"`
	TotalCacheCreationInputTokens int       `json:"total_cache_creation_input_tokens"`
	TotalCacheReadInputTokens     int       `json:"total_cache_read_input_tokens"`
	TotalCostUSD                  float64   `json:"total_cost_usd"`
	CompletedRounds               int       `json:"completed_rounds"`
	ErrorRounds                   int       `json:"error_rounds"`
	CostSessions                  int       `json:"cost_sessions"`
	UpdatedAt                     time.Time `json:"updated_at"`
}

