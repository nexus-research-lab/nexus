// INPUT: 子智能体任务目录的任务快照、读取失败、选择和刷新动作。
// OUTPUT: 证明目录复用共享列表/排版/运行头像与行内提示，并保留唯一恢复动作。
// POS: 子任务列表反馈 DOM 合同；任务分组与请求隔离由 model/hook 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
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

describe("SubagentTaskList", () => {
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
