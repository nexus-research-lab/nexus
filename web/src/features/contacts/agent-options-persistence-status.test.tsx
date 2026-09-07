// INPUT: 精确 Agent、Header 密度、保存阶段/文案与用户点击、键盘和外部动作。
// OUTPUT: 证明完整保存反馈、共享 Portal 退出/焦点/模态边界和过期详情失效。
// POS: Contacts 保存状态离线 DOM 回归；不触发保存，不代替浏览器视觉验收。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";
import { UiDialogBackdrop, UiDialogPortal, UiDialogShell } from "@/shared/ui/dialog/dialog";

import { AgentOptionsPersistenceStatus } from "./agent-options-persistence-status";

const error = { message: "保存失败：服务暂时不可用，已有配置保留，请稍后重试。", phase: "error" as const };
beforeEach(() => {
  localStorage.setItem(LOCALE_STORAGE_KEY, "zh");
  // jsdom 不计算布局，只向共享焦点目录提供可见控件的矩形。
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([new DOMRect(0, 0, 100, 32)] as unknown as DOMRectList);
});
afterEach(() => vi.restoreAllMocks());

describe("AgentOptionsPersistenceStatus", () => {
  it("announces the complete status once and uses semantic type and reduced-motion loading", () => {
    const { container } = render(<AgentOptionsPersistenceStatus agentId="a" compact={false} state={{ message: "正在保存", phase: "saving" }} />, { wrapper: I18nProvider });
    const status = screen.getByRole("status");
    expect(status.textContent).toBe("正在保存");
    expect(status.parentElement?.className).toContain("ui-type-metadata");
    expect(status.parentElement?.className).toContain("ui-type-tone-muted");
    expect(container.querySelector("svg.animate-spin")?.getAttribute("class")).toContain("motion-reduce:animate-none");
    expect(screen.queryByRole("button")).toBeNull();
  });

  it.each([false, true])("opens complete error details with compact=%s and returns focus on Escape", async (compact) => {
    const user = userEvent.setup();
    const { container } = render(<AgentOptionsPersistenceStatus agentId="a" compact={compact} state={error} />, { wrapper: I18nProvider });
    const trigger = screen.getByRole("button", { name: "保存状态" });
    expect(document.getElementById(trigger.getAttribute("aria-describedby")!)?.textContent).toBe(error.message);
    expect(trigger.getAttribute("title")).toBeNull();
    await user.click(trigger);
    const popup = screen.getByRole("dialog", { name: "保存状态" });
    expect(container.contains(popup)).toBe(false);
    expect(trigger.getAttribute("aria-controls")).toBe(popup.id);
    expect(popup.textContent).toBe(error.message);
    expect(popup.className).toContain("ui-type-supporting");
    expect(popup.className).toContain("overflow-y-auto");
    expect(popup.getAttribute("aria-hidden")).toBeNull();
    expect(document.activeElement).toBe(popup);
    fireEvent.keyDown(popup, { key: "Tab", isComposing: true });
    expect(document.activeElement).toBe(popup);
    expect(screen.queryByRole("tooltip")).toBeNull();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

  it("preserves an uncertain save result without labelling it as a definite failure", async () => {
    const user = userEvent.setup();
    const message = "保存结果需要确认。请先刷新配置，不要重复提交。";
    render(<AgentOptionsPersistenceStatus agentId="a" compact state={{ message, phase: "error" }} />, { wrapper: I18nProvider });
    await user.click(screen.getByRole("button", { name: "保存状态" }));
    expect(screen.getByRole("dialog", { name: "保存状态" }).textContent).toBe(message);
    expect(screen.queryByText("保存失败")).toBeNull();
  });

  it.each([false, true])("continues from the trigger on Tab with shift=%s", async (shift) => {
    const user = userEvent.setup();
    render(<><button>Before</button><AgentOptionsPersistenceStatus agentId="a" compact state={error} /><button>After</button></>, { wrapper: I18nProvider });
    await user.click(screen.getByRole("button", { name: "保存状态" }));
    await user.tab({ shift });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: shift ? "Before" : "After" }));
  });

  it("closes on outside press without taking focus from the clicked target", async () => {
    const user = userEvent.setup();
    render(<><AgentOptionsPersistenceStatus agentId="a" compact state={error} /><button>Outside</button></>, { wrapper: I18nProvider });
    await user.click(screen.getByRole("button", { name: "保存状态" }));
    const outside = screen.getByRole("button", { name: "Outside" });
    await user.click(outside);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(outside);
  });

  it.each(["agent", "phase", "message", "layout"])("consumes old detail state when %s changes and does not resurrect it", async (boundary) => {
    const user = userEvent.setup();
    const view = render(<AgentOptionsPersistenceStatus agentId="a" compact state={error} />, { wrapper: I18nProvider });
    await user.click(screen.getByRole("button", { name: "保存状态" }));
    view.rerender(<AgentOptionsPersistenceStatus agentId={boundary === "agent" ? "b" : "a"} compact={boundary !== "layout"}
      state={boundary === "phase" ? { message: "已保存", phase: "success" } : boundary === "message" ? { ...error, message: "新的保存问题" } : error} />);
    expect(screen.queryByRole("dialog")).toBeNull();
    view.rerender(<AgentOptionsPersistenceStatus agentId="a" compact state={error} />);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("button", { name: "保存状态" }).getAttribute("aria-controls")).toBeNull();
  });

  it("keeps details in the current modal and consumes only the first Escape", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<UiDialogPortal><UiDialogBackdrop aria-label="设置" onClose={onClose}><UiDialogShell>
      <AgentOptionsPersistenceStatus agentId="a" compact state={error} />
    </UiDialogShell></UiDialogBackdrop></UiDialogPortal>, { wrapper: I18nProvider });
    const parent = screen.getByRole("dialog", { name: "设置" });
    await user.click(screen.getByRole("button", { name: "保存状态" }));
    expect(parent.contains(screen.getByRole("dialog", { name: "保存状态" }))).toBe(true);
    await user.keyboard("{Escape}");
    expect(onClose).not.toHaveBeenCalled();
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledOnce();
  });
});
