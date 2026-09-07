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

function view(task: ScheduledTaskItem, onRunNow = vi.fn(), locale: "zh" | "en" = "zh") {
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
  it("keeps internal error text out of the card and retains it in explicit diagnostics", async () => {
    const error = "private-agent-id: /private/workspace\nprovider details";
    const task = { ...TASK, last_error: error, failure_streak: 1 };
    const { container, rerender } = render(view(task, vi.fn(), "en"));
    expect(screen.getByText("This run encountered a problem. View diagnostics for details.")).toBeTruthy();
    expect(container.textContent).not.toContain("private-agent-id");
    await userEvent.setup().click(screen.getByRole("button", { name: /查看.*详情/ }));
    expect(screen.getByRole("dialog").textContent).toContain(error);
    rerender(view(task, vi.fn(), "zh"));
    expect(screen.getByText("运行遇到问题，可查看诊断了解详情")).toBeTruthy();
    expect(screen.getByRole("dialog").textContent).toContain(error);
  });

  it("uses one known-error translation in both the card and the open diagnostic", async () => {
    const task = { ...TASK, last_error: "Permission request timeout", failure_streak: 1 };
    const { rerender } = render(view(task, vi.fn(), "en"));
    expect(screen.getByText("Timed out waiting for a permission response")).toBeTruthy();
    await userEvent.setup().click(screen.getByRole("button", { name: /查看.*详情/ }));
    expect(screen.getByRole("dialog").textContent).toContain("Technical details: Permission request timeout");
    rerender(view(task, vi.fn(), "zh"));
    expect(screen.getByText("等待权限响应超时")).toBeTruthy();
    expect(screen.getByRole("dialog").textContent).toContain("技术信息：Permission request timeout");
  });

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
