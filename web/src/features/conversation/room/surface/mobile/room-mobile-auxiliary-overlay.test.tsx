// INPUT: 窄窗辅助页模式、现有空 WorkGraph、平台 Header 与原页面命令。
// OUTPUT: 三种辅助页共用模态、空资源保持打开、切页焦点重置与文件动作透传回归。
// POS: 辅助层装配测试；Workspace/About 以类型化边界替身隔离文件和编辑器 API。

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ComponentProps } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { RoomWorkspaceView } from "../../workspace/room-workspace-view";
import type { RoomAgentAboutSurface } from "../room-agent-about-surface";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { RoomMobileAuxiliaryOverlay, type RoomMobileAuxiliaryTab } from "./room-mobile-auxiliary-overlay";

vi.mock("../../workspace/room-workspace-view", () => ({ RoomWorkspaceView: ({ onOpenWorkspaceFile }: ComponentProps<typeof RoomWorkspaceView>) =>
  <button onClick={() => onOpenWorkspaceFile("/report.md")}>Open file</button> }));
vi.mock("../room-agent-about-surface", () => ({ RoomAgentAboutSurface: ({ agent }: ComponentProps<typeof RoomAgentAboutSurface>) => <button>{agent.name} profile</button> }));

const agent = { agent_id: "agent", name: "Nova", workspace_path: "/workspace", created_at: 1, options: {}, status: "idle" as const };
type Props = ComponentProps<typeof RoomMobileAuxiliaryOverlay>;
const base: Omit<Props, "activeTab" | "onClose"> = {
  activeWorkspacePath: null, composerDraftScopeKey: "draft", conversationId: "conversation", currentAgent: agent,
  executionResource: { dismiss: vi.fn(), error: null, execution: null, isLoading: false, isStale: false, lastSuccessfulAt: null, refresh: vi.fn(), sessionKey: null },
  executionTaskRuns: [], isDm: false, onOpenWorkspaceFile: vi.fn(), onSaveAgentOptions: vi.fn(async () => undefined), onValidateAgentName: vi.fn(), roomId: "room", roomMembers: [agent],
};
const titleKey = { about: "room.about", workgraph: "room.workgraph", workspace: "room.workspace" } as const;
beforeEach(() => { vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([{} as DOMRect] as unknown as DOMRectList); });
afterEach(() => vi.restoreAllMocks());

function Harness({ tab, locale = "en" }: { tab: RoomMobileAuxiliaryTab; locale?: "en" | "zh" }) {
  const [open, setOpen] = useState(false);
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
    <button onClick={() => setOpen(true)}>Open page</button>
    <RoomMobileAuxiliaryOverlay {...base} activeTab={open ? tab : null} onClose={() => setOpen(false)} />
  </I18N_CONTEXT.Provider>;
}

it.each(["about", "workspace", "workgraph"] as const)("keeps the %s page in one named modal and returns to its original trigger", async (tab) => {
  const user = userEvent.setup();
  const previousOverflow = document.body.style.overflow;
  render(<Harness tab={tab} />);
  expect(screen.queryByRole("dialog")).toBeNull();
  const trigger = screen.getByRole("button", { name: "Open page" });
  await user.click(trigger);
  const dialog = screen.getByRole("dialog", { name: MESSAGES.en[titleKey[tab]] });
  expect(dialog.getAttribute("aria-modal")).toBe("true");
  const header = dialog.querySelector("header")!;
  expect(header.className).toContain("h-[var(--mobile-shell-header-height,52px)]");
  expect(header.hasAttribute("data-desktop-window-drag-region")).toBe(true);
  const back = within(dialog).getByRole("button", { name: "Back" });
  await waitFor(() => expect(document.activeElement).toBe(back));
  if (tab === "workgraph") expect(within(dialog).getByText(MESSAGES.en["execution.surface_empty"])).toBeTruthy();
  await user.click(back);
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(trigger);
  expect(document.body.style.overflow).toBe(previousOverflow);
});

it("keeps localized title updates stable and renews focus when replacing an auxiliary page", async () => {
  const user = userEvent.setup();
  const rendered = render(<Harness tab="about" />);
  const trigger = screen.getByRole("button", { name: "Open page" });
  await user.click(trigger);
  const dialog = screen.getByRole("dialog", { name: MESSAGES.en["room.about"] });
  await user.click(screen.getByRole("button", { name: "Nova profile" }));
  const focused = document.activeElement;
  rendered.rerender(<Harness tab="about" locale="zh" />);
  expect(screen.getByRole("dialog", { name: MESSAGES.zh["room.about"] })).toBe(dialog);
  expect(document.activeElement).toBe(focused);
  rendered.rerender(<Harness tab="workspace" locale="zh" />);
  expect(screen.getByRole("dialog", { name: MESSAGES.zh["room.workspace"] })).not.toBe(dialog);
  const back = screen.getByRole("button", { name: MESSAGES.zh["common.back"] });
  await waitFor(() => expect(document.activeElement).toBe(back));
  await user.click(back);
  expect(document.activeElement).toBe(trigger);
});

it("forwards the original file navigation command and releases the modal on external close", async () => {
  const openFile = vi.fn();
  const close = vi.fn();
  const previousOverflow = document.body.style.overflow;
  const view = (activeTab: RoomMobileAuxiliaryTab | null) => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => MESSAGES.en[key] }}>
    <RoomMobileAuxiliaryOverlay {...base} activeTab={activeTab} onClose={close} onOpenWorkspaceFile={openFile} />
  </I18N_CONTEXT.Provider>;
  const rendered = render(view("workspace"));
  await userEvent.click(screen.getByRole("button", { name: "Open file" }));
  expect(openFile).toHaveBeenCalledExactlyOnceWith("/report.md");
  expect(close).not.toHaveBeenCalled();
  rendered.rerender(view(null));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.body.style.overflow).toBe(previousOverflow);
});
