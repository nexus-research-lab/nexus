// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：agent_client_options.go
// @Date   ：2026/04/21 20:05:00
// @Author ：leemysw
// 2026/04/21 20:05:00   Create
// =====================================================

package runtime

import (
	"context"
	"strings"

	providercfg "github.com/nexus-research-lab/nexus/internal/provider"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

// RuntimeConfigResolver 负责解析 Agent 运行时环境。
type RuntimeConfigResolver interface {
	ResolveRuntimeConfig(context.Context, string) (*providercfg.RuntimeConfig, error)
}

// AgentClientOptionsInput 表示构造 SDK options 所需的统一输入。
type AgentClientOptionsInput struct {
	WorkspacePath      string
	Provider           string
	PermissionMode     sdkprotocol.PermissionMode
	PermissionHandler  agentclient.PermissionHandler
	AllowedTools       []string
	DisallowedTools    []string
	SettingSources     []string
	AppendSystemPrompt string
	ResumeSessionID    string
	MaxThinkingTokens  *int
	MaxTurns           *int
	MCPServers         map[string]agentclient.SDKMCPServer
}

// BuildAgentClientOptions 构建统一的 SDK client options。
func BuildAgentClientOptions(
	ctx context.Context,
	resolver RuntimeConfigResolver,
	input AgentClientOptionsInput,
) (agentclient.Options, error) {
	runtimeEnv, err := BuildRuntimeEnv(ctx, resolver, input.Provider)
	if err != nil {
		return agentclient.Options{}, err
	}

	permissionMode := input.PermissionMode
	if permissionMode == "" {
		permissionMode = sdkprotocol.PermissionModeDefault
	}

	options := agentclient.Options{
		CWD:                    strings.TrimSpace(input.WorkspacePath),
		PermissionMode:         permissionMode,
		AllowedTools:           append([]string(nil), input.AllowedTools...),
		DisallowedTools:        append([]string(nil), input.DisallowedTools...),
		SettingSources:         append([]string(nil), input.SettingSources...),
		IncludePartialMessages: true,
		Env:                    runtimeEnv,
		PermissionHandler:      input.PermissionHandler,
		AppendSystemPrompt:     input.AppendSystemPrompt,
	}
	if strings.TrimSpace(input.ResumeSessionID) != "" {
		options.Resume = strings.TrimSpace(input.ResumeSessionID)
	}
	if input.MaxThinkingTokens != nil && *input.MaxThinkingTokens > 0 {
		options.MaxThinkingTokens = *input.MaxThinkingTokens
	}
	if input.MaxTurns != nil && *input.MaxTurns > 0 {
		options.MaxTurns = *input.MaxTurns
	}
	if len(input.MCPServers) > 0 {
		options.SDKMCPServers = cloneMCPServers(input.MCPServers)
	}
	return options, nil
}

// BuildRuntimeEnv 统一把 provider 配置收口成 Claude SDK 所需环境变量。
func BuildRuntimeEnv(
	ctx context.Context,
	resolver RuntimeConfigResolver,
	provider string,
) (map[string]string, error) {
	if resolver == nil {
		return nil, nil
	}
	runtimeConfig, err := resolver.ResolveRuntimeConfig(ctx, strings.TrimSpace(provider))
	if err != nil {
		return nil, err
	}
	if runtimeConfig == nil {
		return nil, nil
	}
	env := map[string]string{
		"ANTHROPIC_AUTH_TOKEN":           runtimeConfig.AuthToken,
		"ANTHROPIC_BASE_URL":             runtimeConfig.BaseURL,
		"ANTHROPIC_MODEL":                runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_SONNET_MODEL": runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  runtimeConfig.Model,
		"CLAUDE_CODE_SUBAGENT_MODEL":     runtimeConfig.Model,
	}
	if strings.Contains(strings.ToLower(runtimeConfig.Model), "kimi") {
		env["ENABLE_TOOL_SEARCH"] = "false"
	}
	return env, nil
}

func cloneMCPServers(
	current map[string]agentclient.SDKMCPServer,
) map[string]agentclient.SDKMCPServer {
	if len(current) == 0 {
		return nil
	}
	result := make(map[string]agentclient.SDKMCPServer, len(current))
	for key, value := range current {
		result[key] = value
	}
	return result
}
