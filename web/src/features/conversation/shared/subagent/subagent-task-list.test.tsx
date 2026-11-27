// INPUT: 子智能体任务目录的任务快照、读取失败、选择和刷新动作。
// OUTPUT: 共享行/状态与当前语言分组、唯一读取反馈、键盘选择和分钟时间显示。
// POS: 子任务列表 DOM 合同；纯状态排序与请求隔离继续由 model/hook 测试负责。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
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
  expect(time.title).toBe("");
  const initial = time.textContent;
  act(() => vi.advanceTimersByTime(60_000));
  expect(time.textContent).not.toBe(initial);
  expect(time.textContent).toBe(new Intl.RelativeTimeFormat("en", { numeric: "always", style: "narrow" }).format(-1, "minute"));
  expect(refresh).not.toHaveBeenCalled();
  expect(screen.getByRole("region", { name: "Active" })).toBeTruthy();
});
