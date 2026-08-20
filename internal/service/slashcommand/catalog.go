// INPUT: Nexus 版本内置的 runtime 指令与产品提示型 Slash 指令。
// OUTPUT: 按 runtime kind 选择的只读命令快照与运行时提示展开。
// POS: 固定 Slash 指令的唯一真相源；不启动 runtime，也不绑定业务 session。
package slashcommand

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

const (
	catalogGeneration    = 6
	visualizeCommandName = "visualize"
	workGraphCommandName = "workgraph"
)

// RuntimeCatalogSnapshot 是当前 Nexus 版本内置的单个 runtime 指令快照。
type RuntimeCatalogSnapshot struct {
	Status      protocol.CommandCatalogStatus
	Generation  uint64
	RuntimeKind agentclient.RuntimeKind
	Commands    []protocol.CommandDescriptor
}

// Catalog 保存与当前 Nexus 版本一起发布的 runtime 指令清单。
//
// `/skills` 作为宿主侧技能选择入口会一并进入目录；其余项目 Skill、用户命令
// 和 MCP 动态命令不进入这里，仍可由用户直接输入并透传给 runtime，下一次
// Nexus 版本更新时再决定是否纳入补全清单。
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
						"compact",
						"Free up context by summarizing the conversation so far",
						"<optional instructions>",
					),
					newRuntimeCommand(
						"skills",
						"Browse and insert available skills",
						"",
					),
				},
			),
			agentclient.RuntimeClaude: newRuntimeSnapshot(
				agentclient.RuntimeClaude,
				[]protocol.CommandDescriptor{
					newRuntimeCommand(
						"compact",
						"Free up context by summarizing the conversation so far",
						"<optional instructions>",
					),
					newRuntimeCommand(
						"skills",
						"Browse and insert available skills",
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

// VisualizeCommandDescriptor 返回产品内置的 Generative UI 入口。
func VisualizeCommandDescriptor() protocol.CommandDescriptor {
	return newRuntimeCommand(
		visualizeCommandName,
		"Create an interactive visual with Generative UI",
		"<request>",
	)
}

// WorkGraphCommandDescriptor 返回显式启用 Nexus WorkGraph 协作的产品入口。
// 具体命名工作图使用各自的 Slash，不复用该固定名称。
func WorkGraphCommandDescriptor() protocol.CommandDescriptor {
	return newRuntimeCommand(
		workGraphCommandName,
		"Use WorkGraph collaboration for this request",
		"<request>",
	)
}

// ExpandVisualizePrompt 仅在投递 runtime 时把 /visualize 展开为简短提示。
func ExpandVisualizePrompt(content string) string {
	return ExpandProductPrompt(content)
}

// ExpandProductPrompt 只在 runtime 投递边界展开 Nexus 固定产品提示型命令。
func ExpandProductPrompt(content string) string {
	name, arguments, ok := parseInvocation(content)
	if !ok {
		return content
	}
	switch name {
	case visualizeCommandName:
		if arguments == "" {
			return "Use Generative UI to create a relevant interactive visual from the current conversation."
		}
		return "Use Generative UI to create an interactive visual for the following request:\n\n" + arguments
	case workGraphCommandName:
		request := arguments
		if request == "" {
			request = "Use the actionable request already established in this conversation."
		}
		return "Use Nexus WorkGraph collaboration for the following request. Load the execution-orchestrator skill, create a fresh managed WorkGraph with explicit deliverables, dependencies, responsibility and review where needed, then execute it. Do not treat mentions as Assignment and do not reuse historical run identities.\n\nRequest:\n" + request
	default:
		return content
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
