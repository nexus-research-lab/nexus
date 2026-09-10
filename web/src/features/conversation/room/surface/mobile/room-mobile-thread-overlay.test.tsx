// INPUT: 真实 Thread 控制器/live store、精确目标与当前语言。
// OUTPUT: 窄窗 Thread 模态命名、缺源卸载、焦点退出和稳定名称更新回归。
// POS: Thread 挂载集成测试；使用真实空消息面板，不伪造来源或执行命令。

import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { GroupThreadContextProvider } from "../../group/thread/group-thread-context";
import { useGroupThread } from "../../group/thread/group-thread-state";
import { useRoomThreadLiveStore, type RoomThreadLiveSource } from "../../group/thread/live/room-thread-live-store";
import { RoomMobileThreadOverlay } from "./room-mobile-thread-overlay";

const source = (name?: string): RoomThreadLiveSource => ({ agentAvatarMap: {}, agentNameMap: name ? { "private-id": name } : {}, messageGroups: new Map(),
  pendingPermissionGroups: new Map(), pendingSlotGroups: new Map(), roomAgentExecutionStateGroups: new Map(), onPermissionResponse: vi.fn(() => true) });
beforeEach(() => {
  useRoomThreadLiveStore.getState().clearSource();
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([{} as DOMRect] as unknown as DOMRectList);
});
afterEach(() => { act(() => useRoomThreadLiveStore.getState().clearSource()); vi.restoreAllMocks(); });
function Entry() {
  const { openThread } = useGroupThread();
  return <><button onClick={() => openThread("round", "private-id", "agent-round")}>Inspect turn</button><RoomMobileThreadOverlay /></>;
}
function view(locale: "en" | "zh" = "en") {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((value, [name, replacement]) => value.replaceAll(`{${name}}`, String(replacement)), MESSAGES[locale][key]);
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}><GroupThreadContextProvider><Entry /></GroupThreadContextProvider></I18N_CONTEXT.Provider>;
}

it.each(["en", "zh"] as const)("names the real Thread in %s without exposing an Agent ID and restores its trigger", async (locale) => {
  useRoomThreadLiveStore.getState().setSource(source());
  const user = userEvent.setup();
  render(view(locale));
  const trigger = screen.getByRole("button", { name: "Inspect turn" });
  await user.click(trigger);
  const name = MESSAGES[locale]["room.thread_dialog"].replace("{name}", MESSAGES[locale]["agent.name_fallback"]);
  const dialog = screen.getByRole("dialog", { name });
  expect(dialog.textContent).not.toContain("private-id");
  const back = screen.getByRole("button", { name: MESSAGES[locale]["common.back"] });
  await waitFor(() => expect(document.activeElement).toBe(back));
  expect(dialog.querySelector("header")?.getAttribute("data-desktop-window-drag-region")).not.toBeNull();
  await screen.findByRole("tooltip");
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("tooltip")).toBeNull();
  expect(screen.getByRole("dialog")).toBe(dialog);
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(trigger);
});

it("does not lock without live data, opens when data arrives and keeps name refreshes in place", async () => {
  const previousOverflow = document.body.style.overflow;
  render(view());
  await userEvent.click(screen.getByRole("button", { name: "Inspect turn" }));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.body.style.overflow).toBe(previousOverflow);
  act(() => useRoomThreadLiveStore.getState().setSource(source("Nova")));
  const dialog = screen.getByRole("dialog", { name: "Execution details for Nova" });
  const back = screen.getByRole("button", { name: "Back" });
  await waitFor(() => expect(document.activeElement).toBe(back));
  act(() => useRoomThreadLiveStore.getState().setSource(source("Pixel")));
  expect(screen.getByRole("dialog", { name: "Execution details for Pixel" })).toBe(dialog);
  expect(document.activeElement).toBe(back);
  await userEvent.click(back);
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.body.style.overflow).toBe(previousOverflow);
});
