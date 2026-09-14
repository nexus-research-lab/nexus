// INPUT: 会话/来源切换、变化的 Todo 目录、真实 Portal 与嵌套选择器。
// OUTPUT: 证明详情身份隔离、非模态 Escape/外部点击/Tab 边界与可读任务状态。
// POS: Workspace Task Strip DOM 行为测试；Todo 归一化由 model 测试负责。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
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
    const visual = container.querySelector("[data-workspace-task-visual]");
    expect(trigger).not.toBeNull();
    expect(visual?.className).toContain("conversation-activity-chip");
    expect(visual?.className).toContain("conversation-activity-plain");
    expect(visual?.className).toContain("ui-type-metadata");
    expect(screen.getByText("Nexus").className).toContain("ui-type-caption");

    fireEvent.click(trigger!);

    const progress = screen.getByRole("dialog").querySelector("[data-workspace-task-progress-label]");
    expect(progress?.className).toContain("ui-type-metadata");
    expect(progress?.className).toContain("ui-type-tone-soft");
    expect(screen.getByText("统一活动条").className).toContain("ui-type-metadata");

    const detailActions = screen.getAllByRole("button", {
      name: /expand task details|展开任务详情/i,
    });
    expect(detailActions[0].className).toContain("radius-control-xs");
    fireEvent.click(detailActions[0]);
    expect(screen.getByText("已完成需求解析").className).toContain("ui-type-caption");

    const collapse = screen.getAllByRole("button", {
      name: /collapse tasks panel|收起任务面板/i,
    }).find((action) => action !== trigger);
    expect(collapse).toBeDefined();
    expect(collapse!.className).toContain("radius-control-xs");
    fireEvent.click(collapse!);
    expect(document.activeElement).toBe(trigger);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it.each(["zh", "en"] as const)("names the nonmodal panel and dismisses without stealing outside focus in %s", async (locale) => {
    const user = userEvent.setup();
    render(localizedPanel({ todos: TODOS, scopeKey: "session-a" }, locale));
    const trigger = screen.getByRole("button", { name: MESSAGES[locale]["tasks.expand_panel"] });
    trigger.focus();
    await user.keyboard("{Enter}");
    const dialog = screen.getByRole("dialog", { name: MESSAGES[locale]["tasks.label"] });
    expect(dialog.getAttribute("aria-modal")).toBe("false");
    expect(dialog.id).toBe(trigger.getAttribute("aria-controls"));
    expect(document.activeElement).toBe(dialog);
    const description = document.getElementById(trigger.getAttribute("aria-describedby")!);
    expect(description?.textContent).toContain("正在统一排版");
    expect(within(dialog).getAllByRole("listitem")).toHaveLength(2);
    fireEvent.keyDown(dialog, { key: "Escape", isComposing: true });
    expect(screen.getByRole("dialog")).toBe(dialog);
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(trigger);
    await user.click(trigger);
    const outside = screen.getByRole("button", { name: "Next action" });
    await user.click(outside);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(outside);
    expect(document.body.style.overflow).not.toBe("hidden");
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

  it("keeps status updates readable but never reuses a detail index for a reordered or replaced task", async () => {
    const user = userEvent.setup();
    const props = { todos: TODOS, scopeKey: "session-a", source: { agentId: "alpha", name: "Alpha", avatar: null } };
    const { rerender } = render(localizedPanel(props));
    await user.click(screen.getByRole("button", { name: MESSAGES.en["tasks.expand_panel"] }));
    const action = screen.getAllByRole("button", { name: MESSAGES.en["tasks.expand_detail"] })[0];
    await user.click(action);
    const detailId = action.getAttribute("aria-controls")!;
    expect(document.getElementById(detailId)?.textContent).toBe("已完成需求解析");
    const descriptions = action.getAttribute("aria-describedby")!.split(/\s+/).map((id) => document.getElementById(id)?.textContent);
    expect(descriptions).toContain("解析需求");
    rerender(localizedPanel({ ...props, todos: [{ ...TODOS[0], status: "in_progress", active_form: "新的解析进展" }, TODOS[1]] }));
    expect(document.getElementById(detailId)?.textContent).toBe("新的解析进展");
    expect(document.activeElement).toBe(action);
    rerender(localizedPanel({ ...props, todos: [...TODOS].reverse() }));
    expect(screen.queryByRole("button", { name: MESSAGES.en["tasks.collapse_detail"] })).toBeNull();
    expect(document.activeElement).toBe(action);
    await user.click(screen.getAllByRole("button", { name: MESSAGES.en["tasks.expand_detail"] })[1]);
    rerender(localizedPanel({ ...props, todos: [TODOS[1], { content: "另一任务", status: "pending", active_form: "另一详情" }] }));
    expect(screen.queryByText("另一详情")).toBeNull();
    expect(screen.getByRole("dialog")).toBeTruthy();
    expect(document.activeElement).toBe(screen.getByRole("dialog"));
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

  it("clears detail state on source changes or removed detail while keeping the task panel open", async () => {
    const user = userEvent.setup();
    const props = { todos: TODOS, scopeKey: "session-a", source: { agentId: "alpha", name: "Alpha", avatar: null } };
    const { rerender } = render(localizedPanel(props));
    await user.click(screen.getByRole("button", { name: MESSAGES.en["tasks.expand_panel"] }));
    await user.click(screen.getAllByRole("button", { name: MESSAGES.en["tasks.expand_detail"] })[0]);
    const switched = { ...props, source: { ...props.source, agentId: "beta", name: "Beta" } };
    rerender(localizedPanel(switched));
    expect(screen.getByRole("dialog")).toBeTruthy();
    expect(screen.queryByText("已完成需求解析")).toBeNull();
    await user.click(screen.getAllByRole("button", { name: MESSAGES.en["tasks.expand_detail"] })[0]);
    rerender(localizedPanel({ ...switched, todos: [{ ...TODOS[0], active_form: TODOS[0].content }, TODOS[1]] }));
    rerender(localizedPanel(switched));
    expect(screen.queryByText("已完成需求解析")).toBeNull();
    expect(screen.queryAllByRole("img")).toHaveLength(0);
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

  it("repositions from a changed summary anchor without resetting detail focus", async () => {
    const user = userEvent.setup();
    const props = { todos: TODOS, scopeKey: "session-a" };
    const { rerender } = render(localizedPanel(props));
    const trigger = screen.getByRole("button", { name: MESSAGES.en["tasks.expand_panel"] });
    const bounds = vi.spyOn(trigger, "getBoundingClientRect").mockReturnValue(new DOMRect(20, 600, 100, 44));
    await user.click(trigger);
    const dialog = screen.getByRole("dialog");
    expect(dialog.style.left).toBe("12px");
    const bottom = dialog.style.bottom;
    const detail = screen.getAllByRole("button", { name: MESSAGES.en["tasks.expand_detail"] })[0];
    await user.click(detail);
    bounds.mockReturnValue(new DOMRect(300, 500, 100, 44));
    rerender(localizedPanel({ ...props, todos: [TODOS[0], { ...TODOS[1], active_form: "更长的最新任务说明" }] }));
    expect(dialog.style.left).toBe("170px");
    expect(dialog.style.bottom).not.toBe(bottom);
    expect(document.activeElement).toBe(detail);
    expect(screen.getByText("已完成需求解析")).toBeTruthy();
  });

  it("keeps nested menu presses inside the task popover and dismisses one layer per Escape", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(localizedPanel({
      todos: TODOS,
      sourceControl: <UiSelectMenu ariaLabel="Task owner" onChange={onChange} options={[{ value: "alpha", label: "Alpha" }, { value: "beta", label: "Beta" }]} value="alpha" />,
    }));
    const trigger = screen.getByRole("button", { name: MESSAGES.en["tasks.expand_panel"] });
    await user.click(trigger);
    await user.click(screen.getByRole("button", { name: "Task owner" }));
    await user.click(screen.getByRole("option", { name: "Beta" }));
    expect(onChange).toHaveBeenCalledExactlyOnceWith("beta");
    expect(screen.getByRole("dialog")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Task owner" }));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(screen.getByRole("dialog")).toBeTruthy();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });
});
