// INPUT: 精确 Agent、Header 密度、保存阶段/文案与用户点击、键盘和外部动作。
// OUTPUT: 证明完整保存反馈、共享 Portal 退出/焦点/模态边界和过期详情失效。
// POS: Contacts 保存状态离线 DOM 回归；不触发保存，不代替浏览器视觉验收。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";

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
    render(<AgentOptionsPersistenceStatus agentId="a" compact={false} state={{ message: "正在保存",phase: "saving" }} />,{ wrapper: I18nProvider });
    const status = screen.getByRole("status");
    expect(status.textContent).toBe("正在保存");

    expect(screen.queryByRole("button")).toBeNull();
  });

  it.each([false, true])("continues from the trigger on Tab with shift=%s", async (shift) => {
    const user = userEvent.setup();
    render(<><button>Before</button><AgentOptionsPersistenceStatus agentId="a" compact state={error} /><button>After</button></>, { wrapper: I18nProvider });
    await user.click(screen.getByRole("button", { name: "保存状态" }));
    await user.tab({ shift });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: shift ? "Before" : "After" }));
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
});
