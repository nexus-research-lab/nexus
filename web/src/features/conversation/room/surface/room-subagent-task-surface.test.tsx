// INPUT: Real caller switcher, directory/detail resources, exact navigation requests and changing member catalogs.
// OUTPUT: Manual selection wins, source changes consume old intent, and late reads never replace the current scope.
// POS: Room-to-shared-subagent integration; only HTTP and realtime transports are replaced.

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { getSubagentTaskMessagesApi, listSubagentTasksApi } from "@/lib/api/conversation/subagent-task-api";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { Agent } from "@/types/agent/agent";
import type { SubagentTask, SubagentTaskListResponse, SubagentTaskSource } from "@/types/conversation/subagent-task";
import { RoomSubagentTaskSurface } from "./room-subagent-task-surface";

vi.mock("@/lib/api/conversation/subagent-task-api", () => ({ listSubagentTasksApi: vi.fn(), getSubagentTaskMessagesApi: vi.fn(), sendSubagentTaskMessageApi: vi.fn(), stopSubagentTaskApi: vi.fn() }));
vi.mock("@/features/conversation/shared/subagent/use-subagent-task-realtime-refresh", () => ({ useSubagentTaskRealtimeRefresh: vi.fn() }));

const CAPABILITIES = { observe: true, transcript: true, stop: true, send_message: true, resume: true };
const MEMBERS: Agent[] = ["Ada", "Bo"].map((name) => ({ agent_id: name.toLowerCase(), name, workspace_path: "", options: {}, created_at: 1, status: "idle" }));
const TASKS: SubagentTask[] = MEMBERS.map((member) => ({ task_id: `task-${member.agent_id}`, tool_use_id: `tool-${member.agent_id}`, host_agent_id: member.agent_id, name: `${member.name} research`, status: "running", runtime_kind: "nxs", capabilities: CAPABILITIES }));
const SOURCE: SubagentTaskSource = { kind: "room", room_id: "room-one", conversation_id: "conversation-one" };
const response = (items = TASKS): SubagentTaskListResponse => ({ runtime_kind: "nxs", capabilities: CAPABILITIES, items });
type Props = ComponentProps<typeof RoomSubagentTaskSurface>;
const localized = (children: ReactNode) => {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES.en[key]);
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t }}>{children}</I18N_CONTEXT.Provider>;
};
function view(overrides: Partial<Props> = {}) {
  return localized(<RoomSubagentTaskSurface currentAgentId="ada" onClose={vi.fn()} roomMembers={MEMBERS} source={SOURCE} {...overrides} />);
}
async function click(element: HTMLElement) {
  await act(async () => { fireEvent.click(element); });
}
async function switchCaller(name: string) {
  await click(screen.getByRole("button", { name: /^Switch calling Agent/ }));
  await click(await screen.findByRole("menuitem", { name }));
}
const direct = { requestKey: 1, requestedHostAgentId: "bo", requestedTaskToolUseId: "tool-bo" };

beforeEach(() => {
  vi.mocked(listSubagentTasksApi).mockResolvedValue(response());
  vi.mocked(getSubagentTaskMessagesApi).mockImplementation(async (_source, taskId) => ({ task: TASKS.find((task) => task.task_id === taskId)!, messages: [] }));
});
afterEach(() => { vi.resetAllMocks(); vi.restoreAllMocks(); });

describe("Subagent navigation", () => {
  it("opens the requested caller once, returns with one read and honors later manual selection", async () => {
    const { rerender } = render(view(direct));
    await screen.findByRole("button", { name: "Back" });
    expect(getSubagentTaskMessagesApi).toHaveBeenCalledExactlyOnceWith(SOURCE, "task-bo");
    await click(screen.getByRole("button", { name: "Back" }));
    await screen.findByRole("button", { name: /Bo research/ });
    await waitFor(() => expect(listSubagentTasksApi).toHaveBeenCalledTimes(2));
    expect(getSubagentTaskMessagesApi).toHaveBeenCalledTimes(1);
    await switchCaller("Ada");
    expect(screen.getByRole("button", { name: /Ada research/ })).toBeTruthy();
    expect(screen.queryByRole("button", { name: /Bo research/ })).toBeNull();
    rerender(view({ ...direct, roomMembers: [...MEMBERS].reverse().map((member) => ({ ...member })) }));
    expect(screen.getByRole("button", { name: /^Switch calling Agent.*Ada/ })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Back" })).toBeNull();
    rerender(view({ ...direct, requestKey: 2 }));
    await screen.findByRole("button", { name: "Back" });
    expect(getSubagentTaskMessagesApi).toHaveBeenCalledTimes(2);
  });

  it("waits for the exact requested task without selecting another task from that caller", async () => {
    vi.mocked(listSubagentTasksApi).mockResolvedValueOnce(response([TASKS[0]]));
    render(view(direct));
    await screen.findByText(MESSAGES.en["subagents.requested_task_missing_title"]);
    expect(getSubagentTaskMessagesApi).not.toHaveBeenCalled();
    await click(screen.getByRole("button", { name: "Retry" }));
    await screen.findByRole("button", { name: "Back" });
    expect(getSubagentTaskMessagesApi).toHaveBeenCalledExactlyOnceWith(SOURCE, "task-bo");
  });

  it("lets manual caller selection cancel a task request that has not arrived", async () => {
    vi.mocked(listSubagentTasksApi).mockResolvedValueOnce(response([TASKS[0]]));
    render(view(direct));
    await screen.findByText(MESSAGES.en["subagents.requested_task_missing_title"]);
    await switchCaller("Ada");
    await click(screen.getByRole("button", { name: /Ada research/ }));
    await screen.findByRole("button", { name: "Back" });
    await click(screen.getByRole("button", { name: "Back" }));
    await screen.findByRole("button", { name: /Ada research/ });
    await switchCaller("Bo");
    expect(screen.getByRole("button", { name: /Bo research/ })).toBeTruthy();
    expect(getSubagentTaskMessagesApi).toHaveBeenCalledExactlyOnceWith(SOURCE, "task-ada");
  });

  it("keeps manual task selection when the earlier target arrives on the return refresh", async () => {
    const other: SubagentTask = { ...TASKS[0], task_id: "task-other", tool_use_id: "tool-other", name: "Other task" };
    vi.mocked(listSubagentTasksApi).mockResolvedValueOnce(response([other])).mockResolvedValue(response([other, TASKS[0]]));
    vi.mocked(getSubagentTaskMessagesApi).mockResolvedValue({ task: other, messages: [] });
    render(view({ requestKey: 1, requestedHostAgentId: "ada", requestedTaskToolUseId: "tool-ada" }));
    await click(await screen.findByRole("button", { name: /Other task/ }));
    await screen.findByRole("button", { name: "Back" });
    await click(screen.getByRole("button", { name: "Back" }));
    await screen.findByRole("button", { name: /Ada research/ });
    expect(screen.getByRole("button", { name: /Other task/ })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Back" })).toBeNull();
    expect(getSubagentTaskMessagesApi).toHaveBeenCalledExactlyOnceWith(SOURCE, "task-other");
  });

  it("consumes old intent on source change and does not replay it when returning", async () => {
    const { rerender } = render(view(direct));
    await screen.findByRole("button", { name: "Back" });
    rerender(view({ ...direct, source: { ...SOURCE, conversation_id: "conversation-two" } }));
    await screen.findByRole("button", { name: /Ada research/ });
    expect(screen.queryByRole("button", { name: "Back" })).toBeNull();
    rerender(view(direct));
    await screen.findByRole("button", { name: /Ada research/ });
    expect(screen.queryByRole("button", { name: "Back" })).toBeNull();
    expect(getSubagentTaskMessagesApi).toHaveBeenCalledTimes(1);
    rerender(view({ ...direct, requestKey: 2 }));
    await screen.findByRole("button", { name: "Back" });
    expect(getSubagentTaskMessagesApi).toHaveBeenCalledTimes(2);
  });

  it("allows late member arrival to resolve a new request, but does not restore a removed manual caller", async () => {
    const { rerender } = render(view({ ...direct, roomMembers: [MEMBERS[0]] }));
    await screen.findByRole("button", { name: /Ada research/ });
    expect(getSubagentTaskMessagesApi).not.toHaveBeenCalled();
    rerender(view(direct));
    await screen.findByRole("button", { name: "Back" });
    await click(screen.getByRole("button", { name: "Back" }));
    await switchCaller("Bo");
    rerender(view({ ...direct, roomMembers: [MEMBERS[0]] }));
    expect(screen.getByRole("button", { name: /Ada research/ })).toBeTruthy();
    rerender(view(direct));
    expect(screen.getByRole("button", { name: /^Switch calling Agent.*Ada/ })).toBeTruthy();
  });

  it("does not treat a missing caller as an unfiltered Room directory", async () => {
    render(view({ roomMembers: [] }));
    await screen.findByText("No active subagents");
    expect(screen.queryByRole("button", { name: /research/ })).toBeNull();
    expect(getSubagentTaskMessagesApi).not.toHaveBeenCalled();
  });

  it("discards a late old-source list and preserves the latest directory", async () => {
    let resolveOld!: (value: SubagentTaskListResponse) => void;
    vi.mocked(listSubagentTasksApi).mockReturnValueOnce(new Promise((resolve) => { resolveOld = resolve; }));
    const { rerender } = render(view());
    const nextSource: SubagentTaskSource = { ...SOURCE, conversation_id: "conversation-two" };
    rerender(view({ source: nextSource, currentAgentId: "bo" }));
    await screen.findByRole("button", { name: /Bo research/ });
    await act(async () => { resolveOld(response([{ ...TASKS[1], name: "Stale private result" }])); });
    expect(screen.queryByText("Stale private result")).toBeNull();
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /Bo research/ }));
    await screen.findByRole("button", { name: "Back" });
    expect(getSubagentTaskMessagesApi).toHaveBeenCalledExactlyOnceWith(nextSource, "task-bo");
  });

  it("keeps the selected task on a list failure, then honors an explicit loss of observation support", async () => {
    const { rerender } = render(view());
    await click(await screen.findByRole("button", { name: /Ada research/ }));
    await screen.findByRole("button", { name: "Back" });
    vi.mocked(listSubagentTasksApi).mockRejectedValueOnce(new Error("offline"));
    await click(screen.getByRole("button", { name: "Back" }));
    await screen.findByText(MESSAGES.en["subagents.list_load_failed_title"]);
    await click(screen.getByRole("button", { name: /Ada research/ }));
    await screen.findByRole("button", { name: "Back" });
    vi.mocked(listSubagentTasksApi).mockResolvedValue({ ...response(), capabilities: { ...CAPABILITIES, observe: false } });
    await click(screen.getByRole("button", { name: "Back" }));
    await screen.findByText(MESSAGES.en["subagents.unsupported_description"]);
    rerender(view(direct));
    expect(screen.queryByRole("button", { name: "Back" })).toBeNull();
    expect(screen.queryByRole("button", { name: /research/ })).toBeNull();
  });

  it("preserves DM navigation without a caller filter and forwards file actions from the real details", async () => {
    const source: SubagentTaskSource = { kind: "session", session_key: "dm-session" };
    const open = vi.fn();
    vi.mocked(getSubagentTaskMessagesApi).mockResolvedValue({ task: TASKS[0], messages: [{ role: "assistant", agent_id: "ada", message_id: "file-message", session_key: "task-session", round_id: "round", timestamp: 1, is_complete: true, content: [{ type: "workspace_file_artifact", path: "result.md", workspace_agent_id: "source-author" }] }] });
    render(view({ source, requestKey: 1, requestedTaskToolUseId: "tool-ada", onOpenWorkspaceFile: open }));
    await click(await screen.findByRole("button", { name: /^result.md/ }));
    expect(open).toHaveBeenCalledExactlyOnceWith("result.md", "source-author");
    expect(screen.queryByRole("button", { name: /^Switch calling Agent/ })).toBeNull();
    await click(screen.getByRole("button", { name: "Back" }));
    expect(within(screen.getByRole("region", { name: "Active" })).getAllByRole("button")).toHaveLength(2);
  });
});
