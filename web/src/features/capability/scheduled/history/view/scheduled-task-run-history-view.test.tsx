// INPUT: 失败运行、所属任务与诊断/重跑动作。
// OUTPUT: 证明历史行复用 Typography、Panel、Button 与语义圆角且动作仍正确派发。
// POS: Scheduled 运行历史 DOM 合同；可执行动作集合归 history model。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { ScheduledTaskRunHistoryItem } from "./scheduled-task-run-history-item";

const TASK: ScheduledTaskItem = {
  agent_id: "agent-1",
  configuration_version: 1,
  delivery: { mode: "none" },
  enabled: true,
  expires_at: null,
  failure_streak: 1,
  instruction: "整理今日进展",
  job_id: "task-1",
  last_error: "network timeout",
  last_run_at: 1_735_689_600_000,
  name: "每日工作简报",
  next_run_at: null,
  running: false,
  running_started_at: null,
  schedule: { interval_seconds: 86_400, kind: "every" },
  session_target: { kind: "main" },
  source: { kind: "user_page" },
};

const RUN: ScheduledTaskRunItem = {
  attempts: 1,
  delivered_at: null,
  delivery_dead_letter_at: null,
  delivery_next_attempt_at: null,
  delivery_status: "not_required",
  error_message: "network timeout",
  finished_at: 1_735_689_660_000,
  job_id: TASK.job_id,
  run_id: "run-1",
  scheduled_for: 1_735_689_600_000,
  started_at: 1_735_689_601_000,
  status: "failed",
};

describe("ScheduledTaskRunHistoryItem", () => {
  it("uses shared history chrome and preserves diagnostic and retry actions", async () => {
    const onCopyDiagnostic = vi.fn();
    const onRetry = vi.fn();
    const user = userEvent.setup();
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <ScheduledTaskRunHistoryItem
          defaultOpen
          isCopied={false}
          isRecovering={false}
          isRecoveryUnconfirmed={false}
          isRetryDeliveryUnconfirmed={false}
          isRetryUnconfirmed={false}
          isRetrying={false}
          isRetryingDelivery={false}
          onCopyDiagnostic={onCopyDiagnostic}
          onRecover={vi.fn()}
          onRetry={onRetry}
          onRetryDelivery={vi.fn()}
          run={RUN}
          task={TASK}
        />
      </I18N_CONTEXT.Provider>,
    );

    expect(container.querySelector("summary.radius-control-md")).toBeTruthy();
    expect(screen.getByText("诊断详情").className).toContain("ui-type-caption");
    expect(container.querySelectorAll("section.surface-radius-sm").length).toBeGreaterThan(0);

    const copyButton = screen.getByRole("button", { name: "复制诊断" });
    expect(copyButton.className).toContain("radius-control-xs");
    await user.click(copyButton);
    expect(onCopyDiagnostic).toHaveBeenCalledWith(RUN);

    const retryButton = screen.getByRole("button", { name: "重新运行" });
    expect(retryButton.className).toContain("focus-visible:ring-2");
    await user.click(retryButton);
    expect(onRetry).toHaveBeenCalledWith(RUN);
  });
});
