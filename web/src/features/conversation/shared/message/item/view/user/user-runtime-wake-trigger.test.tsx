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

it("projects a main-window scheduled task as a trigger showing only the task name", () => {
  const message = {
    ...wakeMessage({ input_kind: "scheduled_task", task_name: " 日报 ", job_id: "job-1", run_id: "run-1" }),
    content: "<scheduled-task name=\"日报\">\n汇总今天的提交\n</scheduled-task>",
  };
  expect(projectRuntimeWakePresentation(message))
    .toMatchObject({ source: "scheduled_task", labelKey: "message.scheduled_task.triggered", summary: "日报" });
  render(
    <I18nProvider>
      <MessageUserSection compact={false} message={message} onEditUserMessage={() => undefined} />
    </I18nProvider>,
  );
  const note = screen.getByRole("note");
  expect(note.textContent).toContain("日报");
  expect(note.textContent).not.toContain("汇总今天的提交");
  expect(note.closest("[data-message-kind='scheduled-task']")).not.toBeNull();
  expect(screen.queryByRole("button")).toBeNull();
});

it("projects a due follow-up as a trigger showing only the note", () => {
  const message = {
    ...wakeMessage({ input_kind: "follow_up", follow_up_id: "inbox_1", follow_up_note: "确认 CI 结果" }),
    content: "<follow-up id=\"inbox_1\">\n内部包装\n</follow-up>",
  };
  expect(projectRuntimeWakePresentation(message))
    .toMatchObject({ source: "follow_up", labelKey: "message.follow_up.triggered", summary: "确认 CI 结果" });
  render(
    <I18nProvider>
      <MessageUserSection compact={false} message={message} onEditUserMessage={() => undefined} />
    </I18nProvider>,
  );
  const note = screen.getByRole("note");
  expect(note.textContent).toContain("确认 CI 结果");
  expect(note.textContent).not.toContain("内部包装");
  expect(note.closest("[data-message-kind='follow-up']")).not.toBeNull();
});

it("renders a runtime wake as an action-free trigger row instead of a user bubble", () => {
  render(
    <I18nProvider>
      <MessageUserSection
        compact={false}
        message={wakeMessage({ input_kind: "runtime_wake", trigger_kind: "background_task", task_status: "completed" })}
        onEditUserMessage={() => undefined}
      />
    </I18nProvider>,
  );
  const note = screen.getByRole("note");
  expect(note.textContent).toContain(SUMMARY);
  expect(note.closest("[data-message-kind='runtime-wake']")?.getAttribute("data-conversation-round-user-anchor")).toBe("true");
  expect(document.querySelector(".nexus-chat-user-frame")).toBeNull();
  expect(screen.queryByRole("button")).toBeNull();
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
