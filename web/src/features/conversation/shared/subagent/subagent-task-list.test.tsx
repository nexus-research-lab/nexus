// INPUT: 子智能体任务目录的任务快照、读取失败、选择和刷新动作。
// OUTPUT: 共享行/状态与当前语言分组、唯一读取反馈、键盘选择和分钟时间显示。
// POS: 子任务列表 DOM 合同；纯状态排序与请求隔离继续由 model/hook 测试负责。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { ComponentProps } from "react";
import type { SubagentTask } from "@/types/conversation/subagent-task";

import { SubagentTaskList } from "./subagent-task-list";

const ACTIVE_TASK: SubagentTask = {
  capabilities: {
    observe: true,
    resume: true,
    send_message: true,
    stop: true,
    transcript: true,
  },
  name: "资料整理",
  runtime_kind: "nxs",
  status: "running",
  summary: "汇总公共组件使用情况",
  task_id: "task-1",
  tool_use_id: "tool-1",
  updated_at: Date.now(),
};

function list(overrides: Partial<ComponentProps<typeof SubagentTaskList>> = {}, locale: "zh" | "en" = "en") {
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}><SubagentTaskList data={null} error={null} isLoading={false} onClose={vi.fn()} onRefresh={vi.fn()} onSelectTask={vi.fn()} tasks={[]} {...overrides} /></I18N_CONTEXT.Provider>;
}
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); });

describe("SubagentTaskList", () => {
  it("uses the same localized missing task name as the details header", () => {
    const task = { ...ACTIVE_TASK, name: " ", description: " ", agent_type: " " };
    const view = (locale: "en" | "zh") => <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}><SubagentTaskList data={null} error={null} isLoading={false} onClose={vi.fn()} onRefresh={vi.fn()} onSelectTask={vi.fn()} tasks={[task]} /></I18N_CONTEXT.Provider>;
    const { rerender } = render(view("zh"));
    expect(screen.getByRole("button", { name: new RegExp(MESSAGES.zh["agent.subagent_name_fallback"]) })).toBeTruthy();
    rerender(view("en"));
    expect(screen.getByRole("button", { name: new RegExp(MESSAGES.en["agent.subagent_name_fallback"]) })).toBeTruthy();
  });

  it("keeps one shared retry notice when the task list cannot refresh", async () => {
    const user = userEvent.setup();
    const onRefresh = vi.fn();
    render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <SubagentTaskList
          data={null}
          error="network"
          isLoading={false}
          onClose={vi.fn()}
          onRefresh={onRefresh}
          onSelectTask={vi.fn()}
          showTitle={false}
          tasks={[]}
        />
      </I18N_CONTEXT.Provider>,
    );

    const notice = screen.getByRole("status");
    expect(notice.getAttribute("data-inline-notice-tone")).toBe("danger");
    expect(notice.textContent).toContain("subagents.list_load_failed_title");
    expect(notice.textContent).toContain("subagents.list_load_failed_impact");
    expect(notice.textContent).not.toContain("subagents.list_load_failed_next_step");

    await user.click(screen.getByRole("button", { name: "subagents.retry" }));
    expect(onRefresh).toHaveBeenCalledOnce();
    expect(screen.queryByText("subagents.no_active")).toBeNull();
  });

  it("renders running tasks as shared dense rows with semantic text and avatar state", async () => {
    const user = userEvent.setup();
    const onSelectTask = vi.fn();
    render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <SubagentTaskList
          data={null}
          error={null}
          isLoading={false}
          onClose={vi.fn()}
          onRefresh={vi.fn()}
          onSelectTask={onSelectTask}
          showTitle={false}
          tasks={[ACTIVE_TASK]}
        />
      </I18N_CONTEXT.Provider>,
    );

    const row = screen.getByRole("button", { name: /资料整理/ });
    expect(row.tagName).toBe("DIV");
    expect(row.className).toContain("min-h-10");
    expect(row.className).toContain("radius-control-md");
    expect(screen.getByText("资料整理").className).toContain("ui-type-supporting");
    expect(screen.getByText("汇总公共组件使用情况").className).toContain("ui-type-metadata");
    expect(row.querySelector("[title='资料整理']")?.className)
      .toContain("status-running-soft-border");

    await user.click(row);
    expect(onSelectTask).toHaveBeenCalledWith("task-1");
  });
});


it("separates unknown and failed/stopped history while keeping exact keyboard selection", () => {
  const onSelectTask = vi.fn();
  const tasks = [
    { ...ACTIVE_TASK, task_id: "private-unknown", name: "New state", status: "internal_future_state" },
    { ...ACTIVE_TASK, task_id: "private-failed", name: "Failed task", status: "failed" },
    { ...ACTIVE_TASK, task_id: "private-stopped", name: "Stopped task", status: "cancelled" },
    { ...ACTIVE_TASK, task_id: "private-queued", name: "Queued task", status: "queued" },
  ];
  const { container, rerender } = render(list({ tasks, onSelectTask }));
  const unknown = screen.getByRole("region", { name: "Status unavailable · 1" });
  const row = within(unknown).getByRole("button", { name: /New state/ });
  expect(row.querySelector('[title="New state"]')?.className).not.toContain("status-running-soft-border");
  expect(screen.queryByText(MESSAGES.en["subagents.no_active"])).toBeNull();
  const history = screen.getByRole("region", { name: "History · 2" });
  expect(within(history).getByText("Failed")).toBeTruthy();
  expect(within(history).getByText("Stopped")).toBeTruthy();
  expect(screen.getByText("Queued")).toBeTruthy();
  expect(container.textContent).not.toMatch(/private-|internal_future_state/);
  row.focus();
  fireEvent.keyDown(row, { key: "Enter" });
  expect(onSelectTask).toHaveBeenCalledExactlyOnceWith("private-unknown");
  rerender(list({ tasks, onSelectTask }, "zh"));
  expect(screen.getByRole("region", { name: "历史任务 · 2" })).toBeTruthy();
  expect(screen.getByText("排队中")).toBeTruthy();
});

it("exposes one initial loading state and keeps cached rows readable during retry", () => {
  const { container, rerender } = render(list({ isLoading: true }));
  expect(screen.getByRole("status").textContent).toBe(MESSAGES.en["subagents.loading"]);
  expect(container.querySelector('[aria-busy="true"]')).toBeTruthy();
  expect(screen.queryByText(MESSAGES.en["subagents.no_active"])).toBeNull();
  rerender(list({ tasks: [ACTIVE_TASK], error: "failed" }));
  expect(screen.getByRole("button", { name: /资料整理/ })).toBeTruthy();
  expect(screen.getAllByRole("status")).toHaveLength(1);
  rerender(list({ tasks: [ACTIVE_TASK], isLoading: true }));
  expect(screen.getByRole("button", { name: /资料整理/ })).toBeTruthy();
  expect(screen.queryByText(MESSAGES.en["subagents.loading"])).toBeNull();
});

it("omits invalid times and provides an exact date plus updated minute labels for valid observations", () => {
  vi.useFakeTimers();
  const now = Date.UTC(2026, 8, 7, 12);
  vi.setSystemTime(now);
  vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
  const refresh = vi.fn();
  const { container, rerender } = render(list({ tasks: [{ ...ACTIVE_TASK, updated_at: Infinity }], onRefresh: refresh }));
  expect(container.querySelector("time")).toBeNull();
  expect(vi.getTimerCount()).toBe(0);
  rerender(list({ tasks: [{ ...ACTIVE_TASK, updated_at: now }], onRefresh: refresh }));
  const time = container.querySelector("time")!;
  expect(time.dateTime).toBe(new Date(now).toISOString());
  expect(time.title).toBe(new Intl.DateTimeFormat("en", { dateStyle: "medium", timeStyle: "short" }).format(now));
  const initial = time.textContent;
  act(() => vi.advanceTimersByTime(60_000));
  expect(time.textContent).not.toBe(initial);
  expect(time.textContent).toBe(new Intl.RelativeTimeFormat("en", { numeric: "always", style: "narrow" }).format(-1, "minute"));
  expect(refresh).not.toHaveBeenCalled();
  expect(screen.getByRole("region", { name: "Active" })).toBeTruthy();
});


it.each(["en", "zh"] as const)("keeps a missing requested task actionable with one busy feedback surface in %s", (locale) => {
  const refresh = vi.fn();
  const data = { runtime_kind: ACTIVE_TASK.runtime_kind, capabilities: ACTIVE_TASK.capabilities, items: [] };
  const { rerender } = render(list({ data, requestedTaskUnavailable: true, isLoading: true, onRefresh: refresh }, locale));
  expect(screen.getAllByRole("status")).toHaveLength(1);
  expect(screen.getByText(MESSAGES[locale]["subagents.requested_task_missing_title"])).toBeTruthy();
  expect(screen.queryByText(MESSAGES[locale]["subagents.no_active"])).toBeNull();
  const retry = screen.getByRole("button", { name: MESSAGES[locale]["subagents.retry"] });
  expect(retry.getAttribute("aria-busy")).toBe("true");
  expect(retry.hasAttribute("disabled")).toBe(true);
  fireEvent.click(retry);
  expect(refresh).not.toHaveBeenCalled();
  rerender(list({ data, requestedTaskUnavailable: true, error: "offline", onRefresh: refresh }, locale));
  expect(screen.getAllByRole("status")).toHaveLength(1);
  expect(screen.getByText(MESSAGES[locale]["subagents.list_load_failed_title"])).toBeTruthy();
  expect(screen.queryByText(MESSAGES[locale]["subagents.requested_task_missing_title"])).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: MESSAGES[locale]["subagents.retry"] }));
  expect(refresh).toHaveBeenCalledOnce();
});
