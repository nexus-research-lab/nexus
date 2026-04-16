// =====================================================
// @File   ：service_ready.go
// @Date   ：2026/04/16 13:44:49
// @Author ：leemysw
// 2026/04/16 13:44:49   Create
// =====================================================

package agent

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
)

// EnsureReady 确保主智能体和 workspace 根目录存在。
func (s *Service) EnsureReady(ctx context.Context) error {
	s.once.Do(func() {
		s.readyErr = s.ensureReady(ctx)
	})
	return s.readyErr
}

func (s *Service) ensureReady(ctx context.Context) error {
	workspaceBase := WorkspaceBasePath(s.config)
	if err := os.MkdirAll(workspaceBase, 0o755); err != nil {
		return err
	}

	agent, err := s.repository.GetAgent(ctx, s.config.DefaultAgentID)
	if err != nil {
		return err
	}
	if agent != nil && agent.Status == "active" {
		if err = s.syncMainAgentRuntime(ctx, agent); err != nil {
			return err
		}
		return os.MkdirAll(agent.WorkspacePath, 0o755)
	}

	record := BuildDefaultMainAgentRecord(s.config)
	if err := os.MkdirAll(record.WorkspacePath, 0o755); err != nil {
		return err
	}

	created, err := s.repository.CreateAgent(ctx, record)
	if err == nil {
		return os.MkdirAll(created.WorkspacePath, 0o755)
	}
	if !strings.Contains(err.Error(), "UNIQUE") && !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		return err
	}

	agent, err = s.repository.GetAgent(ctx, s.config.DefaultAgentID)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("主智能体初始化失败: %s", s.config.DefaultAgentID)
	}
	return os.MkdirAll(agent.WorkspacePath, 0o755)
}

func (s *Service) syncMainAgentRuntime(ctx context.Context, agent *Agent) error {
	if agent == nil {
		return nil
	}
	defaultOptions := defaultMainAgentOptions()
	if !mainAgentRuntimeNeedsSync(agent.Options, defaultOptions) {
		return nil
	}
	_, err := s.repository.UpdateAgent(ctx, UpdateRecord{
		AgentID:             agent.AgentID,
		Slug:                BuildWorkspaceDirName(agent.Name),
		Name:                agent.Name,
		WorkspacePath:       agent.WorkspacePath,
		Avatar:              agent.Avatar,
		Description:         agent.Description,
		VibeTagsJSON:        mustJSONString(agent.VibeTags, "[]"),
		Provider:            "",
		PermissionMode:      defaultOptions.PermissionMode,
		AllowedToolsJSON:    mustJSONString(defaultOptions.AllowedTools, "[]"),
		DisallowedToolsJSON: mustJSONString(agent.Options.DisallowedTools, "[]"),
		MCPServersJSON:      mustJSONString(agent.Options.MCPServers, "{}"),
		MaxTurns:            agent.Options.MaxTurns,
		MaxThinkingTokens:   agent.Options.MaxThinkingTokens,
		SettingSourcesJSON:  mustJSONString(defaultOptions.SettingSources, "[]"),
	})
	return err
}

func mainAgentRuntimeNeedsSync(current Options, expected Options) bool {
	if strings.TrimSpace(current.Provider) != "" {
		return true
	}
	if strings.TrimSpace(current.PermissionMode) != expected.PermissionMode {
		return true
	}
	if !slices.Equal(current.AllowedTools, expected.AllowedTools) {
		return true
	}
	return !slices.Equal(current.SettingSources, expected.SettingSources)
}
