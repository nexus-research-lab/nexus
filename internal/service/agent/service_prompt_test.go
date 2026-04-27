package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/config"
	memorysvc "github.com/nexus-research-lab/nexus/internal/memory"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

func TestServiceBuildRuntimePromptIncludesWorkspaceFilesAndProfile(t *testing.T) {
	workspacePath := t.TempDir()
	writePromptFile(t, workspacePath, "AGENTS.md", "# AGENTS.md\n\n执行规则：必须先读 AGENTS。")
	writePromptFile(t, workspacePath, "USER.md", "# USER.md\n\n用户偏好：默认中文。")
	writePromptFile(t, workspacePath, "MEMORY.md", "# MEMORY.md\n\n长期约束：不要改路径。")
	if _, err := memorysvc.NewService(workspacePath).Log("REF", "提示词注入测试", "", []memorysvc.Field{
		{Key: "经验", Value: "近期记忆应进入运行时提示词。"},
	}, ""); err != nil {
		t.Fatalf("写入记忆条目失败: %v", err)
	}

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
		WorkspacePath:   workspacePath,
	})
	if err != nil {
		t.Fatalf("构建运行时提示词失败: %v", err)
	}

	assertPromptContains(t, prompt, "BASE CUSTOM PROMPT")
	assertPromptContains(t, prompt, "运行模式: 单用户系统作用域")
	assertPromptContains(t, prompt, "当前工作区绝对路径: "+workspacePath)
	assertPromptContains(t, prompt, "执行规则：必须先读 AGENTS。")
	assertPromptContains(t, prompt, "用户偏好：默认中文。")
	assertPromptContains(t, prompt, "长期约束：不要改路径。")
	assertPromptContains(t, prompt, "规划助手")
	assertPromptContains(t, prompt, "擅长任务拆解")
	assertPromptContains(t, prompt, "偏好明确目标与验收标准")
	assertPromptContains(t, prompt, "最近日记提醒")
	assertPromptContains(t, prompt, "提示词注入测试")
}

func TestServiceBuildRuntimePromptUsesMainAgentPromptOverride(t *testing.T) {
	workspacePath := t.TempDir()
	writePromptFile(t, workspacePath, "AGENTS.md", "# AGENTS.md\n\n主智能体规则。")

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
	assertPromptContains(t, prompt, "主智能体规则")
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

	assertPromptContains(t, prompt, "运行模式: 多用户用户作用域")
	assertPromptContains(t, prompt, "当前 user_id: user-123")
	assertPromptContains(t, prompt, "当前 username: alice")
	assertPromptContains(t, prompt, "不要假设可访问其他用户的数据")
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
