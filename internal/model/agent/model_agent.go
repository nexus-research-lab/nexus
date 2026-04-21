// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：model_agent.go
// @Date   ：2026/04/16 22:18:54
// @Author ：leemysw
// 2026/04/16 22:18:54   Create
// =====================================================

package agent

import "time"

// Options 表示 Agent 运行时配置。
type Options struct {
	Provider          string         `json:"provider,omitempty"`
	PermissionMode    string         `json:"permission_mode,omitempty"`
	AllowedTools      []string       `json:"allowed_tools,omitempty"`
	DisallowedTools   []string       `json:"disallowed_tools,omitempty"`
	MaxTurns          *int           `json:"max_turns,omitempty"`
	MaxThinkingTokens *int           `json:"max_thinking_tokens,omitempty"`
	MCPServers        map[string]any `json:"mcp_servers,omitempty"`
	SettingSources    []string       `json:"setting_sources,omitempty"`
}

// Agent 表示对外 Agent 模型。
type Agent struct {
	AgentID         string    `json:"agent_id"`
	Name            string    `json:"name"`
	WorkspacePath   string    `json:"workspace_path"`
	DisplayName     string    `json:"display_name,omitempty"`
	Headline        string    `json:"headline,omitempty"`
	ProfileMarkdown string    `json:"profile_markdown,omitempty"`
	Options         Options   `json:"options"`
	CreatedAt       time.Time `json:"created_at"`
	Status          string    `json:"status"`
	Avatar          string    `json:"avatar,omitempty"`
	Description     string    `json:"description,omitempty"`
	VibeTags        []string  `json:"vibe_tags,omitempty"`
	SkillsCount     int       `json:"skills_count"`
}

// CreateRequest 表示创建 Agent 请求。
type CreateRequest struct {
	Name        string   `json:"name"`
	Options     *Options `json:"options,omitempty"`
	Avatar      string   `json:"avatar,omitempty"`
	Description string   `json:"description,omitempty"`
	VibeTags    []string `json:"vibe_tags,omitempty"`
}

// UpdateRequest 表示更新 Agent 请求。
type UpdateRequest struct {
	Name        *string  `json:"name,omitempty"`
	Options     *Options `json:"options,omitempty"`
	Avatar      *string  `json:"avatar,omitempty"`
	Description *string  `json:"description,omitempty"`
	VibeTags    []string `json:"vibe_tags,omitempty"`
}

// ValidateNameResponse 对齐当前校验协议。
type ValidateNameResponse struct {
	Name           string `json:"name"`
	NormalizedName string `json:"normalized_name"`
	IsValid        bool   `json:"is_valid"`
	IsAvailable    bool   `json:"is_available"`
	WorkspacePath  string `json:"workspace_path,omitempty"`
	Reason         string `json:"reason,omitempty"`
}
