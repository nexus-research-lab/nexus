package operation

import (
	"context"
	"path/filepath"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestStageOpenRoutingHookRedirectsWorkspaceOpenForNXSAndClaude(t *testing.T) {
	t.Parallel()

	for _, runtimeKind := range []agentclient.RuntimeKind{
		agentclient.RuntimeNXS,
		agentclient.RuntimeClaude,
	} {
		runtimeKind := runtimeKind
		t.Run(string(runtimeKind), func(t *testing.T) {
			t.Parallel()
			workspace := t.TempDir()
			service := NewService(config.Config{CacheFileDir: t.TempDir()})
			if _, err := service.TouchStagePresence(
				context.Background(),
				"room:group:conversation-1",
				"browser-a",
			); err != nil {
				t.Fatal(err)
			}

			options := service.WithStageOpenRoutingHook(agentclient.Options{
				Runtime: agentclient.RuntimeOptions{Kind: runtimeKind},
			}, "agent:agent-1:ws:group:conversation-1", workspace)
			matchers := options.Hooks.Matchers[sdkhook.EventPreToolUse]
			if len(matchers) != 1 || matchers[0].Matcher != "Bash" || len(matchers[0].Hooks) != 1 {
				t.Fatalf("PreToolUse Bash hook not registered: %+v", matchers)
			}

			originalCommand := `open "` + filepath.Join(workspace, "games", "five stones.html") + `"`
			originalInput := map[string]any{"command": originalCommand, "timeout": float64(120000)}
			output, err := matchers[0].Hooks[0](context.Background(), sdkhook.Input{
				EventName: sdkhook.EventPreToolUse,
				ToolName:  "Bash",
				ToolInput: originalInput,
				CWD:       workspace,
			}, "tool-1")
			if err != nil {
				t.Fatalf("stage hook error = %v", err)
			}
			if output.SpecificOutput == nil {
				t.Fatal("stage hook did not return updated input")
			}
			updatedCommand, _ := output.SpecificOutput.UpdatedInput["command"].(string)
			payload, ok := decodeStageOpenRedirectCommand(updatedCommand)
			if !ok {
				t.Fatalf("updated command does not contain a stage redirect: %q", updatedCommand)
			}
			if payload.Command != originalCommand || payload.Target != "games/five stones.html" {
				t.Fatalf("redirect payload = %+v", payload)
			}
			if originalInput["command"] != originalCommand {
				t.Fatal("hook mutated original tool input")
			}
		})
	}
}

func TestStageOpenRoutingHookRequiresActiveStageAndStandaloneOpen(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	service := NewService(config.Config{CacheFileDir: t.TempDir()})
	options := service.WithStageOpenRoutingHook(
		agentclient.Options{},
		"agent:agent-1:ws:dm:conversation-1",
		workspace,
	)
	hook := options.Hooks.Matchers[sdkhook.EventPreToolUse][0].Hooks[0]
	input := sdkhook.Input{
		EventName: sdkhook.EventPreToolUse,
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": `open "page.html"`},
		CWD:       workspace,
	}

	output, err := hook(context.Background(), input, "tool-1")
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput != nil {
		t.Fatal("inactive stage should not rewrite Bash input")
	}

	if _, err = service.TouchStagePresence(
		context.Background(),
		"agent:agent-1:ws:dm:conversation-1",
		"browser-a",
	); err != nil {
		t.Fatal(err)
	}
	input.ToolInput = map[string]any{"command": `echo ready && open "page.html"`}
	output, err = hook(context.Background(), input, "tool-2")
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput != nil {
		t.Fatal("compound shell command should not be partially replaced")
	}
}
