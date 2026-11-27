// INPUT: 会话/来源切换、变化的 Todo 目录、真实 Portal 与嵌套选择器。
// OUTPUT: 证明详情身份隔离、非模态 Escape/外部点击/Tab 边界与可读任务状态。
// POS: Workspace Task Strip DOM 行为测试；Todo 归一化由 model 测试负责。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import type { TodoItem } from "@/types/conversation/todo";

import { WorkspaceTaskPanel } from "./workspace-task-strip";

const TODOS: TodoItem[] = [
  {
    active_form: "已完成需求解析",
    content: "解析需求",
    status: "completed",
  },
  {
    active_form: "正在统一排版",
    content: "统一活动条",
    status: "in_progress",
  },
];

function localizedPanel(props: Parameters<typeof WorkspaceTaskPanel>[0], locale: Locale = "en") {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {})
    .reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return (
    <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>
      <WorkspaceTaskPanel {...props} />
      <button type="button">Next action</button>
    </I18N_CONTEXT.Provider>
  );
}

afterEach(() => vi.restoreAllMocks());

describe("WorkspaceTaskPanel", () => {
  it("keeps compact typography and icon actions under shared UI owners", () => {
    const { container } = render(
      <I18nProvider>
        <WorkspaceTaskPanel
          source={{ agentId: "agent-nexus", avatar: null, name: "Nexus" }}
          todos={TODOS}
        />
      </I18nProvider>,
    );

    const trigger = container.querySelector<HTMLButtonElement>("[data-workspace-task-trigger]");
    expect(trigger).not.toBeNull();

    fireEvent.click(trigger!);

    const detailActions = screen.getAllByRole("button", {
      name: /expand task details|展开任务详情/i,
    });

    fireEvent.click(detailActions[0]);

    const collapse = screen.getAllByRole("button", {
      name: /collapse tasks panel|收起任务面板/i,
    }).find((action) => action !== trigger);
    expect(collapse).toBeDefined();

    fireEvent.click(collapse!);
    expect(document.activeElement).toBe(trigger);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("clears open details and the popup across session changes and empty-task recovery", async () => {
    const user = userEvent.setup();
    const props = { todos: TODOS, scopeKey: "session-a" };
    const { rerender } = render(localizedPanel(props));
    await user.click(screen.getByRole("button", { name: MESSAGES.en["tasks.expand_panel"] }));
    await user.click(screen.getAllByRole("button", { name: MESSAGES.en["tasks.expand_detail"] })[0]);
    expect(screen.getByText("已完成需求解析")).toBeTruthy();
    rerender(localizedPanel({ ...props, scopeKey: "session-b" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    rerender(localizedPanel(props));
    expect(screen.queryByRole("dialog")).toBeNull();
    await user.click(screen.getByRole("button", { name: MESSAGES.en["tasks.expand_panel"] }));
    expect(screen.queryByText("已完成需求解析")).toBeNull();
    rerender(localizedPanel({ ...props, todos: [] }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByRole("button", { name: MESSAGES.en["tasks.expand_panel"] })).toBeNull();
    rerender(localizedPanel(props));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("does not confuse identically named steps with different details", async () => {
    const user = userEvent.setup();
    const todos: TodoItem[] = [
      { content: "重复步骤", active_form: "第一项详情", status: "pending" },
      { content: "重复步骤", active_form: "第二项详情", status: "pending" },
    ];
    const { rerender } = render(localizedPanel({ todos }));
    await user.click(screen.getByRole("button", { name: MESSAGES.en["tasks.expand_panel"] }));
    await user.click(screen.getAllByRole("button", { name: MESSAGES.en["tasks.expand_detail"] })[0]);
    expect(screen.getByText("第一项详情")).toBeTruthy();
    rerender(localizedPanel({ todos: [...todos].reverse() }));
    expect(screen.queryByText("第一项详情")).toBeNull();
    expect(screen.queryByText("第二项详情")).toBeNull();
    expect(screen.queryByRole("button", { name: MESSAGES.en["tasks.collapse_detail"] })).toBeNull();
  });

  it("continues Tab from the trigger rather than stranding focus in the Portal", async () => {
    vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([new DOMRect(0, 0, 100, 32)] as unknown as DOMRectList);
    const user = userEvent.setup();
    render(localizedPanel({ todos: TODOS }));
    const trigger = screen.getByRole("button", { name: MESSAGES.en["tasks.expand_panel"] });
    await user.click(trigger);
    await user.tab();
    const first = within(screen.getByRole("dialog")).getByRole("button", { name: MESSAGES.en["tasks.collapse_panel"] });
    expect(document.activeElement).toBe(first);
    await user.tab({ shift: true });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(trigger);
    await user.click(trigger);
    const details = screen.getAllByRole("button", { name: MESSAGES.en["tasks.expand_detail"] });
    act(() => details.at(-1)!.focus());
    await user.tab();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Next action" }));
  });
});
