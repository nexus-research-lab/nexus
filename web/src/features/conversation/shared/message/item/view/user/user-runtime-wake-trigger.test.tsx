// INPUT: runtime 自唤醒用户行与普通用户行。
// OUTPUT: 自唤醒渲染为无动作的触发分隔条，普通用户行保持气泡与动作。
// POS: runtime 自唤醒展示回归。
import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { UserMessage } from "@/types/conversation/message/entity";

import { MessageUserSection } from "./message-user-section";
import { projectRuntimeWakePresentation } from "./user-message-model";

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
