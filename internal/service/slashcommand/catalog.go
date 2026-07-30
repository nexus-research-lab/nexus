// INPUT: Nexus 版本内置的 nxs/Claude runtime Slash 指令清单。
// OUTPUT: 按 runtime kind 选择的只读命令快照。
// POS: 固定 runtime 指令的唯一真相源；不启动 runtime，也不绑定业务 session。
package slashcommand

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

const catalogGeneration = 1

// RuntimeCatalogSnapshot 是当前 Nexus 版本内置的单个 runtime 指令快照。
type RuntimeCatalogSnapshot struct {
	Status      protocol.CommandCatalogStatus
	Generation  uint64
	RuntimeKind agentclient.RuntimeKind
	Commands    []protocol.CommandDescriptor
}

// Catalog 保存与当前 Nexus 版本一起发布的 runtime 指令清单。
//
// 项目 Skill、用户命令和 MCP 动态命令不进入这里；它们仍可由用户直接输入
// 并透传给 runtime，下一次 Nexus 版本更新时再决定是否纳入补全清单。
type Catalog struct {
	snapshots map[agentclient.RuntimeKind]RuntimeCatalogSnapshot
}

// NewCatalog 创建无需启动同步的只读目录。
func NewCatalog() *Catalog {
	return &Catalog{
		snapshots: map[agentclient.RuntimeKind]RuntimeCatalogSnapshot{
			agentclient.RuntimeNXS: newRuntimeSnapshot(
				agentclient.RuntimeNXS,
				[]protocol.CommandDescriptor{
					newRuntimeCommand(
						"model",
						"Set the AI model for this session",
						"<model>",
					),
					newRuntimeCommand(
						"summary",
						"Update the current session summary",
						"",
					),
				},
			),
			agentclient.RuntimeClaude: newRuntimeSnapshot(
				agentclient.RuntimeClaude,
				[]protocol.CommandDescriptor{
					newRuntimeCommand(
						"clear",
						"Start a new session with empty context; the previous session remains resumable",
						"[name]",
					),
					newRuntimeCommand(
						"color",
						"Set the prompt bar color for this session",
						"[red|blue|green|yellow|purple|orange|pink|cyan|default]",
					),
					newRuntimeCommand(
						"compact",
						"Free up context by summarizing the conversation so far",
						"<optional instructions>",
					),
					newRuntimeCommand(
						"config",
						"Set a setting by key",
						"key=value",
					),
					newRuntimeCommand(
						"context",
						"Show current context usage",
						"",
					),
					newRuntimeCommand(
						"effort",
						"Set effort level for model usage",
						"<low|medium|high|xhigh|max|ultracode|auto>",
					),
					newRuntimeCommand(
						"fast",
						"Toggle fast mode",
						"[on|off]",
					),
					newRuntimeCommand(
						"goal",
						"Set a goal and keep working until the condition is met",
						"",
					),
					newRuntimeCommand(
						"heapdump",
						"Dump the JavaScript heap to ~/Desktop",
						"",
					),
					newRuntimeCommand(
						"init",
						"Initialize a new CLAUDE.md file with codebase documentation",
						"",
					),
					newRuntimeCommand(
						"insights",
						"Generate a report analyzing Claude Code sessions",
						"",
					),
					newRuntimeCommand(
						"mcp",
						"Manage MCP servers",
						"[reconnect|enable|disable [server|all]]",
					),
					newRuntimeCommand(
						"model",
						"Set the AI model for Claude Code",
						"<model>",
					),
					newRuntimeCommand(
						"recap",
						"Generate a one-line session recap now",
						"",
					),
					newRuntimeCommand(
						"reload-skills",
						"Pick up skills added or changed on disk during this session",
						"",
					),
					newRuntimeCommand(
						"rename",
						"Rename the current conversation",
						"[name]",
					),
					newRuntimeCommand(
						"review",
						"Review a GitHub pull request",
						"[pr number]",
					),
					newRuntimeCommand(
						"security-review",
						"Review pending changes for security issues",
						"",
					),
					newRuntimeCommand(
						"team-onboarding",
						"Help teammates ramp on Claude Code with a guide from your usage",
						"",
					),
					newRuntimeCommand(
						"usage",
						"Show session cost, plan usage, and what contributes to the limits",
						"",
					),
				},
			),
		},
	}
}

// Snapshot 返回指定 runtime 的不可变副本。
func (c *Catalog) Snapshot(kind agentclient.RuntimeKind) RuntimeCatalogSnapshot {
	kind = normalizeRuntimeKind(kind)
	if c == nil {
		return unavailableRuntimeCatalog(kind)
	}
	snapshot, ok := c.snapshots[kind]
	if !ok {
		return unavailableRuntimeCatalog(kind)
	}
	snapshot.Commands = cloneCommandDescriptors(snapshot.Commands)
	return snapshot
}

func newRuntimeSnapshot(
	kind agentclient.RuntimeKind,
	commands []protocol.CommandDescriptor,
) RuntimeCatalogSnapshot {
	return RuntimeCatalogSnapshot{
		Status:      protocol.CommandCatalogStatusReady,
		Generation:  catalogGeneration,
		RuntimeKind: kind,
		Commands:    cloneCommandDescriptors(commands),
	}
}

func newRuntimeCommand(
	name string,
	description string,
	argumentHint string,
) protocol.CommandDescriptor {
	return protocol.CommandDescriptor{
		Name:         name,
		Description:  description,
		ArgumentHint: argumentHint,
		Execution:    protocol.CommandExecutionRuntime,
		Enabled:      true,
	}
}

func normalizeRuntimeKind(kind agentclient.RuntimeKind) agentclient.RuntimeKind {
	switch strings.ToLower(strings.TrimSpace(string(kind))) {
	case "claude", "cc", "claude-code", "claudecode":
		return agentclient.RuntimeClaude
	case "", "nxs", "go", "go-native", "gonative":
		return agentclient.RuntimeNXS
	default:
		return kind
	}
}

func unavailableRuntimeCatalog(kind agentclient.RuntimeKind) RuntimeCatalogSnapshot {
	return RuntimeCatalogSnapshot{
		Status:      protocol.CommandCatalogStatusUnavailable,
		RuntimeKind: kind,
		Commands:    []protocol.CommandDescriptor{},
	}
}

func cloneCommandDescriptors(
	commands []protocol.CommandDescriptor,
) []protocol.CommandDescriptor {
	if len(commands) == 0 {
		return []protocol.CommandDescriptor{}
	}
	return append([]protocol.CommandDescriptor(nil), commands...)
}
