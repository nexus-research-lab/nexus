// INPUT: 真实窄窗 Header/菜单/历史与会话、owner、辅助目录异步变化。
// OUTPUT: 临时导航隔离、稳定身份刷新、成员单飞和迟到打开保护的 DOM 回归。
// POS: Surface 装配测试；聊天/任务/编辑表单用类型化边界替身避免独立 HTTP/runtime。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { expect, it, vi } from "vitest";

import type { RoomMemberManagerDialog } from "../../members/room-member-manager-dialog";
import type { RoomChatSurface } from "../room-chat-surface";
import type { RoomMobileAuxiliaryOverlay } from "./room-mobile-auxiliary-overlay";
import type { RoomMobileSubagentOverlay } from "./room-mobile-subagent-overlay";
import { advanceAuthOwnerScopeGeneration, publishAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { RoomMobileSurface } from "./room-mobile-surface";

vi.mock("../room-chat-surface", () => ({ RoomChatSurface: ({ onOpenSubagentTask, onOpenWorkGraph }: ComponentProps<typeof RoomChatSurface>) =>
  <><button onClick={() => onOpenSubagentTask?.(" tool-a ", " host-a ")}>Open task from chat</button><button onClick={onOpenWorkGraph}>Open graph from chat</button></> }));
vi.mock("./room-mobile-thread-overlay", () => ({ RoomMobileThreadOverlay: () => null }));
vi.mock("./room-mobile-auxiliary-overlay", () => ({ RoomMobileAuxiliaryOverlay: ({ activeTab, onClose }: ComponentProps<typeof RoomMobileAuxiliaryOverlay>) =>
  activeTab ? <div role="region" aria-label={activeTab}><button onClick={onClose}>Close auxiliary</button></div> : null }));
vi.mock("./room-mobile-subagent-overlay", () => ({ RoomMobileSubagentOverlay: ({ source, onClose, requestedTaskToolUseId, requestedHostAgentId, onOpenWorkspaceFile }: ComponentProps<typeof RoomMobileSubagentOverlay>) =>
  source ? <div role="region" aria-label="tasks"><p>{JSON.stringify(source)}</p><p>{requestedTaskToolUseId}</p><p>{requestedHostAgentId}</p>
    <button onClick={onClose}>Close tasks</button><button onClick={() => onOpenWorkspaceFile?.("/report.md", "host-a")}>Open task file</button></div> : null }));
vi.mock("../../members/room-member-manager-dialog", () => ({ RoomMemberManagerDialog: ({ isOpen, initialName, onClose, roomMembers }: ComponentProps<typeof RoomMemberManagerDialog>) =>
  isOpen ? <div role="dialog" aria-label={initialName}><p>{roomMembers.map((member) => member.name).join(", ")}</p><button onClick={onClose}>Close members</button></div> : null }));

type Props = ComponentProps<typeof RoomMobileSurface>;
const conversation = { conversation_id: "conversation-a", room_id: "room-a", session_id: null, session_key: "session-a", title: "First", created_at: 1, last_activity_at: 2, options: {} };
const agent = { agent_id: "agent-a", name: "Nova", created_at: 1, options: {}, status: "idle" as const, workspace_path: "/workspace" };
const base: Props = {
  roomId: "room-a", conversationId: "conversation-a", currentRoomType: "group", currentRoomTitle: "Research",
  currentAgent: agent, currentAgentSessionIdentity: null, currentRoomConversation: conversation,
  currentRoomConversations: [conversation, { ...conversation, conversation_id: "history", title: "Older session" }],
  availableRoomAgents: [agent], roomMembers: [agent], roomSkillNames: [], roomHostAgentId: agent.agent_id,
  roomHostAutoReplyEnabled: true, roomPrivateMessagesEnabled: true, runtimeKind: "nxs", activeWorkspacePath: null,
  currentTodos: [], executionTaskRuns: [], executionResource: { dismiss: vi.fn(), error: null, execution: null, isLoading: false, isStale: false, lastSuccessfulAt: null, refresh: vi.fn(), sessionKey: null },
  externalSessionsReliability: { failure: null, isLoading: false, isStale: false, refresh: vi.fn() },
  onBackToDirectory: vi.fn(), onConversationSnapshotChange: vi.fn(), onCreateConversation: vi.fn(async () => null), onDeleteConversation: vi.fn(async () => null),
  onExecutionTaskRunsChange: vi.fn(), onForkConversation: vi.fn(async () => undefined), onManageRoom: vi.fn(async () => undefined),
  onOpenMemberManager: vi.fn(async () => undefined), onOpenWorkspaceFile: vi.fn(), onSaveAgentOptions: vi.fn(async () => undefined),
  onSelectConversation: vi.fn(), onTodosChange: vi.fn(), onUpdateConversationTitle: vi.fn(async () => undefined), onValidateAgentName: vi.fn(),
};
function view(props: Partial<Props> = {}) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}><RoomMobileSurface {...base} {...props} /></I18N_CONTEXT.Provider>;
}
function deferred() { let resolve!: () => void; const promise = new Promise<void>((finish) => { resolve = finish; }); return { promise, resolve }; }
async function action(name: string) {
  await userEvent.click(screen.getByRole("button", { name: "common.more_actions" }));
  await userEvent.click(screen.getByRole("menuitem", { name }));
}

it.each(["room.workgraph", "room.workspace", "room.about", "subagents.label"])("consumes %s navigation across A to B to A while preserving title/catalog refresh", async (name) => {
  const rendered = render(view());
  await action(name);
  const label = name === "subagents.label" ? "tasks" : name.slice(5);
  expect(screen.getByRole("region", { name: label })).toBeTruthy();
  rendered.rerender(view({ currentRoomTitle: "Updated", roomMembers: [...base.roomMembers], currentAgent: { ...agent } }));
  expect(screen.getByRole("region", { name: label })).toBeTruthy();
  rendered.rerender(view({ conversationId: "conversation-b" }));
  expect(screen.queryByRole("region", { name: label })).toBeNull();
  rendered.rerender(view());
  expect(screen.queryByRole("region", { name: label })).toBeNull();
});

it("keeps history selection on the original command and clears menus/switcher on navigation", async () => {
  const select = vi.fn();
  const rendered = render(view({ onSelectConversation: select }));
  await userEvent.click(screen.getByRole("button", { name: "room.switch_conversation" }));
  await userEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: /Older session/ }));
  expect(select).toHaveBeenCalledExactlyOnceWith("history");
  expect(screen.queryByRole("dialog")).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "room.switch_conversation" }));
  rendered.rerender(view({ roomId: "room-b" }));
  expect(screen.queryByRole("dialog")).toBeNull();
  rendered.rerender(view());
  expect(screen.queryByRole("dialog")).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "common.more_actions" }));
  rendered.rerender(view({ conversationId: "conversation-b" }));
  expect(screen.queryByRole("menu")).toBeNull();
  rendered.rerender(view());
  expect(screen.queryByRole("menu")).toBeNull();
});

it("uses exact DM session identity, preserves equivalent snapshots and never revives a closed task source", async () => {
  const identity = { chat_type: "dm" as const, session_key: "session-a" };
  const dm = { currentRoomType: "dm", currentAgentSessionIdentity: identity };
  const rendered = render(view(dm));
  await userEvent.click(screen.getByRole("button", { name: "Open task from chat" }));
  const tasks = screen.getByRole("region", { name: "tasks" });
  expect(within(tasks).getByText("tool-a")).toBeTruthy();
  expect(within(tasks).getByText("host-a")).toBeTruthy();
  rendered.rerender(view({ ...dm, currentAgentSessionIdentity: { ...identity, session_key: " session-a " } }));
  expect(screen.getByRole("region", { name: "tasks" })).toBeTruthy();
  rendered.rerender(view({ ...dm, currentAgentSessionIdentity: { ...identity, session_key: "session-b" } }));
  expect(screen.queryByRole("region", { name: "tasks" })).toBeNull();
  rendered.rerender(view(dm));
  expect(screen.queryByRole("region", { name: "tasks" })).toBeNull();
  await action("subagents.label");
  expect(screen.queryByText("tool-a")).toBeNull();
  expect(screen.queryByText("host-a")).toBeNull();
  const openFile = vi.fn();
  rendered.rerender(view({ ...dm, onOpenWorkspaceFile: openFile }));
  await userEvent.click(screen.getByRole("button", { name: "Open task file" }));
  expect(openFile).toHaveBeenCalledExactlyOnceWith("/report.md", "host-a");
  expect(screen.queryByRole("region", { name: "tasks" })).toBeNull();
  expect(screen.getByRole("region", { name: "workspace" })).toBeTruthy();
});

it("coalesces member preparation and ignores the old request after returning to the same Room", async () => {
  const old = deferred(); const fresh = deferred();
  const prepare = vi.fn().mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise);
  const rendered = render(view({ onOpenMemberManager: prepare }));
  await action("room.members");
  await userEvent.click(screen.getByRole("button", { name: "common.more_actions" }));
  const members = screen.getByRole("menuitem", { name: "room.members" });
  expect(members.hasAttribute("disabled")).toBe(true);
  fireEvent.click(members);
  expect(prepare).toHaveBeenCalledOnce();
  rendered.rerender(view({ roomId: "room-b", onOpenMemberManager: prepare }));
  rendered.rerender(view({ onOpenMemberManager: prepare }));
  await action("room.members");
  await act(async () => old.resolve());
  expect(screen.queryByRole("dialog")).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "common.more_actions" }));
  await act(async () => fresh.resolve());
  expect(screen.getByRole("dialog", { name: "Research" })).toBeTruthy();
  expect(screen.queryByRole("menu")).toBeNull();
  expect(prepare).toHaveBeenCalledTimes(2);
  // Membership belongs to the Room, so changing only the selected Session keeps its editor.
  rendered.rerender(view({ conversationId: "conversation-b", currentRoomTitle: "Renamed" }));
  expect(screen.getByRole("dialog", { name: "Renamed" })).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: "Close members" }));
  expect(screen.queryByRole("dialog")).toBeNull();
});

it("closes owner-scoped navigation and rejects late catalog opens after owner change or unmount", async () => {
  const pending = deferred();
  const rendered = render(view({ onOpenMemberManager: vi.fn(() => pending.promise) }));
  await action("room.members");
  act(() => { advanceAuthOwnerScopeGeneration(); publishAuthOwnerScopeGeneration(); });
  await act(async () => pending.resolve());
  expect(screen.queryByRole("dialog")).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Open graph from chat" }));
  act(() => { advanceAuthOwnerScopeGeneration(); publishAuthOwnerScopeGeneration(); });
  expect(screen.queryByRole("region", { name: "workgraph" })).toBeNull();
  const another = deferred();
  rendered.rerender(view({ onOpenMemberManager: vi.fn(() => another.promise) }));
  await action("room.members");
  rendered.unmount();
  await act(async () => another.resolve());
  render(view());
  expect(screen.queryByRole("dialog")).toBeNull();
});

it.each(["Open graph from chat", "room.switch_conversation"])("cancels a pending member open when choosing %s", async (name) => {
  const pending = deferred();
  render(view({ onOpenMemberManager: vi.fn(() => pending.promise) }));
  await action("room.members");
  await userEvent.click(screen.getByRole("button", { name }));
  await act(async () => pending.resolve());
  expect(screen.queryByRole("dialog", { name: "Research" })).toBeNull();
  if (name === "room.switch_conversation") expect(screen.getByRole("dialog")).toBeTruthy();
  else expect(screen.getByRole("region", { name: "workgraph" })).toBeTruthy();
});

it("keeps current members available on auxiliary read failure and has no member action in a DM", async () => {
  const log = vi.spyOn(console, "error").mockImplementation(() => undefined);
  try {
    const rendered = render(view({ onOpenMemberManager: vi.fn(async () => { throw new Error("catalog unavailable"); }) }));
    await action("room.members");
    expect(within(screen.getByRole("dialog", { name: "Research" })).getByText("Nova")).toBeTruthy();
    rendered.rerender(view({ currentRoomType: "dm" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "common.more_actions" }));
    expect(screen.queryByRole("menuitem", { name: "room.members" })).toBeNull();
  } finally { log.mockRestore(); }
});
