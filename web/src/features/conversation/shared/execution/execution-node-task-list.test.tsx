// INPUT: 真实局部任务投影、历史异常载荷、当前语言及精确 Agent round 切换。
// OUTPUT: 验证归一化清单、完整步骤访问、独立展开状态和可读任务正文。
// POS: 节点 Task 只读 DOM 回归；关联资格仍由 node-task-model 合同验证。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";

import { ExecutionNodeTaskList } from "./execution-node-task-list";

const RUN: ConversationTaskRun = {
  agentId: "agent-1",
  agentRoundId: "agent-round-1",
  latestTaskEventIndex: 8,
  todos: Array.from({ length: 8 }, (_, index) => ({
    content: `步骤 ${index + 1}`,
    status: index < 4 ? "completed" : index === 4 ? "in_progress" : "pending",
    ...(index === 4 ? { active_form: "  正在核对完整的协议与本轮交付结果  " } : {}),
  })),
};

function view(run = RUN, locale: I18nContextValue["locale"] = "zh", copies = 1) {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {})
    .reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return (
    <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>
      {Array.from({ length: copies }, (_, index) => <ExecutionNodeTaskList key={index} run={run} />)}
    </I18N_CONTEXT.Provider>
  );
}

describe("ExecutionNodeTaskList", () => {
  it("focuses the current step and offers keyboard access to every step without resetting on language changes", async () => {
    const user = userEvent.setup();
    const { rerender } = render(view());
    const list = screen.getByRole("list");
    expect(within(list).getAllByRole("listitem")).toHaveLength(5);
    expect(list.firstElementChild?.getAttribute("value")).toBe("3");
    expect(screen.queryByText("步骤 1")).toBeNull();
    expect(screen.queryByText("步骤 8")).toBeNull();
    expect(screen.getByText("4/8")).toBeTruthy();
    const active = screen.getByText("正在核对完整的协议与本轮交付结果");
    expect(active.getAttribute("title")).toBeNull();
    expect(active.className).not.toContain("truncate");
    expect(active.closest("li")?.className).toContain("ui-type-supporting");

    const toggle = screen.getByRole("button", { name: "另有 3 步" });
    expect(toggle.getAttribute("aria-controls")).toBe(list.id);
    await user.tab();
    expect(document.activeElement).toBe(toggle);
    await user.keyboard("{Enter}");
    expect(toggle.getAttribute("aria-expanded")).toBe("true");
    expect(within(list).getAllByRole("listitem")).toHaveLength(8);
    expect(screen.getByText("步骤 1")).toBeTruthy();
    expect(screen.getByText("步骤 8")).toBeTruthy();

    rerender(view(RUN, "en"));
    expect(screen.getByRole("button", { name: "Collapse steps" })).toBe(toggle);
    expect(within(list).getAllByRole("listitem")).toHaveLength(8);
    await user.keyboard("{Enter}");
    expect(within(list).getAllByRole("listitem")).toHaveLength(5);
    expect(screen.getByRole("button", { name: "3 more steps" }).getAttribute("aria-expanded")).toBe("false");
  });

  it("renders the same normalized legacy tasks used for counts and ignores malformed entries", () => {
    const todos = [
      null,
      { content: "", status: "completed" },
      { content: "忽略未知状态", status: "future" },
      { task: "  旧格式任务  ", status: "in_progress", activeForm: " 正在处理旧格式 " },
      { content: "  已完成  ", status: "completed" },
      { content: "等待处理", status: "pending", active_form: 42 },
    ] as unknown as ConversationTaskRun["todos"];
    render(view({ ...RUN, todos }));
    expect(screen.getByText("1/3")).toBeTruthy();
    expect(screen.getAllByRole("listitem")).toHaveLength(3);
    expect(screen.getByText("正在处理旧格式").textContent).toBe("正在处理旧格式");
    expect(screen.queryByText("忽略未知状态")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("keeps simultaneous task lists independently named and expanded", async () => {
    const user = userEvent.setup();
    render(view(RUN, "zh", 2));
    const regions = screen.getAllByRole("region", { name: "局部步骤" });
    const lists = screen.getAllByRole("list");
    expect(lists[0].id).not.toBe(lists[1].id);
    expect(regions[0].getAttribute("aria-labelledby")).not.toBe(regions[1].getAttribute("aria-labelledby"));
    await user.click(within(regions[0]).getByRole("button"));
    expect(within(lists[0]).getAllByRole("listitem")).toHaveLength(8);
    expect(within(lists[1]).getAllByRole("listitem")).toHaveLength(5);
  });

  it.each(["agentId", "agentRoundId"] as const)("resets expanded details after the exact %s changes, including returning to the original run", async (field) => {
    const user = userEvent.setup();
    const { rerender } = render(view());
    await user.click(screen.getByRole("button"));
    expect(screen.getAllByRole("listitem")).toHaveLength(8);
    rerender(view({ ...RUN, [field]: "another-identity" }));
    expect(screen.getAllByRole("listitem")).toHaveLength(5);
    await user.click(screen.getByRole("button"));
    rerender(view());
    expect(screen.getAllByRole("listitem")).toHaveLength(5);
  });

  it("keeps the last five completed steps visible in a finished run", () => {
    render(view({ ...RUN, todos: RUN.todos.map((todo) => ({ ...todo, status: "completed" })) }));
    expect(screen.getByText("8/8")).toBeTruthy();
    expect(screen.getByRole("list").firstElementChild?.getAttribute("value")).toBe("4");
    expect(screen.getByText("步骤 8").className).toContain("line-through");
  });

  it("renders no empty section for a run without displayable tasks", () => {
    const { container } = render(view({ ...RUN, todos: [] }));
    expect(container.childElementCount).toBe(0);
  });
});
