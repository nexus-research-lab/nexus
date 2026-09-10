// INPUT: Scheduled 高级表单草稿、资源状态与字段变更命令。
// OUTPUT: 证明字段实例隔离、资源缺项/恢复显示、选择组具名及原样输入/精确选择命令。
// POS: Scheduled 基础表单 DOM 合同；不覆盖资源加载与提交事务。

import { fireEvent, render, screen, within } from "@testing-library/react";
import { TaskDestinationPicker } from "./task-destination-picker";
import { createRef } from "react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { createDefaultTaskSchedule } from "../schedule/task-schedule-model";
import type { TranslationKey } from "@/shared/i18n/messages";

import type { TaskFormDraft } from "../scheduled-task-dialog-types";
import { buildTaskDestinationPickerModel } from "./task-destination-picker-model";
import { TaskBasicsPanel } from "./task-basics-panel";
import type { TaskBasicsActions, TaskBasicsData } from "./task-basics-model";
import { buildTaskConfirmationSummary } from "./task-basics-model";

const READY_RESOURCE = {
  error: null,
  loading: false,
  retry: vi.fn(),
};

const DATA: TaskBasicsData = {
  destinations: [],
  destinationStatus: READY_RESOURCE,
  inheritedPermissionMode: "auto",
  agentOptions: [],
  agents: READY_RESOURCE,
  defaultDeliveryRoomAgentId: "",
  defaultExecutionRoomAgentId: "",
  deliveryRoomAgentOptions: [],
  deliveryRoomOptions: [],
  deliverySessionOptions: [],
  deliverySessions: READY_RESOURCE,
  executionRoomAgentOptions: [],
  roomOptions: [],
  rooms: READY_RESOURCE,
  sessionOptions: [],
  sessions: READY_RESOURCE,
};

const FORM: TaskFormDraft = {
  dedicatedSessionKey: "",
  deliveryTargetType: "agent",
  enabled: true,
  executionKind: "agent",
  executionMode: "temporary",
  expiresAt: "",
  instruction: "整理今日进展",
  permissionMode: "acceptEdits",
  replyMode: "none",
  selectedAgentId: "agent-1",
  selectedDeliveryAgentId: "",
  selectedDeliveryPresenterAgentId: "",
  selectedDeliveryRoomId: "",
  selectedReplySessionKey: "",
  selectedRoomId: "",
  selectedSessionKey: "",
  targetType: "agent",
  taskName: "每日简报",
};

function createActions(): TaskBasicsActions {
  return {
    selectExecution: vi.fn(),
    selectDelivery: vi.fn(),
    setDedicatedSessionKey: vi.fn(),
    setDeliveryTargetType: vi.fn(),
    setExecutionMode: vi.fn(),
    setExpiresAt: vi.fn(),
    setPermissionMode: vi.fn(),
    setReplyMode: vi.fn(),
    setSelectedAgentId: vi.fn(),
    setSelectedDeliveryAgentId: vi.fn(),
    setSelectedDeliveryPresenterAgentId: vi.fn(),
    setSelectedDeliveryRoomId: vi.fn(),
    setSelectedReplySessionKey: vi.fn(),
    setSelectedRoomId: vi.fn(),
    setSelectedSessionKey: vi.fn(),
    setTargetType: vi.fn(),
    setTaskName: vi.fn(),
  };
}

describe("TaskBasicsPanel", () => {
  it("lists all destinations and selects an exact complete binding", async () => {
    const user = userEvent.setup();
    const actions = createActions();
    const destination = {value: "session-b", sessionKey: "session-b", label: "Chat B", group: "Nova", targetType: "agent" as const, agentId: "agent-b", roomId: ""};
    render(<I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: (key) => key}}>
      <TaskBasicsPanel actions={actions} data={{...DATA, destinations: [destination]}} form={FORM} nameRef={createRef<HTMLInputElement>()} needsSessionRebind={false} />
    </I18N_CONTEXT.Provider>);
    await user.click(screen.getByText("capability.scheduled_run_in"));
    const searches = screen.getAllByRole("searchbox");
    await user.type(searches[0], "Chat B");
    await user.click(within(screen.getByRole("group", {name: "capability.scheduled_run_in"})).getByRole("button", {name: "Chat B"}));
    expect(actions.selectExecution).toHaveBeenCalledExactlyOnceWith(destination);
    expect(actions.selectDelivery).not.toHaveBeenCalled();
    expect(actions.setTargetType).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog", {name: "capability.scheduled_run_in"})).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", {name: "capability.scheduled_run_in"}));
    await user.click(screen.getByRole("button", {name: "capability.scheduled_run_in"}));
    const picker = screen.getByRole("dialog", {name: "capability.scheduled_run_in"});
    expect(picker.parentElement).toBe(document.body);
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog", {name: "capability.scheduled_run_in"})).toBeNull();
  });

  it("browses and searches objects without changing the task", async () => {
    const user = userEvent.setup();
    const actions = createActions();
    const room = {value: "room-session", sessionKey: "room-session", label: "Discussion", group: "Research", targetType: "room" as const, agentId: "", roomId: "room"};
    render(<I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: (key) => key}}>
      <TaskBasicsPanel actions={actions} data={{...DATA, agentOptions: [{value: "agent-1", label: "Nova"}], roomOptions: [{value: "room", label: "Research"}], destinations: [room]}} form={FORM} nameRef={createRef<HTMLInputElement>()} needsSessionRebind={false} />
    </I18N_CONTEXT.Provider>);
    await user.click(screen.getByRole("button", {name: "capability.scheduled_run_in"}));
    await user.click(screen.getByRole("button", {name: "Research"}));
    expect(actions.selectExecution).not.toHaveBeenCalled();
    expect(screen.getByRole("button", {name: "Discussion"})).toBeTruthy();
    await user.click(screen.getByRole("button", {name: "capability.scheduled_target_filter"}));
    expect(screen.getAllByRole("option").map((item) => item.textContent)).toEqual([
      "capability.scheduled_target_all", "capability.scheduled_target_agent", "capability.scheduled_target_room",
    ]);
    await user.click(screen.getByRole("option", {name: "capability.scheduled_target_agent"}));
    expect(screen.getByRole("button", {name: "Nova"})).toBeTruthy();
    expect(screen.queryByRole("button", {name: "Research"})).toBeNull();
    expect(actions.selectExecution).not.toHaveBeenCalled();
    expect(actions.selectDelivery).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", {name: "capability.scheduled_target_filter"}));
    await user.click(screen.getByRole("option", {name: "capability.scheduled_target_all"}));
    expect(screen.getByRole("button", {name: "Nova"})).toBeTruthy();
    expect(screen.getByRole("button", {name: "Research"})).toBeTruthy();
    await user.click(screen.getByRole("button", {name: "capability.scheduled_target_filter"}));
    await user.click(screen.getByRole("option", {name: "capability.scheduled_target_room"}));
    expect(screen.queryByRole("button", {name: "Nova"})).toBeNull();
    expect(actions.selectExecution).not.toHaveBeenCalled();
    await user.type(screen.getByRole("searchbox", {name: "capability.scheduled_search_chats"}), "Research");
    expect(screen.queryByRole("button", {name: "Nova"})).toBeNull();
    await user.click(screen.getByRole("button", {name: "Discussion"}));
    expect(actions.selectExecution).toHaveBeenCalledExactlyOnceWith(room);
  });

  it("keeps unavailable explicit Agent bindings separate from empty/default choices", () => {
    const t = (key: TranslationKey) => key;
    const data = { ...DATA, agentOptions: [{ value: "available", label: "Nova" }] };
    const form = { ...FORM, selectedAgentId: "missing-executor", selectedDeliveryAgentId: "missing-recipient" };
    const input = { data, form, t, search: "", typeFilter: "all", browsingGroup: null };
    const execution = buildTaskDestinationPickerModel({ ...input, execution: true });
    const delivery = buildTaskDestinationPickerModel({ ...input, execution: false,
      form: { ...form, replyMode: "selected", selectedReplySessionKey: "missing-session" } });
    expect(execution.currentValue).toBe("new:missing-executor");
    expect(delivery.currentValue).toBe("missing-session");
    expect(execution.currentLabel).toBe("capability.scheduled_dialog_session_unavailable");
    expect(delivery.currentLabel).toBe("capability.scheduled_dialog_session_unavailable");
    expect(form.selectedAgentId).toBe("missing-executor");
  });

  it("shows unavailable Room executor and presenter without invoking replacement actions", () => {
    const actions = createActions();
    const t = (key: TranslationKey) => key;
    const data = { ...DATA, executionRoomAgentOptions: [{ value: "available", label: "Nova" }], deliveryRoomAgentOptions: [{ value: "available", label: "Nova" }] };
    const form: TaskFormDraft = { ...FORM, targetType: "room", selectedSessionKey: "room:group:one", replyMode: "selected", deliveryTargetType: "room", selectedReplySessionKey: "room:group:two", selectedDeliveryPresenterAgentId: "missing-presenter" };
    render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t }}>
      <TaskBasicsPanel actions={actions} data={data} form={form} needsSessionRebind={false} nameRef={createRef<HTMLInputElement>()} />
    </I18N_CONTEXT.Provider>);
    expect(screen.getByRole("button", { name: "capability.scheduled_dialog_select_room_agent" }).textContent).toContain("agent.selection_unavailable");
    expect(screen.getByRole("button", { name: "capability.scheduled_dialog_select_delivery_room_agent" }).textContent).toContain("agent.selection_unavailable");
    expect(actions.setSelectedAgentId).not.toHaveBeenCalled();
    expect(actions.setSelectedDeliveryPresenterAgentId).not.toHaveBeenCalled();
  });

  it("keeps the title editable and advanced settings collapsed", () => {
    const actions = createActions();
    const {container} = render(<I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: (key) => key}}>
      <TaskBasicsPanel actions={actions} data={DATA} form={FORM} nameRef={createRef<HTMLInputElement>()} needsSessionRebind={false} />
    </I18N_CONTEXT.Provider>);
    fireEvent.change(screen.getByRole("textbox", {name: "capability.scheduled_dialog_task_name"}), {target: {value: "Keep name"}});
    expect(actions.setTaskName).toHaveBeenCalledExactlyOnceWith("Keep name");
    expect([...container.querySelectorAll("details")].every((details) => !details.open)).toBe(true);
  });
});

it("summarizes the current schedule, performer and delivery without inventing a recipient", () => {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES.zh[key]);
  const schedule = {...createDefaultTaskSchedule(), kind: "cron" as const, selectedWeekdays: ["mo", "tu", "we", "th", "fr"] as const, dailyTime: "09:00"};
  const data = {...DATA, agentOptions: [{value: "agent-1", label: "Lucy"}]};
  const summary = buildTaskConfirmationSummary(FORM, {...schedule, selectedWeekdays: [...schedule.selectedWeekdays]}, data, t);
  expect(summary).toContain("工作日 09:00");
  expect(summary).toContain("Lucy");
  expect(summary).toContain(FORM.instruction);
  expect(summary).toContain("结果仅保留在运行记录中");
  const recipient = {value: "chat", sessionKey: "chat", group: "Lucy", label: "行业研究", agentId: "agent-1", roomId: "", targetType: "agent" as const};
  expect(buildTaskConfirmationSummary({...FORM, replyMode: "selected", selectedReplySessionKey: "chat", selectedDeliveryAgentId: "agent-1"}, createDefaultTaskSchedule(), {...data, destinations: [recipient]}, t)).toContain("结果发到「Lucy · 行业研究」聊天");
  expect(buildTaskConfirmationSummary({...FORM, replyMode: "selected"}, createDefaultTaskSchedule(), data, t)).toContain("接收聊天待选择");
});

it.each([false, true])("returns target picker Tab to parent form order (backward=%s)", async (backward) => {
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([{} as DOMRect] as unknown as DOMRectList);
  const user = userEvent.setup();
  render(<I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: (key) => key}}>
    <input aria-label="Before" /><TaskDestinationPicker kind="delivery" data={DATA} form={FORM} actions={createActions()} /><input aria-label="After" />
  </I18N_CONTEXT.Provider>);
  await user.click(screen.getByRole("button", {name: "capability.scheduled_dialog_delivery"}));
  const root = screen.getByRole("dialog");
  const first = within(root).getByRole("searchbox");
  const last = within(root).getByRole("button", {name: "capability.scheduled_dialog_reply_none"});
  (backward ? first : last).focus();
  await user.tab({shift: backward});
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(screen.getByRole("textbox", {name: backward ? "Before" : "After"}));
});

it("preserves loaded destinations during partial failure and retries only idle failed resources", async () => {
  const agentRetry = vi.fn();
  const roomRetry = vi.fn();
  const catalogRetry = vi.fn();
  const actions = createActions();
  const destination = {value: "session", sessionKey: "session", label: "Known chat", group: "Nova", targetType: "agent" as const, agentId: "agent", roomId: ""};
  const data = {...DATA, destinations: [destination],
    agents: {loading: false, error: "private failure", retry: agentRetry},
    rooms: {loading: true, error: "retry pending", retry: roomRetry},
    destinationStatus: {...READY_RESOURCE, retry: catalogRetry}};
  render(<I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: (key) => key}}>
    <TaskDestinationPicker kind="execution" data={data} form={FORM} actions={actions} />
  </I18N_CONTEXT.Provider>);
  await userEvent.click(screen.getByRole("button", {name: "capability.scheduled_run_in"}));
  expect(screen.getByRole("button", {name: "Known chat"})).toBeTruthy();
  expect(screen.queryByText("private failure")).toBeNull();
  await userEvent.click(screen.getByRole("button", {name: "state.retry"}));
  expect(agentRetry).toHaveBeenCalledOnce();
  expect(roomRetry).not.toHaveBeenCalled();
  expect(catalogRetry).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", {name: "Known chat"}));
  expect(actions.selectExecution).toHaveBeenCalledExactlyOnceWith(destination);
});
it.each([false, true])("does not announce empty results for a failed catalog (retrying=%s)", async (loading) => {
  render(<I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: (key) => key}}>
    <TaskDestinationPicker kind="delivery" data={{...DATA, destinationStatus: {loading, error: "unavailable", retry: vi.fn()}}} form={FORM} actions={createActions()} />
  </I18N_CONTEXT.Provider>);
  await userEvent.click(screen.getByRole("button", {name: "capability.scheduled_dialog_delivery"}));
  expect(screen.queryByText("capability.scheduled_no_matching_chats")).toBeNull();
  expect((screen.getByRole("button", {name: "state.retry"}) as HTMLButtonElement).disabled).toBe(loading);
});

it("associates each target trigger with its current selection without exposing missing identities", () => {
  render(<I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: (key) => key}}>
    <TaskDestinationPicker kind="execution" data={DATA} form={{...FORM, executionMode: "existing", selectedSessionKey: "internal-missing-session"}} actions={createActions()} />
    <TaskDestinationPicker kind="delivery" data={DATA} form={FORM} actions={createActions()} />
  </I18N_CONTEXT.Provider>);
  const execution = screen.getByRole("button", {name: "capability.scheduled_run_in"});
  const delivery = screen.getByRole("button", {name: "capability.scheduled_dialog_delivery"});
  const executionDescription = execution.getAttribute("aria-describedby")!;
  const deliveryDescription = delivery.getAttribute("aria-describedby")!;
  expect(executionDescription).not.toBe(deliveryDescription);
  expect(document.getElementById(executionDescription)?.textContent).toBe("capability.scheduled_dialog_session_unavailable");
  expect(document.getElementById(deliveryDescription)?.textContent).toBe("capability.scheduled_dialog_reply_none");
  expect(screen.queryByText("internal-missing-session")).toBeNull();
});


it("exposes independent Room execution and reply members outside advanced settings", async () => {
  const user = userEvent.setup();
  const actions = createActions();
  const data = { ...DATA, defaultExecutionRoomAgentId: "host", defaultDeliveryRoomAgentId: "host",
    executionRoomAgentOptions: [{ value: "nova", label: "Nova" }],
    deliveryRoomAgentOptions: [{ value: "pixel", label: "Pixel" }],
  };
  const form: TaskFormDraft = { ...FORM, targetType: "room", selectedSessionKey: "run-room-session",
    selectedAgentId: "", replyMode: "selected", deliveryTargetType: "room", selectedReplySessionKey: "reply-room-session" };
  render(<I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: (key) => key}}>
    <TaskBasicsPanel actions={actions} data={data} form={form} needsSessionRebind={false} nameRef={createRef<HTMLInputElement>()} />
  </I18N_CONTEXT.Provider>);
  const executor = screen.getByRole("button", {name: "capability.scheduled_dialog_select_room_agent"});
  const presenter = screen.getByRole("button", {name: "capability.scheduled_dialog_select_delivery_room_agent"});
  expect(executor.closest("details")).toBeNull();
  expect(presenter.closest("details")).toBeNull();
  expect(executor.textContent).toContain("capability.scheduled_dialog_default_room_host");
  await user.click(executor);
  await user.click(screen.getByRole("option", {name: "Nova"}));
  expect(actions.setSelectedAgentId).toHaveBeenCalledExactlyOnceWith("nova");
  expect(actions.setSelectedDeliveryPresenterAgentId).not.toHaveBeenCalled();
  await user.click(presenter);
  await user.click(screen.getByRole("option", {name: "Pixel"}));
  expect(actions.setSelectedDeliveryPresenterAgentId).toHaveBeenCalledExactlyOnceWith("pixel");
  expect(actions.setSelectedAgentId).toHaveBeenCalledTimes(1);
  expect(actions.selectExecution).not.toHaveBeenCalled();
  expect(actions.selectDelivery).not.toHaveBeenCalled();
});
