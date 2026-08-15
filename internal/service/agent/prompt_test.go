package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/config"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

func TestServiceBuildRuntimePromptIncludesWorkspaceFilesAndProfile(t *testing.T) {
	workspacePath := t.TempDir()
	writePromptFile(t, workspacePath, "AGENTS.md", "# AGENTS.md\n\n执行规则：必须先读 AGENTS。")
	writePromptFile(t, workspacePath, "USER.md", "# USER.md\n\n用户偏好：默认中文。")
	writePromptFile(t, workspacePath, "MEMORY.md", "# MEMORY.md\n\n长期约束：不要改路径。")

	service := agentsvc.NewService(config.Config{
		DefaultAgentID:        "nexus",
		BaseSystemPrompt:      "BASE CUSTOM PROMPT",
		MainAgentSystemPrompt: "MAIN CUSTOM PROMPT",
	}, nil)

	prompt, err := service.BuildRuntimePrompt(context.Background(), &protocol.Agent{
		AgentID:         "agent-1",
		Name:            "planner",
		DisplayName:     "规划助手",
		Headline:        "擅长任务拆解",
		ProfileMarkdown: "## 详细档案\n- 偏好明确目标与验收标准。",
		Description:     "补充说明",
		VibeTags:        []string{"严谨", "任务拆解"},
		WorkspacePath:   workspacePath,
	})
	if err != nil {
		t.Fatalf("构建运行时提示词失败: %v", err)
	}

	assertPromptContains(t, prompt, "BASE CUSTOM PROMPT")
	assertPromptContains(t, prompt, "Mode: single-user system scope")
	assertPromptContains(t, prompt, "use $TMPDIR for private data")
	assertPromptContains(t, prompt, "/tmp is a shared compatibility directory")
	assertPromptContains(t, prompt, "## Agent Identity")
	assertPromptContains(t, prompt, "Identity: planner (agent-1)")
	assertPromptContains(t, prompt, "WORKING DIRECTORY: "+workspacePath)
	assertPromptContains(t, prompt, "执行规则：必须先读 AGENTS。")
	assertPromptContains(t, prompt, "用户偏好：默认中文。")
	assertPromptContains(t, prompt, "Description: 补充说明")
	assertPromptContains(t, prompt, "Vibe Tags: 严谨, 任务拆解")
	assertPromptContains(t, prompt, "## Agent Identity\nIdentity: planner (agent-1)\nWORKING DIRECTORY: "+workspacePath+"\n\n---\n\n## Agent Profile\nDescription: 补充说明")
	if strings.Contains(prompt, "长期约束：不要改路径。") {
		t.Fatalf("产品侧 prompt 不应直接加载 MEMORY.md: %s", prompt)
	}
	if strings.Contains(prompt, "规划助手") || strings.Contains(prompt, "擅长任务拆解") || strings.Contains(prompt, "偏好明确目标与验收标准") {
		t.Fatalf("运行时 prompt 不应注入旧 profiles 表展示字段: %s", prompt)
	}
	if strings.Contains(prompt, "最近日记提醒") {
		t.Fatalf("运行时 system prompt 不应无条件注入近期动态记忆: %s", prompt)
	}
}

func TestServiceBuildRuntimeUserMessageSuffixIncludesEmotionWhenEnabled(t *testing.T) {
	service := agentsvc.NewService(config.Config{
		DefaultAgentID:   "nexus",
		DefaultTimezone:  "Asia/Shanghai",
		BaseSystemPrompt: "BASE CUSTOM PROMPT",
	}, nil)

	suffix := service.BuildRuntimeUserMessageSuffixForContext(context.Background(), &protocol.Agent{
		AgentID:     "agent-1",
		Name:        "planner",
		DisplayName: "规划助手",
	}, "", true)

	// 时间不再由本层注入（交给 runtime 基础提示），避免秒级时间戳污染前缀缓存。
	assertPromptContains(t, suffix, "<nexus_runtime_context>")
	assertPromptContains(t, suffix, "## Emotion State")
	assertPromptContains(t, suffix, "Base: focused (energy 6/10, valence 6/10) - clear, proactive, concise")
	assertPromptContains(t, suffix, "Composite: focused (energy 6/10, valence 6/10) - clear, proactive, concise")
	assertPromptContains(t, suffix, "</nexus_runtime_context>")
	if disabled := service.BuildRuntimeUserMessageSuffixForContext(
		context.Background(),
		&protocol.Agent{AgentID: "agent-1", Name: "planner"},
		"",
		false,
	); disabled != "" {
		t.Fatalf("关闭情绪系统后不应注入动态上下文: %q", disabled)
	}
}

func TestServiceBuildRuntimeUserMessageSuffixReadsAgentEmotionState(t *testing.T) {
	workspacePath := t.TempDir()
	statePath := filepath.Join(workspacePath, ".agents", "emotion.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("创建情绪状态目录失败: %v", err)
	}
	if err := os.WriteFile(statePath, []byte(`{
  "base": {
    "mood": "playful",
    "energy": 8,
    "valence": 8,
    "description": "curious and warm"
  },
  "contexts": {
    "dm:test": {
      "mood": "annoyed",
      "valence": 4,
      "trigger": "user said the draft feels wrong"
    }
  },
  "fatigue": {
    "status": "awake",
    "level": 10
  }
}
`), 0o644); err != nil {
		t.Fatalf("写入情绪状态失败: %v", err)
	}

	service := agentsvc.NewService(config.Config{
		DefaultAgentID:   "nexus",
		DefaultTimezone:  "Asia/Shanghai",
		BaseSystemPrompt: "BASE CUSTOM PROMPT",
	}, nil)

	suffix := service.BuildRuntimeUserMessageSuffixForContext(context.Background(), &protocol.Agent{
		AgentID:       "agent-1",
		Name:          "runner",
		WorkspacePath: workspacePath,
	}, "dm:test", true)

	assertPromptContains(t, suffix, "Base: playful (energy 8/10, valence 8/10) - curious and warm")
	assertPromptContains(t, suffix, "Context: annoyed (valence 4/10) - user said the draft feels wrong")
	assertPromptContains(t, suffix, "Composite: annoyed (energy 8/10, valence 6/10) - user said the draft feels wrong")
	assertPromptContains(t, suffix, "Fatigue: awake (10/100)")
}

func TestLoadRuntimeEmotionViewIgnoresLegacyEmotionShape(t *testing.T) {
	workspacePath := t.TempDir()
	statePath := filepath.Join(workspacePath, ".agents", "emotion.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("创建情绪状态目录失败: %v", err)
	}
	if err := os.WriteFile(statePath, []byte(`{"mood":"playful","energy":9,"summary":"old shape"}`), 0o644); err != nil {
		t.Fatalf("写入 legacy 情绪状态失败: %v", err)
	}

	view := agentsvc.LoadRuntimeEmotionView(workspacePath, "", time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC))
	if view.Base.Mood != "focused" || view.Base.Description != "clear, proactive, concise" {
		t.Fatalf("legacy emotion shape should fall back to default state, got %+v", view.Base)
	}
}

func TestServiceBuildRuntimePromptDirectsGoalSkill(t *testing.T) {
	workspacePath := t.TempDir()
	skillPath := filepath.Join(workspacePath, ".agents", "skills", "goal-manager")
	if err := os.MkdirAll(skillPath, 0o755); err != nil {
		t.Fatalf("创建 goal-manager 目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("---\nname: goal-manager\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatalf("写入 goal-manager 失败: %v", err)
	}

	service := agentsvc.NewService(config.Config{
		DefaultAgentID:   "nexus",
		BaseSystemPrompt: "BASE CUSTOM PROMPT",
	}, nil)

	prompt, err := service.BuildRuntimePrompt(context.Background(), &protocol.Agent{
		AgentID:       "agent-1",
		Name:          "planner",
		WorkspacePath: workspacePath,
	})
	if err != nil {
		t.Fatalf("构建运行时提示词失败: %v", err)
	}

	assertPromptContains(t, prompt, "Goal Skill 使用要求")
	assertPromptContains(t, prompt, "goal-manager")
	assertPromptContains(t, prompt, "mcp__nexus_goal__get_goal")
	assertPromptContains(t, prompt, "mcp__nexus_goal__create_goal")
	assertPromptContains(t, prompt, "mcp__nexus_goal__retarget_goal")
	assertPromptContains(t, prompt, "mcp__nexus_goal__audit_objective_alignment")
	assertPromptContains(t, prompt, "mcp__nexus_goal__update_goal")
}

func TestServiceBuildRuntimePromptOmitsDisabledGoalSkillGuidance(t *testing.T) {
	workspacePath := t.TempDir()
	skillPath := filepath.Join(workspacePath, ".agents", "skills", "goal-manager")
	if err := os.MkdirAll(skillPath, 0o755); err != nil {
		t.Fatalf("创建 goal-manager 目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("---\nname: goal-manager\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatalf("写入 goal-manager 失败: %v", err)
	}

	service := agentsvc.NewService(config.Config{
		DefaultAgentID:   "nexus",
		BaseSystemPrompt: "BASE CUSTOM PROMPT",
	}, nil)
	prompt, err := service.BuildRuntimePrompt(context.Background(), &protocol.Agent{
		AgentID:       "agent-1",
		Name:          "planner",
		WorkspacePath: workspacePath,
		Options: protocol.Options{
			SkillIDs:         []string{"goal-manager"},
			DisabledSkillIDs: []string{"goal-manager"},
		},
	})
	if err != nil {
		t.Fatalf("构建运行时提示词失败: %v", err)
	}
	if strings.Contains(prompt, "Goal Skill 使用要求") {
		t.Fatalf("显式停用 goal-manager 后仍注入使用要求: %q", prompt)
	}
}

func TestServiceBuildRuntimePromptUsesMainAgentPromptOverride(t *testing.T) {
	workspacePath := t.TempDir()
	writePromptFile(t, workspacePath, "AGENTS.md", "# AGENTS.md\n\n主智能体规则。")
	writePromptFile(t, workspacePath, "SOUL.md", "# SOUL.md\n\n主智能体人格外置规则。")
	writePromptFile(t, workspacePath, "TOOLS.md", "# TOOLS.md\n\n主智能体工具外置规则。")

	service := agentsvc.NewService(config.Config{
		DefaultAgentID:        "nexus",
		BaseSystemPrompt:      "BASE CUSTOM PROMPT",
		MainAgentSystemPrompt: "MAIN CUSTOM PROMPT",
	}, nil)

	prompt, err := service.BuildRuntimePrompt(context.Background(), &protocol.Agent{
		AgentID:       "nexus",
		Name:          "nexus",
		WorkspacePath: workspacePath,
	})
	if err != nil {
		t.Fatalf("构建主智能体运行时提示词失败: %v", err)
	}
	if strings.Contains(prompt, "BASE CUSTOM PROMPT") {
		t.Fatalf("主智能体提示词不应回退到基础 prompt: %s", prompt)
	}
	assertPromptContains(t, prompt, "MAIN CUSTOM PROMPT")
	assertPromptContains(t, prompt, "Identity: nexus (nexus)")
	assertPromptContains(t, prompt, "WORKING DIRECTORY: "+workspacePath)
	if strings.Contains(prompt, "主智能体规则") {
		t.Fatalf("主智能体不应从 AGENTS.md 加载可见提示词: %s", prompt)
	}
	if strings.Contains(prompt, "主智能体人格外置规则") || strings.Contains(prompt, "主智能体工具外置规则") {
		t.Fatalf("主智能体不应从 SOUL.md/TOOLS.md 加载可见提示词: %s", prompt)
	}
}

func TestServiceBuildRuntimePromptIncludesMainAgentDefaultPolicy(t *testing.T) {
	workspacePath := t.TempDir()
	writePromptFile(t, workspacePath, "USER.md", "# USER.md\n\nsetup_status: configured\n\n- Preferred language: Chinese")
	writePromptFile(t, workspacePath, "MEMORY.md", "# MEMORY.md\n\n- Prefer restoring existing Rooms before creating duplicates.")
	writePromptFile(t, workspacePath, "AGENTS.md", "# AGENTS.md\n\n主智能体可见规则。")
	writePromptFile(t, workspacePath, "SOUL.md", "# SOUL.md\n\n主智能体外置人格。")
	writePromptFile(t, workspacePath, "TOOLS.md", "# TOOLS.md\n\n主智能体外置工具。")

	service := agentsvc.NewService(config.Config{
		DefaultAgentID: "nexus",
	}, nil)

	prompt, err := service.BuildRuntimePrompt(context.Background(), &protocol.Agent{
		AgentID:       "nexus",
		Name:          "nexus",
		IsMain:        true,
		WorkspacePath: workspacePath,
	})
	if err != nil {
		t.Fatalf("构建主智能体默认提示词失败: %v", err)
	}

	assertPromptContains(t, prompt, "You are Nexus — not an assistant, not a chatbot")
	assertPromptContains(t, prompt, "You coordinate from the main chat, but you are not a Room member")
	assertPromptContains(t, prompt, "Before creating durable structure, check for an existing Room, DM, member, file, or scheduled task")
	assertPromptContains(t, prompt, "Use `nexus-manager` for Nexus user accounts, members, Rooms, DMs, workspaces, and skills")
	assertPromptContains(t, prompt, "use the round-scoped `nexuscfg`")
	assertPromptContains(t, prompt, "Configuration changes follow one workflow")
	assertPromptContains(t, prompt, "account registration, user listing, and password resets")
	assertPromptContains(t, prompt, "the host-injected current owner and workspace are authoritative")
	assertPromptContains(t, prompt, "do not prepend environment assignments or add scope-selection arguments")
	assertPromptContains(t, prompt, "Treat account passwords as write-only input")
	for _, staleInstruction := range []string{"--global-scope", "--scope-user-id", "NEXUS_PROJECT_ROOT=/opt/app"} {
		if strings.Contains(prompt, staleInstruction) {
			t.Fatalf("主智能体默认提示词不应注入历史 CLI 指令 %q: %s", staleInstruction, prompt)
		}
	}
	assertPromptContains(t, prompt, "setup_status: configured")
	if strings.Contains(prompt, "Prefer restoring existing Rooms before creating duplicates") || strings.Contains(prompt, "Memory files:") || strings.Contains(prompt, "The runtime loads `MEMORY.md`") {
		t.Fatalf("主智能体产品侧 prompt 不应注入 memory 文案或 MEMORY.md 内容: %s", prompt)
	}
	if strings.Contains(prompt, "main-agent") || strings.Contains(prompt, "This prompt is internal") || strings.Contains(prompt, "editable context") {
		t.Fatalf("主智能体默认提示词不应保留解释性 main-agent 文案: %s", prompt)
	}
	if strings.Contains(prompt, "主智能体可见规则") || strings.Contains(prompt, "主智能体外置人格") || strings.Contains(prompt, "主智能体外置工具") {
		t.Fatalf("主智能体默认提示词不应加载 AGENTS/SOUL/TOOLS: %s", prompt)
	}
}

func TestServiceBuildRuntimePromptIncludesUserScopeContext(t *testing.T) {
	workspacePath := t.TempDir()
	service := agentsvc.NewService(config.Config{
		DefaultAgentID: "nexus",
	}, nil)
	ctx := authsvc.WithState(context.Background(), authsvc.State{
		AuthRequired: true,
		UserCount:    2,
	})
	ctx = authsvc.WithPrincipal(ctx, &authsvc.Principal{
		UserID:     "user-123",
		Username:   "alice",
		AuthMethod: authsvc.AuthMethodPassword,
	})

	prompt, err := service.BuildRuntimePrompt(ctx, &protocol.Agent{
		AgentID:       "nexus",
		Name:          "nexus",
		WorkspacePath: workspacePath,
	})
	if err != nil {
		t.Fatalf("构建多用户运行时提示词失败: %v", err)
	}

	assertPromptContains(t, prompt, "Mode: multi-user user scope")
	assertPromptContains(t, prompt, "Current user_id: user-123")
	assertPromptContains(t, prompt, "Current username: alice")
	assertPromptContains(t, prompt, "Scope: this user only.")
}

func writePromptFile(t *testing.T, workspacePath string, fileName string, content string) {
	t.Helper()
	targetPath := filepath.Join(workspacePath, fileName)
	if err := os.WriteFile(targetPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("写入 %s 失败: %v", fileName, err)
	}
}

func assertPromptContains(t *testing.T, prompt string, expected string) {
	t.Helper()
	if !strings.Contains(prompt, expected) {
		t.Fatalf("提示词缺少内容 %q:\n%s", expected, prompt)
	}
}
