// INPUT: 子智能体任务目录的保留快照、读取失败和刷新动作。
// OUTPUT: 证明错误使用共享行内提示，且只保留一个可执行恢复动作。
// POS: 子任务列表反馈 DOM 合同；任务分组与请求隔离由 model/hook 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { SubagentTaskList } from "./subagent-task-list";

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
});
