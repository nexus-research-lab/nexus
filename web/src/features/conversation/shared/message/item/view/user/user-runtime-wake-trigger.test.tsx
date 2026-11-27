// INPUT: runtime 自唤醒行、主窗口定时任务行、跟进行与普通用户行。
// OUTPUT: 触发行渲染为无动作的分隔条，普通用户行保持气泡与动作。
// POS: 触发分隔条展示回归。
import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { Message, UserMessage } from "@/types/conversation/message/entity";

import { MessageUserSection } from "./message-user-section";
import { projectHostRunOutcome, projectRuntimeWakePresentation } from "./user-message-model";

const SUMMARY = 'Background command "Wait for stage1 confirmation" completed (exit code 0)';

function wakeMessage(metadata: Record<string, unknown>): UserMessage {
  return {
    message_id: "msg_user_round-wake",
    session_key: "agent:a:ws:dm:c",
    agent_id: "a",
    round_id: "round-wake",
    role: "user",
    content: SUMMARY,
    timestamp: Date.UTC(2026, 10, 19, 1, 1),
    is_synthetic: true,
    metadata,
  };
}

it("projects task status into trigger copy and drops summary for non-task wakes", () => {
  expect(projectRuntimeWakePresentation(wakeMessage({ input_kind: "runtime_wake", trigger_kind: "background_task", task_status: "failed" })))
    .toMatchObject({ labelKey: "message.runtime_wake.task_failed", summary: SUMMARY });
  expect(projectRuntimeWakePresentation(wakeMessage({ input_kind: "runtime_wake", trigger_kind: "runtime" })))
    .toMatchObject({ labelKey: "message.runtime_wake.resumed", summary: "" });
  expect(projectRuntimeWakePresentation({ ...wakeMessage({}), metadata: undefined })).toBeNull();
});

it("folds a host run outcome into the trigger divider", () => {
  const outcome = (value: string): Message => ({
    message_id: "host_outcome_round-wake",
    session_key: "agent:a:ws:dm:c",
    agent_id: "a",
    round_id: "round-wake",
    role: "system",
    content: value,
    timestamp: Date.UTC(2026, 10, 19, 1, 2),
    metadata: { subtype: "host_run_outcome", outcome: value },
  });
  expect(projectHostRunOutcome([outcome("quiet")])).toBe("message.host_run.quiet");
  expect(projectHostRunOutcome([outcome("failed")])).toBe("message.host_run.failed");
  expect(projectHostRunOutcome([outcome("unknown")])).toBeNull();
  expect(projectHostRunOutcome([])).toBeNull();

  const message = wakeMessage({ input_kind: "scheduled_task", task_name: "日报" });
  render(
    <I18nProvider>
      <MessageUserSection compact={false} hostOutcome="message.host_run.quiet" message={message} />
    </I18nProvider>,
  );
  const note = screen.getByRole("note");
  expect(note.querySelector("[data-host-run-outcome='true']")?.textContent).toBe("Nothing to report");
});
