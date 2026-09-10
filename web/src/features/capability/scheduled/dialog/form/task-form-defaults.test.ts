// INPUT: 新建草稿、真实会话候选与用户选择。
// OUTPUT: 验证默认选择不猜测、不覆盖，以及名称可从指令生成。
// POS: 自动化表单默认行为回归。
import { act, renderHook } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { buildDefaultTaskDialogInitialState } from "./task-form-initializer";
import { buildScheduledTaskPayload, getTaskDialogValidationError } from "./task-form-submit";
import { useTaskForm } from "./use-task-form";

it("selects only a unique recipient and preserves explicit selections", () => {
  const initial = buildDefaultTaskDialogInitialState("agent-a");
  const { result } = renderHook(() => useTaskForm(initial.form, vi.fn()));
  const one = { value: "one", sessionKey: "one", label: "One" };
  const two = { value: "two", sessionKey: "two", label: "Two" };
  act(() => result.current.resolveDefaultSessions([], [one, two]));
  expect(result.current.draft.selectedReplySessionKey).toBe("");
  act(() => result.current.resolveDefaultSessions([], [one]));
  expect(result.current.draft.selectedReplySessionKey).toBe("one");
  act(() => result.current.resolveDefaultSessions([], [two]));
  expect(result.current.draft.selectedReplySessionKey).toBe("one");
  act(() => result.current.actions.setSelectedAgentId("agent-b"));
  expect(result.current.draft.selectedDeliveryAgentId).toBe("agent-b");
  expect(result.current.draft.selectedReplySessionKey).toBe("");
  act(() => result.current.actions.setReplyMode("none"));
  act(() => result.current.resolveDefaultSessions([], [one]));
  expect(result.current.draft.selectedReplySessionKey).toBe("");
});

it("derives a missing name while keeping permission inheritance and selected delivery", () => {
  const initial = buildDefaultTaskDialogInitialState("agent-a");
  const sessionKey = "agent:agent-a:ws:dm:test";
  const context = {
    ...initial,
    form: { ...initial.form, instruction: "  汇总今日\n 新闻  ", selectedReplySessionKey: sessionKey },
    defaultDeliveryRoomAgentId: "", defaultExecutionRoomAgentId: "",
    selectedSession: null,
    selectedReplySession: { value: sessionKey, sessionKey, label: "Daily" },
  };
  const t = (key: string) => key;
  expect(getTaskDialogValidationError(context, t)).toBeNull();
  const payload = buildScheduledTaskPayload(context, t);
  expect(payload.name).toBe("汇总今日 新闻");
  expect(payload.permission_mode).toBeUndefined();
  expect(payload.session_target?.kind).toBe("isolated");
  expect(payload.delivery?.session_key).toBe(sessionKey);
  context.form.taskName = "我的标题";
  expect(buildScheduledTaskPayload(context, t).name).toBe("我的标题");
});

it("changes a complete execution or delivery binding without changing the other destination", () => {
  const initial = buildDefaultTaskDialogInitialState("agent-a");
  const { result } = renderHook(() => useTaskForm(initial.form, vi.fn()));
  const receiver = {value: "agent:agent-b:ws:dm:reply", sessionKey: "agent:agent-b:ws:dm:reply", label: "Reply", group: "B", targetType: "agent" as const, agentId: "agent-b", roomId: ""};
  act(() => result.current.actions.selectDelivery(receiver));
  const execution = {value: "room:conversation", sessionKey: "room:conversation", label: "Room", group: "Research", targetType: "room" as const, agentId: "", roomId: "room-a"};
  act(() => result.current.actions.selectExecution(execution));
  expect(result.current.draft).toMatchObject({targetType: "room", selectedRoomId: "room-a", selectedSessionKey: execution.sessionKey, selectedAgentId: "", executionMode: "existing", permissionMode: "copy", selectedDeliveryAgentId: "agent-b", selectedReplySessionKey: receiver.sessionKey});
  act(() => result.current.actions.selectDelivery(null));
  expect(result.current.draft).toMatchObject({replyMode: "none", selectedReplySessionKey: "", selectedDeliveryPresenterAgentId: "", selectedSessionKey: execution.sessionKey});
  act(() => result.current.actions.selectExecution({...receiver, sessionKey: ""}));
  expect(result.current.draft).toMatchObject({targetType: "agent", selectedAgentId: "agent-b", selectedRoomId: "", selectedSessionKey: "", executionMode: "temporary", replyMode: "none"});
});
