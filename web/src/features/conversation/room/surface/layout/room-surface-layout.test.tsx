// INPUT: 真实桌面布局、Thread 控制/发布/阅读面与类型化聊天/顶栏/辅助资源边界。
// OUTPUT: 聊天挂载连续、Thread 互斥与精确来源、稳定控制及只读失败恢复回归。
// POS: Room Surface 装配集成；边界不启动聊天连接或文件/配置 API。

import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createRef, memo, useCallback, useEffect, useMemo, useState, type ComponentProps } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { useGroupThread } from "../../group/thread/group-thread-state";
import { useRoomThreadSource } from "../../group/thread/live/use-room-thread-source";
import { useRoomThreadLiveStore } from "../../group/thread/live/room-thread-live-store";
import type { RoomChatSurface } from "../room-chat-surface";
import type { RoomSurfaceHeader } from "./room-surface-header";
import type { RoomSurfaceAuxiliaryPanel } from "./room-surface-auxiliary-panel";
import type { RoomSurfaceLayoutProps } from "./room-surface-layout-types";
import { RoomSurfaceLayout } from "./room-surface-layout";

const boundaries = vi.hoisted(() => ({
  chatMount: vi.fn(), chatUnmount: vi.fn(), auxiliary: vi.fn(), auxiliaryUnmount: vi.fn(), controls: vi.fn(),
  collapse: vi.fn(), restore: vi.fn(),
}));
vi.mock("@/shared/lib/react/use-media-query", () => ({ useMediaQuery: () => false }));
vi.mock("@/store/sidebar", () => ({ useSidebarStore: (selector: (state: unknown) => unknown) => selector({
  collapse_wide_panel_for_right_panel: boundaries.collapse, expand_wide_panel_after_right_panel: boundaries.restore,
}) }));
vi.mock("./room-surface-header", () => ({ RoomSurfaceHeader: (props: ComponentProps<typeof RoomSurfaceHeader>) => <div>
  <output aria-label="Room title">{props.currentRoomTitle}</output>
  {(["workspace", "about", "subagents"] as const).map((tab) => <button key={tab} onClick={() => props.onChangeSurfaceTab(tab)}>Header {tab}</button>)}
  <button onClick={props.onCloseAuxiliaryPanel}>Close auxiliary</button>
</div> }));
vi.mock("./room-surface-auxiliary-panel", () => ({ RoomSurfaceAuxiliaryPanel: (props: ComponentProps<typeof RoomSurfaceAuxiliaryPanel>) => {
  boundaries.auxiliary(props);
  return <AuxiliaryFixture />;
} }));
function AuxiliaryFixture() {
  const [draft, setDraft] = useState("");
  useEffect(() => () => { boundaries.auxiliaryUnmount(); }, []);
  return <input aria-label="Auxiliary draft" value={draft} onChange={(event) => setDraft(event.target.value)} />;
}
vi.mock("../room-chat-surface", () => ({ RoomChatSurface: (props: ComponentProps<typeof RoomChatSurface>) => <ChatFixture {...props} /> }));
function ChatFixture(props: ComponentProps<typeof RoomChatSurface>) {
  const [draft, setDraft] = useState("");
  useEffect(() => { boundaries.chatMount(); return () => { boundaries.chatUnmount(); }; }, []);
  return <div>
    <input aria-label="Chat draft" value={draft} onChange={(event) => setDraft(event.target.value)} />
    <button onClick={props.onOpenWorkGraph}>Chat graph</button>
    <button onClick={() => props.onOpenAgentContact?.("agent")}>Chat contact</button>
    <button disabled={!props.onOpenSubagentTask} onClick={() => props.onOpenSubagentTask?.(" tool-7 ", " worker ")}>Chat task</button>
    {props.currentRoomType !== "dm" ? <GroupSource conversationId={props.conversationId} name={props.currentAgent.name} /> : null}
  </div>;
}
const emptySource = { agentAvatarMap: {}, messageGroups: new Map(), pendingPermissionGroups: new Map(),
  pendingSlotGroups: new Map(), roomAgentExecutionStateGroups: new Map(), sendPermissionResponse: vi.fn(() => true) };
function GroupSource({ conversationId, name }: { conversationId: string | null; name: string }) {
  const source = useMemo(() => ({ ...emptySource, agentNameMap: { agent: name }, conversationId }), [conversationId, name]);
  useRoomThreadSource(source);
  return <ThreadControls />;
}
const ThreadControls = memo(function ThreadControls() {
  const controls = useGroupThread();
  boundaries.controls(controls);
  return <>
    <button onClick={() => controls.openThread("root", "agent", "agent-round")}>Open exact Thread</button>
    <output aria-label="Thread selection">{controls.activeThread ? JSON.stringify(controls.activeThread) : "closed"}</output>
  </>;
});

function baseProps(): RoomSurfaceLayoutProps {
  const agent = { agent_id: "agent", name: "Nova", workspace_path: "/workspace", created_at: 1, options: {}, status: "idle" as const };
  return {
    currentAgent: agent, currentRoomType: "group", roomId: "room-a", roomMembers: [agent], availableRoomAgents: [agent],
    currentRoomTitle: "Research", runtimeKind: "nxs", roomSkillNames: [], roomHostAgentId: "agent",
    roomHostAutoReplyEnabled: true, roomPrivateMessagesEnabled: true, currentAgentSessionIdentity: null,
    conversationId: "conversation-a", currentRoomConversations: [], activeWorkspacePath: null, activeSurfaceTab: "chat",
    executionResource: { dismiss: vi.fn(), error: null, execution: null, isLoading: false, isStale: false,
      lastSuccessfulAt: null, refresh: vi.fn(), sessionKey: null }, executionTaskRuns: [],
    externalSessionsReliability: { failure: null, isLoading: false, isStale: false, refresh: vi.fn() },
    sidePanelWidthPercent: 42, isResizingSidePanel: false, currentTodos: [], surfaceSplitRef: createRef<HTMLElement>(),
    onExecutionTaskRunsChange: vi.fn(), onChangeSurfaceTab: vi.fn(), onCreateConversation: vi.fn(async () => null),
    onReplaceFinalConversation: vi.fn(async () => undefined), onSelectConversation: vi.fn(), onCloseConversation: vi.fn(async () => undefined),
    onDeleteConversation: vi.fn(async () => null), onForkConversation: vi.fn(async () => undefined),
    onManageRoom: vi.fn(async () => undefined), onOpenMemberManager: vi.fn(async () => undefined),
    onSaveAgentOptions: vi.fn(async () => undefined), onValidateAgentName: vi.fn(),
    onUpdateConversationTitle: vi.fn(async () => undefined), onOpenWorkspaceFile: vi.fn(), onStartSidePanelResize: vi.fn(),
    onSidePanelWidthChange: vi.fn(), onTodosChange: vi.fn(), onConversationSnapshotChange: vi.fn(),
  };
}
function Harness({ props }: { props: RoomSurfaceLayoutProps }) {
  const [tab, setTab] = useState(props.activeSurfaceTab);
  const onChange = props.onChangeSurfaceTab;
  const changeTab = useCallback((next: typeof tab) => { onChange(next); setTab(next); }, [onChange]);
  return <RoomSurfaceLayout {...props} activeSurfaceTab={tab} onChangeSurfaceTab={changeTab} />;
}
function view(props: RoomSurfaceLayoutProps) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key, params) => Object.entries(params ?? {}).reduce(
    (text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES.en[key],
  ) }}><Harness props={props} /></I18N_CONTEXT.Provider>;
}
beforeEach(() => { vi.clearAllMocks(); useRoomThreadLiveStore.getState().clearSource(); });
afterEach(() => { act(() => useRoomThreadLiveStore.getState().clearSource()); vi.restoreAllMocks(); });

it("keeps the chat and live publication mounted throughout auxiliary navigation", async () => {
  const user = userEvent.setup();
  const props = baseProps();
  const rendered = render(view(props));
  const chat = screen.getByRole("textbox", { name: "Chat draft" });
  const source = useRoomThreadLiveStore.getState().source;
  await user.type(chat, "Unsent message");
  await user.click(screen.getByRole("button", { name: "Header workspace" }));
  const auxiliary = screen.getByRole("textbox", { name: "Auxiliary draft" });
  await user.type(auxiliary, "Unsaved edit");
  await user.click(screen.getByRole("button", { name: "Header about" }));
  expect(screen.getByLabelText("Auxiliary draft")).toBe(auxiliary);
  expect((auxiliary as HTMLInputElement).value).toBe("Unsaved edit");
  expect(boundaries.auxiliary).toHaveBeenLastCalledWith(expect.objectContaining({
    onOpenWorkspaceFile: props.onOpenWorkspaceFile, onSidePanelWidthChange: props.onSidePanelWidthChange,
    onStartSidePanelResize: props.onStartSidePanelResize, aboutRequest: { agent_id: "agent", tab: "identity", key: 1 },
  }));
  await user.click(screen.getByRole("button", { name: "Close auxiliary" }));
  expect(screen.queryByLabelText("Auxiliary draft")).toBeNull();
  expect(boundaries.auxiliaryUnmount).toHaveBeenCalledOnce();
  expect(screen.getByLabelText("Chat draft")).toBe(chat);
  expect((chat as HTMLInputElement).value).toBe("Unsent message");
  expect(useRoomThreadLiveStore.getState().source).toBe(source);
  expect(boundaries.chatMount).toHaveBeenCalledOnce();
  expect(boundaries.chatUnmount).not.toHaveBeenCalled();
  rendered.unmount();
  expect(boundaries.chatUnmount).toHaveBeenCalledOnce();
  expect(useRoomThreadLiveStore.getState().source).toBeNull();
});

it("keeps exact Thread selection, closes it for auxiliary pages and clears it on Session changes", async () => {
  const user = userEvent.setup();
  const props = baseProps();
  const rendered = render(view(props));
  const chat = screen.getByRole("textbox", { name: "Chat draft" });
  await user.click(screen.getByRole("button", { name: "Open exact Thread" }));
  expect(screen.getByLabelText("Thread selection").textContent).toBe(JSON.stringify({ roundId: "root", agentId: "agent", agentRoundId: "agent-round" }));
  expect(screen.getByRole("separator", { name: MESSAGES.en["room.resize_thread_panel"] })).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "Chat graph" }));
  expect(screen.getByLabelText("Thread selection").textContent).toBe("closed");
  expect(screen.queryByRole("separator")).toBeNull();
  expect(boundaries.auxiliary).toHaveBeenLastCalledWith(expect.objectContaining({ activeSurfaceTab: "workgraph" }));
  await user.click(screen.getByRole("button", { name: "Open exact Thread" }));
  expect(screen.queryByLabelText("Auxiliary draft")).toBeNull();
  expect(screen.getByRole("separator")).toBeTruthy();
  rendered.rerender(view({ ...props, currentAgent: { ...props.currentAgent, name: "Nova updated" } }));
  expect(screen.getByText("Nova updated")).toBeTruthy();
  expect(screen.getByLabelText("Thread selection").textContent).toContain('"agentRoundId":"agent-round"');
  rendered.rerender(view({ ...props, conversationId: "conversation-b" }));
  expect(screen.getByLabelText("Thread selection").textContent).toBe("closed");
  rendered.rerender(view(props));
  expect(screen.queryByRole("separator")).toBeNull();
  expect(screen.getByLabelText("Chat draft")).toBe(chat);
});

it("does not notify memoized Thread controls for unrelated snapshots and uses the latest page command", async () => {
  const user = userEvent.setup();
  const props = baseProps();
  const rendered = render(view(props));
  const previous = boundaries.controls.mock.lastCall?.[0];
  const renders = boundaries.controls.mock.calls.length;
  rendered.rerender(view({ ...props, currentRoomTitle: "Renamed", sidePanelWidthPercent: 45 }));
  expect(boundaries.controls).toHaveBeenCalledTimes(renders);
  expect(boundaries.controls.mock.lastCall?.[0]).toBe(previous);
  const latestCommand = vi.fn();
  rendered.rerender(view({ ...props, onChangeSurfaceTab: latestCommand }));
  await user.click(screen.getByRole("button", { name: "Open exact Thread" }));
  expect(latestCommand).toHaveBeenCalledExactlyOnceWith("chat");
  expect(props.onChangeSurfaceTab).not.toHaveBeenCalled();
});

it.each([
  { access: null, isStale: false, impact: "conversation.external_sessions_unavailable_impact" },
  { access: null, isStale: true, impact: "conversation.external_sessions_stale_impact" },
  { access: "forbidden" as const, isStale: true, impact: "conversation.external_sessions_access_impact" },
] as const)("retains DM content for $impact and reloads only on request", async ({ access, isStale, impact }) => {
  const user = userEvent.setup();
  const props = { ...baseProps(), currentRoomType: "dm" };
  const rendered = render(view(props));
  const chat = screen.getByRole("textbox", { name: "Chat draft" });
  await user.type(chat, "Keep this message");
  const reliability = { ...props.externalSessionsReliability, failure: { access, message: "private-diagnostic-id" }, isStale };
  rendered.rerender(view({ ...props, externalSessionsReliability: reliability }));
  expect(screen.getByText(MESSAGES.en[impact])).toBeTruthy();
  expect(document.body.textContent).not.toContain("private-diagnostic-id");
  expect(reliability.refresh).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: MESSAGES.en["state.reload_check"] }));
  expect(reliability.refresh).toHaveBeenCalledOnce();
  rendered.rerender(view({ ...props, externalSessionsReliability: { ...reliability, isLoading: true } }));
  expect(screen.getByRole("button", { name: MESSAGES.en["state.reload_check"] }).hasAttribute("disabled")).toBe(true);
  rendered.rerender(view(props));
  expect(screen.queryByText(MESSAGES.en[impact])).toBeNull();
  expect(screen.getByLabelText("Chat draft")).toBe(chat);
  expect((chat as HTMLInputElement).value).toBe("Keep this message");
  expect(boundaries.chatMount).toHaveBeenCalledOnce();
  expect(screen.queryByRole("button", { name: "Open exact Thread" })).toBeNull();
});

it.each(["dm", "group"])("opens %s tasks with the exact current source and closes on missing source", async (kind) => {
  const user = userEvent.setup();
  const props = { ...baseProps(), currentRoomType: kind };
  const ready: RoomSurfaceLayoutProps = { ...props,
    currentAgentSessionIdentity: kind === "dm" ? { session_key: " session-a ", chat_type: "dm" } : null };
  const rendered = render(view(ready));
  await user.click(screen.getByRole("button", { name: "Chat task" }));
  expect(boundaries.auxiliary).toHaveBeenLastCalledWith(expect.objectContaining({
    activeSurfaceTab: "subagents", subagentRequest: { hostAgentId: "worker", key: 1, toolUseId: "tool-7" },
    subagentTaskSource: kind === "dm" ? { kind: "session", session_key: "session-a" }
      : { kind: "room", room_id: "room-a", conversation_id: "conversation-a" },
  }));
  rendered.rerender(view({ ...props, conversationId: null, currentAgentSessionIdentity: null }));
  expect(screen.queryByLabelText("Auxiliary draft")).toBeNull();
  expect(screen.getByRole("button", { name: "Chat task" }).hasAttribute("disabled")).toBe(true);
});
