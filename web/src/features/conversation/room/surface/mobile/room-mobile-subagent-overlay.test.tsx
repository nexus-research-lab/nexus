// INPUT: Real narrow task directory/details with menu, prompt and outer modal nesting.
// OUTPUT: Localized dialog naming, focus containment, one-layer Escape, scroll lock and return focus.
// POS: Offline mobile surface integration; only subagent HTTP/realtime transport is mocked.

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ReactNode } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { getSubagentTaskMessagesApi, listSubagentTasksApi, sendSubagentTaskMessageApi, stopSubagentTaskApi } from "@/lib/api/conversation/subagent-task-api";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { Agent } from "@/types/agent/agent";
import type { SubagentTask, SubagentTaskSource } from "@/types/conversation/subagent-task";
import { RoomMobileSubagentOverlay } from "./room-mobile-subagent-overlay";

vi.mock("@/lib/api/conversation/subagent-task-api", () => ({ listSubagentTasksApi: vi.fn(), getSubagentTaskMessagesApi: vi.fn(), sendSubagentTaskMessageApi: vi.fn(), stopSubagentTaskApi: vi.fn() }));
vi.mock("@/features/conversation/shared/subagent/use-subagent-task-realtime-refresh", () => ({ useSubagentTaskRealtimeRefresh: vi.fn() }));
const MEMBERS: Agent[] = ["Ada", "Bo"].map((name) => ({ agent_id: name.toLowerCase(), name, workspace_path: "", options: {}, created_at: 1, status: "idle" }));
const TASK: SubagentTask = { task_id: "task-ada", tool_use_id: "tool-ada", host_agent_id: "ada", name: "Research", status: "running", runtime_kind: "nxs", capabilities: { observe: true, transcript: true, stop: true, resume: true, send_message: true } };
const SOURCE: SubagentTaskSource = { kind: "room", room_id: "room", conversation_id: "conversation" };
const localized = (children: ReactNode, locale: "en" | "zh" = "en") => {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>{children}</I18N_CONTEXT.Provider>;
};
function Harness() {
  const [open, setOpen] = useState(false);
  return localized(<><button onClick={() => setOpen(true)}>Open tasks</button><RoomMobileSubagentOverlay currentAgentId="ada" roomMembers={MEMBERS} source={open ? SOURCE : null} onClose={() => setOpen(false)} /></>);
}
beforeEach(() => {
  vi.mocked(listSubagentTasksApi).mockResolvedValue({ items: [TASK], runtime_kind: "nxs", capabilities: TASK.capabilities });
  vi.mocked(getSubagentTaskMessagesApi).mockResolvedValue({ task: TASK, messages: [] });
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([{} as DOMRect] as unknown as DOMRectList);
});
afterEach(() => { vi.resetAllMocks(); vi.restoreAllMocks(); });

it.each(["en", "zh"] as const)("names the mounted modal in %s and leaves a missing source unmounted", async (locale) => {
  const view = (source: SubagentTaskSource | null) => localized(<RoomMobileSubagentOverlay currentAgentId="ada" roomMembers={MEMBERS} source={source} onClose={vi.fn()} />, locale);
  const { rerender, unmount } = render(view(null));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(listSubagentTasksApi).not.toHaveBeenCalled();
  rerender(view(SOURCE));
  const dialog = screen.getByRole("dialog", { name: MESSAGES[locale]["subagents.panel_title"] });
  expect(dialog.getAttribute("aria-modal")).toBe("true");
  expect(dialog.className).toContain("min-h-0");
  await screen.findByRole("button", { name: /Research/ });
  unmount();
  expect(document.body.style.overflow).not.toBe("hidden");
});

it("traps Tab, closes the member menu before the task modal, and restores the opener", async () => {
  const user = userEvent.setup();
  const overflow = document.body.style.overflow;
  render(<Harness />);
  const trigger = screen.getByRole("button", { name: "Open tasks" });
  await user.click(trigger);
  const dialog = screen.getByRole("dialog", { name: "Subagents" });
  const back = within(dialog).getByRole("button", { name: "Back" });
  await screen.findByRole("button", { name: /Research/ });
  await waitFor(() => expect(document.activeElement).toBe(back));
  expect(document.body.style.overflow).toBe("hidden");
  await user.keyboard("{Shift>}{Tab}{/Shift}");
  expect(document.activeElement).toBe(screen.getByRole("button", { name: /Research/ }));
  await user.keyboard("{Tab}");
  expect(document.activeElement).toBe(back);
  await user.click(screen.getByRole("button", { name: /^Switch calling Agent/ }));
  await screen.findByRole("menu", { name: "Switch calling Agent" });
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("menu")).toBeNull();
  expect(screen.getByRole("dialog", { name: "Subagents" })).toBe(dialog);
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(trigger);
  expect(document.body.style.overflow).toBe(overflow);
});

it("closes only the nested instruction prompt and keeps task navigation inside the outer modal", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const trigger = screen.getByRole("button", { name: "Open tasks" });
  await user.click(trigger);
  const outer = screen.getByRole("dialog", { name: "Subagents" });
  await user.click(await screen.findByRole("button", { name: /Research/ }));
  const send = await screen.findByRole("button", { name: "Send instruction" });
  await waitFor(() => expect(document.activeElement).toBe(within(outer).getByRole("button", { name: "Back" })));
  await user.click(send);
  const prompt = screen.getAllByRole("dialog").find((dialog) => dialog !== outer)!;
  fireEvent.change(within(prompt).getByRole("textbox"), { target: { value: "Unsent draft" } });
  await user.keyboard("{Escape}");
  expect(screen.getAllByRole("dialog")).toEqual([outer]);
  expect(document.activeElement).toBe(send);
  expect(sendSubagentTaskMessageApi).not.toHaveBeenCalled();
  expect(stopSubagentTaskApi).not.toHaveBeenCalled();
  await user.click(within(outer).getByRole("button", { name: "Back" }));
  const returnedRow = await screen.findByRole("button", { name: /Research/ });
  expect(document.activeElement).toBe(returnedRow);
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(trigger);
});
