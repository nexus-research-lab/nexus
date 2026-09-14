// INPUT: Task/source changes and user intents entering real confirmation and prompt dialogs.
// OUTPUT: Only explicit confirmation invokes the exact controller; scope changes clear pending local dialogs.
// POS: Task interaction regression; the resource/mutation hook is the only replaced boundary.

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { SubagentTask, SubagentTaskSource } from "@/types/conversation/subagent-task";
import { createSubagentTaskThreadScope } from "./subagent-task-thread-model";
import { useSubagentTaskThread } from "./use-subagent-task-thread";
import { SubagentTaskThread } from "./subagent-task-thread";

vi.mock("./use-subagent-task-thread", () => ({ useSubagentTaskThread: vi.fn() }));

const TASK: SubagentTask = { task_id: "task-one", name: "Research", runtime_kind: "nxs", status: "running", capabilities: { observe: true, transcript: true, resume: true, send_message: true, stop: true } };
const SOURCE: SubagentTaskSource = { kind: "session", session_key: "session-one" };
const stop = vi.fn().mockResolvedValue(true);
const send = vi.fn().mockResolvedValue(true);

function view(task = TASK, source = SOURCE) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => MESSAGES.en[key] }}><SubagentTaskThread source={source} task={task} onBack={vi.fn()} /></I18N_CONTEXT.Provider>;
}

beforeEach(() => {
  vi.mocked(useSubagentTaskThread).mockImplementation(({ source, task }) => ({
    actions: { stop, send, pendingAction: null, error: null, feedback: null }, detail: null, error: null, isLoading: false, messages: [], rounds: [], refresh: vi.fn().mockResolvedValue(undefined), task, sessionKey: createSubagentTaskThreadScope(source, task).key,
  }));
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([{} as DOMRect] as unknown as DOMRectList);
});
afterEach(() => {
  vi.clearAllMocks();
  vi.restoreAllMocks();
});

describe("Subagent task confirmations", () => {
  it("requires confirmation before stopping and never sends on cancel", async () => {
    const user = userEvent.setup();
    render(view());
    await user.click(screen.getByRole("button", { name: "Stop" }));
    expect(stop).not.toHaveBeenCalled();
    let dialog = screen.getByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(stop).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Stop" }));
    dialog = screen.getByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Stop task" }));
    expect(stop).toHaveBeenCalledOnce();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("sends the entered instruction once through the shared prompt", async () => {
    const user = userEvent.setup();
    render(view());
    await user.click(screen.getByRole("button", { name: "Send instruction" }));
    const dialog = screen.getByRole("dialog");
    fireEvent.change(within(dialog).getByRole("textbox"), { target: { value: "Review the final evidence." } });
    await user.click(within(dialog).getByRole("button", { name: MESSAGES.en["subagents.message_send"] }));
    expect(send).toHaveBeenCalledExactlyOnceWith("Review the final evidence.");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it.each(["task", "source"])("clears the open prompt immediately on %s change and does not restore it on return", async (change) => {
    const user = userEvent.setup();
    const { rerender } = render(view());
    await user.click(screen.getByRole("button", { name: "Send instruction" }));
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "Private draft for task one" } });
    rerender(change === "task" ? view({ ...TASK, task_id: "task-two" }) : view(TASK, { kind: "session", session_key: "session-two" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    rerender(view());
    expect(screen.queryByRole("dialog")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Send instruction" }));
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("");
    expect(send).not.toHaveBeenCalled();
    expect(stop).not.toHaveBeenCalled();
  });
});
