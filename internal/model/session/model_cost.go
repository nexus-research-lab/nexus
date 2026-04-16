// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：model_cost.go
// @Date   ：2026/04/16 22:18:54
// @Author ：leemysw
// 2026/04/16 22:18:54   Create
// =====================================================

package session

import "time"

// CostSummary 表示 Session 维度成本汇总。
type CostSummary struct {
	AgentID                       string    `json:"agent_id"`
	SessionKey                    string    `json:"session_key"`
	SessionID                     string    `json:"session_id"`
	TotalInputTokens              int       `json:"total_input_tokens"`
	TotalOutputTokens             int       `json:"total_output_tokens"`
	TotalTokens                   int       `json:"total_tokens"`
	TotalCacheCreationInputTokens int       `json:"total_cache_creation_input_tokens"`
	TotalCacheReadInputTokens     int       `json:"total_cache_read_input_tokens"`
	TotalCostUSD                  float64   `json:"total_cost_usd"`
	CompletedRounds               int       `json:"completed_rounds"`
	ErrorRounds                   int       `json:"error_rounds"`
	LastRoundID                   *string   `json:"last_round_id"`
	LastRunDurationMS             *int      `json:"last_run_duration_ms"`
	LastRunCostUSD                *float64  `json:"last_run_cost_usd"`
	UpdatedAt                     time.Time `json:"updated_at"`
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
