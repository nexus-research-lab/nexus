// INPUT: 在线群的本机执行绑定与 Room 辅助栏目。
// OUTPUT: 精确会话复用、远程成员隔离与文件导航的回归检查。
// POS: 只替换资源边界，不启动模型或读取用户文件。
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import type { Agent } from "@/types/agent/agent";
import { TeamExecutionSurface } from "./team-execution-surface";

const model = vi.hoisted(() => ({ conversation: vi.fn(), graph: vi.fn(), subagents: vi.fn(), workspace: vi.fn(), about: vi.fn() }));
vi.mock("@/shared/i18n/i18n-context", () => ({useI18n: () => ({t: (key: string) => key})}));
vi.mock("@/hooks/agent/use-agent-conversation", () => ({useAgentConversation: model.conversation}));
vi.mock("@/features/conversation/shared/execution/use-execution-resource", () => ({useExecutionResource: () => ({})}));
vi.mock("@/features/conversation/shared/execution/execution-workgraph-surface", () => ({ExecutionWorkGraphSurface: (props: {onOpenWorkspaceFile: (path: string) => void}) => { model.graph(props); return <button onClick={() => props.onOpenWorkspaceFile("report.md")}>artifact</button>; }}));
vi.mock("@/features/conversation/room/surface/room-subagent-task-surface", () => ({RoomSubagentTaskSurface: (props: unknown) => { model.subagents(props); return null; }}));
vi.mock("@/features/conversation/room/workspace/room-workspace-view", () => ({RoomWorkspaceView: (props: unknown) => { model.workspace(props); return null; }}));
vi.mock("@/features/conversation/room/surface/room-agent-about-surface", () => ({RoomAgentAboutSurface: (props: unknown) => { model.about(props); return null; }}));
vi.mock("./team-workspace", () => ({TeamWorkspace: () => <div>shared files</div>}));

it("uses only the bound local session for all four panels and forwards artifact navigation", () => {
  model.conversation.mockReturnValue({messages: []});
  const agent: Agent = {agent_id: "local", name: "My Agent", options: {}, workspace_path: "/local", status: "active", created_at: 0};
  const props = {roomId: "online", agents: [agent], selectedAgentId: "local", compact: false, activeWorkspacePath: null,
    binding: {agent_id: "remote", local_agent_id: "local", room_id: "internal-room", conversation_id: "internal-conversation"},
    onSelectAgent: vi.fn(), onClose: vi.fn(), onOpenWorkspaceFile: vi.fn()};
  const view = render(<TeamExecutionSurface {...props} tab="workgraph" />);
  expect(model.conversation.mock.lastCall?.[0].identity).toMatchObject({agent_id: "local", room_id: "internal-room", conversation_id: "internal-conversation", chat_type: "group"});
  fireEvent.click(screen.getByText("artifact"));
  expect(props.onOpenWorkspaceFile).toHaveBeenCalledWith("report.md");
  view.rerender(<TeamExecutionSurface {...props} tab="subagents" />);
  expect(model.subagents.mock.lastCall?.[0].source).toEqual({kind: "room", room_id: "internal-room", conversation_id: "internal-conversation"});
  view.rerender(<TeamExecutionSurface {...props} tab="workspace" activeWorkspacePath="report.md" />);
  expect(model.workspace.mock.lastCall?.[0]).toMatchObject({agentId: "local", activeWorkspacePath: "report.md", composerDraftScopeKey: null});
  view.rerender(<TeamExecutionSurface {...props} tab="about" />);
  expect(model.about.mock.lastCall?.[0]).toMatchObject({agent, conversationId: "internal-conversation"});
  model.conversation.mockClear();
  view.rerender(<TeamExecutionSurface {...props} agents={[]} tab="workgraph" />);
  expect(screen.getByRole("status").textContent).toBe("team.execution_empty");
  expect(model.conversation).not.toHaveBeenCalled();
  view.rerender(<TeamExecutionSurface {...props} agents={[]} tab="workspace" />);
  expect(screen.getByText("shared files")).toBeTruthy();
});
