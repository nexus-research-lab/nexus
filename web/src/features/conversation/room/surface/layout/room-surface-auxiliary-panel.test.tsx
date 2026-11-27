// INPUT: 真实辅助面板/分隔条与按类型隔离的内容资源边界。
// OUTPUT: 首次访问才加载、已访问页保留草稿、作用域变化和原始导航/尺寸命令回归。
// POS: 右栏装配 DOM 测试；不重建文件、编辑器或 Execution 领域状态机。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect, useState, type ComponentProps, type ReactNode } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import type { ExecutionWorkGraphSurface } from "@/features/conversation/shared/execution/execution-workgraph-surface";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import type { RoomWorkspaceView } from "../../workspace/room-workspace-view";
import type { RoomAgentAboutSurface } from "../room-agent-about-surface";
import type { RoomSubagentTaskSurface } from "../room-subagent-task-surface";
import { RoomSurfaceAuxiliaryPanel } from "./room-surface-auxiliary-panel";

const boundaries = vi.hoisted(() => ({
  mount: vi.fn(), unmount: vi.fn(), graph: vi.fn(), workspace: vi.fn(), about: vi.fn(), task: vi.fn(),
}));
function ContentFixture({ name, children }: { name: string; children?: ReactNode }) {
  const [draft, setDraft] = useState("");
  useEffect(() => { boundaries.mount(name); return () => { boundaries.unmount(name); }; }, [name]);
  return <div>
    <input aria-label={`${name} draft`} onChange={(event) => setDraft(event.target.value)} value={draft} />
    {children}
  </div>;
}
vi.mock("@/features/conversation/shared/execution/execution-workgraph-surface", () => ({
  ExecutionWorkGraphSurface: (props: ComponentProps<typeof ExecutionWorkGraphSurface>) => {
    boundaries.graph(props);
    return <ContentFixture name="Graph"><button onClick={() => props.onOpenWorkspaceFile?.("/graph.md", "producer")}>Open graph file</button></ContentFixture>;
  },
}));
vi.mock("../../workspace/room-workspace-view", () => ({
  RoomWorkspaceView: (props: ComponentProps<typeof RoomWorkspaceView>) => {
    boundaries.workspace(props);
    return <ContentFixture name="Workspace"><button onClick={() => props.onOpenWorkspaceFile("/notes.md")}>Open workspace file</button></ContentFixture>;
  },
}));
vi.mock("../room-agent-about-surface", () => ({
  RoomAgentAboutSurface: (props: ComponentProps<typeof RoomAgentAboutSurface>) => {
    boundaries.about(props);
    return <ContentFixture name="About" />;
  },
}));
vi.mock("../room-subagent-task-surface", () => ({
  RoomSubagentTaskSurface: (props: ComponentProps<typeof RoomSubagentTaskSurface>) => {
    boundaries.task(props);
    return <ContentFixture name="Task">
      <button onClick={props.onClose}>Close tasks</button>
      <button onClick={() => props.onOpenWorkspaceFile?.("/task.md", "worker")}>Open task file</button>
    </ContentFixture>;
  },
}));

const agent = { agent_id: "agent", name: "Nova", workspace_path: "/workspace", created_at: 1, options: {}, status: "idle" as const };
type Props = ComponentProps<typeof RoomSurfaceAuxiliaryPanel>;
function baseProps(): Props {
  return {
    aboutRequest: { agent_id: null, key: 0, tab: "identity" }, activeSurfaceTab: "workgraph", activeWorkspacePath: null,
    conversationId: "conversation-a", composerDraftScopeKey: "draft-a", currentAgent: agent, executionTaskRuns: [],
    executionResource: { dismiss: vi.fn(), error: null, execution: null, isLoading: false, isStale: false,
      lastSuccessfulAt: null, refresh: vi.fn(), sessionKey: null },
    isDm: false, onClose: vi.fn(), onOpenWorkspaceFile: vi.fn(), onSaveAgentOptions: vi.fn(async () => undefined),
    onStartSidePanelResize: vi.fn(), onSidePanelWidthChange: vi.fn(), onValidateAgentName: vi.fn(), roomId: "room-a",
    roomMembers: [agent], sidePanelWidthPercent: 42, subagentRequest: { hostAgentId: null, key: 0, toolUseId: null },
    subagentTaskSource: { kind: "room", room_id: "room-a", conversation_id: "conversation-a" },
  };
}
function view(props: Props, locale: Locale = "en") {
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key, params) => Object.entries(params ?? {}).reduce(
    (text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key],
  ) }}><div><RoomSurfaceAuxiliaryPanel {...props} /></div></I18N_CONTEXT.Provider>;
}
beforeEach(() => { vi.clearAllMocks(); });
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it("keeps task mounting conditional and forwards exact request and file commands", async () => {
  const user = userEvent.setup();
  const props = baseProps();
  const rendered = render(view({ ...props, activeSurfaceTab: "subagents", subagentTaskSource: null }));
  expect(boundaries.mount).not.toHaveBeenCalled();
  const requested = { hostAgentId: "worker", key: 7, toolUseId: "tool-7" };
  rendered.rerender(view({ ...props, activeSurfaceTab: "subagents", subagentRequest: requested }));
  expect(boundaries.mount.mock.calls).toEqual([["Task"]]);
  expect(boundaries.task).toHaveBeenLastCalledWith(expect.objectContaining({
    requestedHostAgentId: "worker", requestKey: 7, requestedTaskToolUseId: "tool-7", source: props.subagentTaskSource,
  }));
  await user.click(screen.getByRole("button", { name: "Open task file" }));
  expect(props.onOpenWorkspaceFile).toHaveBeenLastCalledWith("/task.md", "worker");
  await user.click(screen.getByRole("button", { name: "Close tasks" }));
  expect(props.onClose).toHaveBeenCalledOnce();
  rendered.rerender(view({ ...props, activeSurfaceTab: "workspace" }));
  expect(boundaries.unmount).toHaveBeenCalledWith("Task");
  await user.click(screen.getByRole("button", { name: "Open workspace file" }));
  expect(props.onOpenWorkspaceFile).toHaveBeenLastCalledWith("/notes.md");
  rendered.rerender(view(props));
  await user.click(screen.getByRole("button", { name: "Open graph file" }));
  expect(props.onOpenWorkspaceFile).toHaveBeenLastCalledWith("/graph.md", "producer");
  expect(props.onOpenWorkspaceFile).toHaveBeenCalledTimes(3);
  expect(boundaries.graph).toHaveBeenLastCalledWith(expect.objectContaining({
    agents: [agent], resource: props.executionResource, taskRuns: props.executionTaskRuns,
  }));
});
