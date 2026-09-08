// INPUT: 真实控制上下文、会话发布 Hook、live store 与面板只读模型。
// OUTPUT: 精确 Agent round 切换、会话重置、刷新连续和卸载清理的 DOM 回归。
// POS: Thread 生命周期集成；消息数据与资源动作保持原 owner，不新增导航缓存。

import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { GroupThreadContextProvider } from "./group-thread-context";
import { useGroupThread } from "./group-thread-state";
import { useRoomThreadSource } from "./live/use-room-thread-source";
import { useRoomThreadPanel } from "./live/use-room-thread-panel";
import { useRoomThreadLiveStore } from "./live/room-thread-live-store";

const data: Parameters<typeof useRoomThreadSource>[0] = { agentAvatarMap: {}, agentNameMap: { agent: "Nova" }, conversationId: "a", messageGroups: new Map(),
  pendingPermissionGroups: new Map(), pendingSlotGroups: new Map(), roomAgentExecutionStateGroups: new Map(), sendPermissionResponse: vi.fn(() => true) };
function Publisher(props: typeof data) { useRoomThreadSource(props); return null; }
function Reader() {
  const { activeThread, openThread, closeThread } = useGroupThread();
  const panel = useRoomThreadPanel();
  return <><button onClick={() => openThread("root", "agent", "first")}>Open first execution</button>
    <button onClick={() => openThread("root", "agent", "second")}>Open second execution</button>
    <button onClick={() => openThread("root", "agent")}>Open legacy execution</button>
    <button onClick={closeThread}>Close Thread</button>
    <output aria-label="Selection">{activeThread ? JSON.stringify(activeThread) : "closed"}</output>
    <output aria-label="Participant">{panel?.agentName ?? "none"}</output></>;
}
function view(options: Partial<typeof data> = {}, onOpenThread = vi.fn()) {
  return <I18nProvider><GroupThreadContextProvider onOpenThread={onOpenThread}><Publisher {...data} {...options} /><Reader /></GroupThreadContextProvider></I18nProvider>;
}
afterEach(() => act(() => useRoomThreadLiveStore.getState().clearSource()));

it("keeps exact execution targets without remounting readers and preserves ordinary directory refreshes", async () => {
  const user = userEvent.setup();
  const open = vi.fn();
  const rendered = render(view({}, open));
  const first = screen.getByRole("button", { name: "Open first execution" });
  const selection = screen.getByRole("status", { name: "Selection" });
  expect(selection.textContent).toBe("closed");
  await user.click(first);
  expect(selection.textContent).toBe(JSON.stringify({ roundId: "root", agentId: "agent", agentRoundId: "first" }));
  const selected = selection.textContent;
  await user.click(first);
  expect(selection.textContent).toBe(selected);
  rendered.rerender(view({ agentNameMap: { agent: "Pixel" } }, open));
  expect(screen.getByRole("status", { name: "Participant" }).textContent).toBe("Pixel");
  expect(selection.textContent).toBe(selected);
  expect(screen.getByRole("button", { name: "Open first execution" })).toBe(first);
  await user.click(screen.getByRole("button", { name: "Open second execution" }));
  expect(selection.textContent).toContain('"agentRoundId":"second"');
  await user.click(screen.getByRole("button", { name: "Open legacy execution" }));
  expect(selection.textContent).toContain('"agentRoundId":null');
  expect(open).toHaveBeenCalledTimes(4);
  await user.click(screen.getByRole("button", { name: "Close Thread" }));
  expect(selection.textContent).toBe("closed");
});

it("closes A to B to A selections, clears a missing conversation and releases the published source on exit", async () => {
  const user = userEvent.setup();
  const rendered = render(view());
  const first = screen.getByRole("button", { name: "Open first execution" });
  await user.click(first);
  rendered.rerender(view({ conversationId: "b" }));
  expect(screen.getByRole("status", { name: "Selection" }).textContent).toBe("closed");
  rendered.rerender(view());
  expect(screen.getByRole("status", { name: "Selection" }).textContent).toBe("closed");
  expect(screen.getByRole("button", { name: "Open first execution" })).toBe(first);
  await user.click(first);
  rendered.rerender(view({ conversationId: null }));
  expect(screen.getByRole("status", { name: "Selection" }).textContent).toBe("closed");
  expect(useRoomThreadLiveStore.getState().source).not.toBeNull();
  rendered.unmount();
  expect(useRoomThreadLiveStore.getState().source).toBeNull();
});
