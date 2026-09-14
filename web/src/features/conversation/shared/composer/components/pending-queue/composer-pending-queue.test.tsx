// INPUT: 有序队列快照、拖动/键盘/折叠事件与 Conversation 命令。
// OUTPUT: 证明公共 Disclosure/Menu/Button 保持精确行命令和当前顺序，无私有乐观重排。
// POS: Queue 真实组件集成回归；浏览器拖动手感和视觉验收仍单独记录。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { InputQueueItem } from "@/types/agent/agent-conversation";
import { ComposerPendingQueue } from "./composer-pending-queue";

const queue: InputQueueItem[] = ["Alpha", "Bravo", "Charlie"].map((content, index) => ({
  id: String(index), scope: "dm", session_key: "session-a", source: "user", content,
  delivery_policy: "queue", created_at: 1, updated_at: 1,
}));
const commands = () => ({ onDeleteQueuedMessage: vi.fn(), onGuideQueuedMessage: vi.fn(), onReorderQueueMessages: vi.fn() });
const wrap = (items: InputQueueItem[], actions = commands()) => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
  <ComposerPendingQueue compact={false} inputQueueItems={items} {...actions} />
</I18N_CONTEXT.Provider>;

describe("ComposerPendingQueue", () => {
  it("uses a native disclosure and a named ordered list, keeping queued content when collapsed", async () => {
    const user = userEvent.setup();
    const { container, rerender } = render(wrap(queue));
    const details = container.querySelector("details")!;
    const summary = container.querySelector("summary")!;
    expect(details.open).toBe(true);
    expect(screen.getByRole("list", { name: "composer.pending_queue" }).tagName).toBe("OL");
    expect(screen.getAllByRole("listitem")).toHaveLength(3);
    await user.click(summary);
    expect(details.open).toBe(false);
    expect(details.textContent).toContain("Bravo");
    await user.click(summary);
    expect(details.open).toBe(true);
    rerender(wrap([]));
    expect(container.querySelector("details")).toBeNull();
  });

  it("moves by keyboard through the shared menu, preserving server-owned order and focus", async () => {
    const user = userEvent.setup();
    const actions = commands();
    const { rerender } = render(wrap(queue, actions));
    const first = screen.getAllByRole("listitem")[0];
    const handle = within(first).getByRole("button", { name: "composer.reorder_pending" });
    act(() => handle.focus());
    await user.keyboard("{Enter}");
    expect((screen.getByRole("menuitem", { name: "composer.move_pending_up" }) as HTMLButtonElement).disabled).toBe(true);
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "composer.move_pending_down" }));
    await user.keyboard("{Enter}");
    expect(actions.onReorderQueueMessages).toHaveBeenCalledExactlyOnceWith(["1", "0", "2"]);
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(handle);
    expect(screen.getAllByRole("listitem")[0]).toBe(first);
    rerender(wrap([queue[1], queue[0], queue[2]], actions));
    expect(screen.getAllByRole("listitem")[1]).toBe(first);
    expect(document.activeElement).toBe(handle);
  });

  it("drags only the dedicated handle and dispatches a valid drop once", async () => {
    const actions = commands();
    render(wrap(queue, actions));
    const rows = screen.getAllByRole("listitem");
    const handle = within(rows[0]).getByRole("button", { name: "composer.reorder_pending" });
    expect(rows[0].draggable).toBe(false);
    expect(handle.draggable).toBe(true);
    const dataTransfer = { effectAllowed: "", setData: vi.fn() };
    fireEvent.dragStart(handle, { dataTransfer });
    expect(dataTransfer.effectAllowed).toBe("move");
    expect(dataTransfer.setData).toHaveBeenCalledWith("application/x-nexus-input-queue", "0");
    await act(async () => { fireEvent.drop(rows[2]); });
    fireEvent.dragEnd(handle);
    fireEvent.drop(rows[1]);
    expect(actions.onReorderQueueMessages).toHaveBeenCalledExactlyOnceWith(["1", "2", "0"]);
  });

  it("describes the exact message for actions and shows the full long content", async () => {
    const user = userEvent.setup();
    const actions = commands();
    const text = "A long queued message ".repeat(30);
    render(wrap([{ ...queue[0], content: text, delivery_policy: "guide" }], actions));
    const content = screen.getByText(text.trim());
    const row = screen.getByRole("listitem");
    const guide = within(row).getByRole("button", { name: "composer.cancel_guidance" });
    const remove = within(row).getByRole("button", { name: "composer.delete_pending" });
    expect(guide.getAttribute("aria-describedby")).toContain(content.id);
    expect(remove.getAttribute("aria-describedby")).toContain(content.id);
    expect((within(row).getByRole("button", { name: "composer.reorder_pending" }) as HTMLButtonElement).disabled).toBe(true);
    await user.click(guide);
    expect(actions.onGuideQueuedMessage).toHaveBeenCalledExactlyOnceWith("0");
    await user.click(remove);
    expect(actions.onDeleteQueuedMessage).toHaveBeenCalledExactlyOnceWith("0");
  });

  it("closes sorting when no neighbor remains and never reopens it when new items arrive", async () => {
    const user = userEvent.setup();
    const actions = commands();
    const { rerender } = render(wrap(queue, actions));
    await user.click(within(screen.getAllByRole("listitem")[0]).getByRole("button", { name: "composer.reorder_pending" }));
    expect(screen.getByRole("menu")).toBeTruthy();
    rerender(wrap([queue[0]], actions));
    expect(screen.queryByRole("menu")).toBeNull();
    rerender(wrap(queue, actions));
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("shows attachment names or a meaningful fallback when a queued item has no text", () => {
    const attachment = { kind: "file" as const, file_name: "report.csv", workspace_path: "files/report.csv" };
    const { rerender } = render(wrap([{ ...queue[0], content: "  ", attachments: [attachment] }]));
    expect(screen.getByText("report.csv")).toBeTruthy();
    rerender(wrap([{ ...queue[0], content: "Message wins", attachments: [attachment] }]));
    expect(screen.getByText("Message wins")).toBeTruthy();
    expect(screen.queryByText("report.csv")).toBeNull();
    rerender(wrap([{ ...queue[0], content: "" }]));
    expect(screen.getByText("composer.pending_message")).toBeTruthy();
  });

  it("disables conflicting row commands only while dispatch is pending", async () => {
    const user = userEvent.setup();
    let finish!: () => void;
    const actions = commands();
    actions.onGuideQueuedMessage.mockImplementation(() => new Promise<void>((resolve) => { finish = resolve; }));
    render(wrap(queue, actions));
    await user.click(within(screen.getAllByRole("listitem")[0]).getByRole("button", { name: "composer.mark_guidance" }));
    for (const row of screen.getAllByRole("listitem")) {
      for (const button of within(row).getAllByRole("button")) expect((button as HTMLButtonElement).disabled).toBe(true);
    }
    await act(async () => { finish(); });
    expect((screen.getAllByRole("button", { name: "composer.delete_pending" })[1] as HTMLButtonElement).disabled).toBe(false);
    expect(actions.onDeleteQueuedMessage).not.toHaveBeenCalled();
  });
});
