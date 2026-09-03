// INPUT: 普通、执行中与待权限处理的定时任务，以及行级动作。
// OUTPUT: 证明任务卡复用 CatalogCard、Panel、Button、Badge、Typography 与 Spinner。
// POS: 定时任务卡 DOM 合同；任务分列和文案仍由 board model 决定。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { ScheduledTaskCard } from "./scheduled-task-card";

const TASK: ScheduledTaskItem = {
  agent_id: "agent-1",
  configuration_version: 1,
  delivery: { mode: "none" },
  enabled: true,
  expires_at: null,
  failure_streak: 0,
  instruction: "整理今日进展和待处理事项",
  job_id: "task-1",
  last_error: null,
  last_run_at: null,
  name: "每日工作简报",
  next_run_at: Date.now() + 60_000,
  running: false,
  running_started_at: null,
  schedule: { interval_seconds: 86_400, kind: "every" },
  session_target: { kind: "main" },
  source: { kind: "user_page" },
};

function view(task: ScheduledTaskItem, onRunNow = vi.fn()) {
  return (
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <ScheduledTaskCard
        isDeleteUnconfirmed={false}
        isDeleting={false}
        isDeletionReviewPending={false}
        isMutationBlocked={false}
        isPermissionPending={false}
        isPermissionUnconfirmed={false}
        isRunUnconfirmed={false}
        isRunning={false}
        isToggleUnconfirmed={false}
        isToggling={false}
        onConfirmDeletionStopped={vi.fn()}
        onDelete={vi.fn()}
        onEdit={vi.fn()}
        onOpenConnector={vi.fn()}
        onOpenHistory={vi.fn()}
        onPermissionDecision={vi.fn()}
        onPermissionResume={vi.fn()}
        onRefresh={vi.fn()}
        onRunNow={onRunNow}
        onToggleEnabled={vi.fn()}
        task={task}
      />
    </I18N_CONTEXT.Provider>
  );
}

describe("ScheduledTaskCard", () => {
  it("uses the shared catalog card and typography while preserving actions", async () => {
    const onRunNow = vi.fn();
    const user = userEvent.setup();
    const { container } = render(view(TASK, onRunNow));

    expect(container.querySelector("article.surface-radius-md")).toBeTruthy();
    expect(screen.getByText(TASK.name).className).toContain("ui-type-section-title");
    expect(screen.getByText(TASK.instruction).className).toContain("ui-type-metadata");

    await user.click(screen.getByRole("button", { name: "立即运行" }));
    expect(onRunNow).toHaveBeenCalledWith(TASK);
  });

  it("uses the shared reduced-motion spinner for a running task", () => {
    const runningTask = { ...TASK, running: true, running_started_at: Date.now() };
    const { container } = render(view(runningTask));

    const spinner = container.querySelector("svg.animate-spin");
    expect(spinner).toBeTruthy();
    expect(spinner?.getAttribute("class")).toContain("motion-reduce:animate-none");
  });

  it("opens permission attention with shared panel and badge owners", async () => {
    const user = userEvent.setup();
    const permissionTask: ScheduledTaskItem = {
      ...TASK,
      pending_permission_request: {
        capability: {
          effect: "read",
          resource_scope: "https://example.com/report",
          tool_name: "web.search",
        },
        created_at: "2025-01-01T00:00:00Z",
        description: "读取报告以完成定时任务",
        job_id: TASK.job_id,
        kind: "tool",
        policy_revision: 2,
        request_id: "permission-1",
        resume_safe: true,
        run_id: "run-1",
        status: "pending",
        updated_at: "2025-01-01T00:00:00Z",
      },
      pending_permission_request_id: "permission-1",
      permission_state: "awaiting_approval",
    };
    const { container } = render(view(permissionTask));

    await user.click(screen.getByRole("button", { name: /查看.*详情/ }));

    expect(screen.getByRole("dialog")).toBeTruthy();
    expect(screen.getByText("等待处理").className).toContain("rounded-full");
    expect(container.querySelectorAll("section.surface-radius-sm").length).toBeGreaterThan(0);
  });
});
