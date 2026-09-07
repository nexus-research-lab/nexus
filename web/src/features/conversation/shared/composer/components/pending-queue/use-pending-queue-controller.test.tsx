// INPUT: 当前队列、原生拖动事件、动画帧与异步队列命令。
// OUTPUT: 证明只派发有效重排、命令防重及拖动消失/边缘滚动的清理。
// POS: Queue 控制器行为回归；命令受理与故障投影仍属于 Conversation transport。

import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { InputQueueItem } from "@/types/agent/agent-conversation";
import { usePendingQueueController } from "./use-pending-queue-controller";

const items: InputQueueItem[] = ["a", "b", "c"].map((id) => ({
  id, scope: "dm", session_key: "session-a", source: "user", content: id,
  delivery_policy: "queue", created_at: 1, updated_at: 1,
}));
const makeCommands = () => ({ deleteMessage: vi.fn(), guideMessage: vi.fn(), reorderMessages: vi.fn() });
afterEach(() => vi.restoreAllMocks());

describe("pending queue controller", () => {
  it("does not send no-op, foreign or stale drag orders", () => {
    const commands = makeCommands();
    const { result, rerender } = renderHook(({ queue }) => usePendingQueueController({ commands, items: queue }), { initialProps: { queue: items } });
    act(() => result.current.actions.startDrag("a"));
    act(() => result.current.actions.dropOnMessage("a"));
    act(() => result.current.actions.startDrag("a"));
    act(() => result.current.actions.dropOnMessage("missing"));
    expect(commands.reorderMessages).not.toHaveBeenCalled();
    act(() => result.current.actions.startDrag("a"));
    rerender({ queue: items.slice(1) });
    expect(result.current.state.dragState.draggingMessageId).toBeNull();
    act(() => result.current.actions.dropOnMessage("b"));
    expect(commands.reorderMessages).not.toHaveBeenCalled();
  });

  it("serializes dispatch synchronously across guide, deletion and reordering", async () => {
    let finish!: () => void;
    const commands = makeCommands();
    commands.guideMessage.mockImplementation(() => new Promise<void>((resolve) => { finish = resolve; }));
    const { result } = renderHook(() => usePendingQueueController({ commands, items }));
    act(() => result.current.actions.startDrag("c"));
    act(() => {
      void result.current.actions.guideMessage("a");
      void result.current.actions.guideMessage("b");
      void result.current.actions.deleteMessage("c");
      result.current.actions.startDrag("b");
    });
    expect(commands.guideMessage.mock.calls).toEqual([["a"]]);
    expect(commands.deleteMessage).not.toHaveBeenCalled();
    expect(result.current.state.dragState.draggingMessageId).toBeNull();
    await act(async () => { finish(); });
    expect(result.current.state.isActionRunning).toBe(false);
    await act(async () => { await result.current.actions.deleteMessage("c"); });
    expect(commands.deleteMessage).toHaveBeenCalledExactlyOnceWith("c");
  });

  it("uses current neighbors for keyboard moves and preserves server-owned order", async () => {
    const commands = makeCommands();
    const { result, rerender } = renderHook(({ queue }) => usePendingQueueController({ commands, items: queue }), { initialProps: { queue: items } });
    await act(async () => { await result.current.actions.moveMessage("b", -1); });
    expect(commands.reorderMessages.mock.calls).toEqual([[["b", "a", "c"]]]);
    expect(items.map((item) => item.id)).toEqual(["a", "b", "c"]);
    rerender({ queue: [items[2], items[1], items[0]] });
    await act(async () => { await result.current.actions.moveMessage("b", 1); });
    expect(commands.reorderMessages.mock.calls[1]).toEqual([["c", "a", "b"]]);
    await act(async () => { await result.current.actions.moveMessage("c", -1); });
    expect(commands.reorderMessages).toHaveBeenCalledTimes(2);
  });

  it("settles rejected dispatches without replaying and keeps later commands usable", async () => {
    const commands = makeCommands();
    const failure = new Error("already projected by transport");
    const log = vi.spyOn(console, "error").mockImplementation(() => undefined);
    commands.deleteMessage.mockRejectedValueOnce(failure);
    const { result } = renderHook(() => usePendingQueueController({ commands, items }));
    await act(async () => { await result.current.actions.deleteMessage("a"); });
    expect(commands.deleteMessage).toHaveBeenCalledOnce();
    expect(result.current.state.isActionRunning).toBe(false);
    expect(log).toHaveBeenCalled();
    await act(async () => { await result.current.actions.guideMessage("b"); });
    expect(commands.guideMessage).toHaveBeenCalledExactlyOnceWith("b");
  });

  it("does not scroll for foreign drags, clamps at an edge and cancels on removal/unmount", () => {
    let frame: FrameRequestCallback | undefined;
    const request = vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback) => { frame = callback; return 12; });
    const cancel = vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => undefined);
    const { result, rerender, unmount } = renderHook(({ queue }) => usePendingQueueController({ commands: makeCommands(), items: queue }), { initialProps: { queue: items } });
    const container = document.createElement("ol");
    Object.defineProperties(container, { clientHeight: { value: 100 }, scrollHeight: { value: 105 } });
    vi.spyOn(container, "getBoundingClientRect").mockReturnValue(new DOMRect(0, 0, 200, 100));
    result.current.refs.scrollRef.current = container;
    act(() => result.current.actions.startAutoScroll(99));
    expect(request).not.toHaveBeenCalled();
    act(() => result.current.actions.startDrag("a"));
    act(() => result.current.actions.startAutoScroll(99));
    act(() => frame?.(0));
    expect(container.scrollTop).toBe(5);
    act(() => frame?.(1));
    const frameCount = request.mock.calls.length;
    act(() => frame?.(2));
    expect(request).toHaveBeenCalledTimes(frameCount);
    act(() => result.current.actions.startAutoScroll(1));
    rerender({ queue: items.slice(1) });
    expect(cancel).toHaveBeenCalledWith(12);
    act(() => result.current.actions.startDrag("b"));
    act(() => result.current.actions.startAutoScroll(1));
    cancel.mockClear();
    unmount();
    expect(cancel).toHaveBeenCalledWith(12);
  });
});
