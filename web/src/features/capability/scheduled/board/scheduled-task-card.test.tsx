// INPUT: 普通、执行中与待权限处理的定时任务，以及行级动作。
// OUTPUT: 证明任务卡复用 CatalogCard、Panel、Button、Badge、Typography 与 Spinner。
// POS: 定时任务卡 DOM 合同；任务分列和文案仍由 board model 决定。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
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

function view(task: ScheduledTaskItem, onRunNow = vi.fn(), locale: "zh" | "en" = "zh", onEdit = vi.fn()) {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return (
    <I18N_CONTEXT.Provider
      value={{ locale, setLocale: vi.fn(), t }}
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
        onEdit={onEdit}
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
  it("opens the task editor from its title", async () => {
    const onEdit = vi.fn();
    render(view(TASK, vi.fn(), "zh", onEdit));
    await userEvent.setup().click(screen.getByRole("button", { name: TASK.name }));
    expect(onEdit).toHaveBeenCalledWith(TASK);
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

    expect(container.querySelectorAll("section.surface-radius-sm").length).toBeGreaterThan(0);
  });
});

it.each(["zh", "en"] as const)("does not expose the internal execution Agent when source names are missing in %s", (locale) => {
  const task = { ...TASK, agent_id: "private-executor-identity" };
  const result = render(view(task, vi.fn(), locale));
  expect(screen.getByText(MESSAGES[locale]["capability.scheduled_context_agent"])).toBeTruthy();
  expect(result.container.textContent).not.toContain(task.agent_id);
  result.rerender(view({ ...task, source: { kind: "user_page", context_type: "agent", context_id: task.agent_id, context_label: "Nova" } }, vi.fn(), locale));
  expect(screen.getByText("Nova")).toBeTruthy();
  expect(result.container.textContent).not.toContain(task.agent_id);
});

it("closes a resolved attention surface when a new attempt starts", async () => {
  const failed = { ...TASK, last_error: "old attempt private error", failure_streak: 1 };
  const { rerender } = render(view(failed));
  await userEvent.setup().click(screen.getByRole("button", { name: /查看.*详情/ }));
  expect(screen.getByRole("dialog").textContent).toContain("old attempt private error");
  rerender(view({ ...failed, running: true, running_started_at: Date.now() }));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(screen.queryByText(/old attempt private error/)).toBeNull();
});

it("localizes an open deletion review without exposing stale permission actions", async () => {
  const task = { ...TASK, name: "Review task", instruction: "Work", deletion_state: "review_required" };
  const { rerender } = render(view(task, vi.fn(), "en"));
  await userEvent.setup().click(screen.getByRole("button", { name: /View details:/ }));
  const dialog = screen.getByRole("dialog");
  expect(dialog.textContent).not.toMatch(/[\u4e00-\u9fff]/);
  expect(screen.getByRole("button", { name: "Confirm stopped and continue deletion" })).toBeTruthy();
  rerender(view(task, vi.fn(), "zh"));
  expect(screen.getByRole("button", { name: "确认已停止，继续删除" })).toBeTruthy();
  expect(screen.getByRole("dialog")).toBe(dialog);
});
