// INPUT: Scheduled 高级表单草稿、资源状态与字段变更命令。
// OUTPUT: 证明字段实例隔离、选择组具名、共享表单样式及原样输入/精确选择命令。
// POS: Scheduled 基础表单 DOM 合同；不覆盖资源加载与提交事务。

import { fireEvent, render, screen, within } from "@testing-library/react";
import { createRef } from "react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";

import type { TaskFormDraft } from "../scheduled-task-dialog-types";
import { TaskBasicsAdvanced } from "./task-basics-advanced";
import { TaskBasicsPanel } from "./task-basics-panel";
import type { TaskBasicsActions, TaskBasicsData } from "./task-basics-model";
import { buildTaskDeliveryTargetPresentation, buildTaskTargetPresentation } from "./task-basics-model";

const READY_RESOURCE = {
  error: null,
  loading: false,
  retry: vi.fn(),
};

const DATA: TaskBasicsData = {
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

describe("TaskBasicsAdvanced", () => {
  it("keeps unavailable explicit Agent bindings separate from empty/default choices", () => {
    const t = (key: TranslationKey) => key;
    const data = { ...DATA, agentOptions: [{ value: "available", label: "Nova" }] };
    const form = { ...FORM, selectedAgentId: "missing-executor", selectedDeliveryAgentId: "missing-recipient" };
    const execution = buildTaskTargetPresentation(form, data, t);
    const delivery = buildTaskDeliveryTargetPresentation(form, data, t);
    expect(execution.value).toBe("missing-executor");
    expect(delivery.value).toBe("missing-recipient");
    expect(execution.options.find((option) => option.value === execution.value)).toEqual({ value: "missing-executor", label: "agent.selection_unavailable", disabled: true });
    expect(delivery.options.find((option) => option.value === delivery.value)?.disabled).toBe(true);
    expect(execution.options[0].value).toBe("");
    expect(form.selectedAgentId).toBe("missing-executor");
  });

  it("shows unavailable Room executor and presenter without invoking replacement actions", () => {
    const actions = createActions();
    const t = (key: TranslationKey) => key;
    const data = { ...DATA, executionRoomAgentOptions: [{ value: "available", label: "Nova" }], deliveryRoomAgentOptions: [{ value: "available", label: "Nova" }] };
    const form: TaskFormDraft = { ...FORM, targetType: "room", selectedSessionKey: "room:group:one", replyMode: "selected", deliveryTargetType: "room", selectedReplySessionKey: "room:group:two", selectedDeliveryPresenterAgentId: "missing-presenter" };
    render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t }}>
      <TaskBasicsAdvanced actions={actions} data={data} form={form} isEditing={false} needsSessionRebind={false}
        deliveryTarget={buildTaskDeliveryTargetPresentation(form, data, t)}
        deliveryTargetActions={{ agent: actions.setSelectedDeliveryAgentId, room: actions.setSelectedDeliveryRoomId }}
      />
    </I18N_CONTEXT.Provider>);
    expect(screen.getByRole("button", { name: "capability.scheduled_dialog_select_room_agent" }).textContent).toContain("agent.selection_unavailable");
    expect(screen.getByRole("button", { name: "capability.scheduled_dialog_select_delivery_room_agent" }).textContent).toContain("agent.selection_unavailable");
    expect(actions.setSelectedAgentId).not.toHaveBeenCalled();
    expect(actions.setSelectedDeliveryPresenterAgentId).not.toHaveBeenCalled();
  });

  it.each(["agent", "room"] as const)("isolates two %s forms and names their choice groups", async (targetType) => {
    const user = userEvent.setup();
    const actions = [createActions(), createActions()];
    const form: TaskFormDraft = {
      ...FORM, targetType, executionMode: targetType === "agent" ? "dedicated" : "existing",
      selectedRoomId: "room", selectedSessionKey: "execution-session", replyMode: "selected",
      deliveryTargetType: "room", selectedDeliveryRoomId: "delivery-room", selectedReplySessionKey: "delivery-session",
    };
    const { container } = render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
      {actions.map((commands, index) => <section aria-label={`Form ${index}`} key={index}>
        <TaskBasicsPanel actions={commands} data={DATA} form={form} isEditing nameRef={createRef<HTMLInputElement>()} needsSessionRebind />
      </section>)}
    </I18N_CONTEXT.Provider>);
    for (const label of container.querySelectorAll<HTMLLabelElement>("label[for]")) {
      expect(label.control).not.toBeNull();
      expect(label.control?.closest("section[aria-label]")).toBe(label.closest("section[aria-label]"));
    }
    const ids = [...container.querySelectorAll("[id]")].map((element) => element.id);
    expect(new Set(ids).size).toBe(ids.length);
    const second = within(screen.getByRole("region", { name: "Form 1" }));
    const location = second.getByRole("group", { name: "capability.scheduled_dialog_execution_location" });
    expect(document.getElementById(location.getAttribute("aria-describedby")!)?.textContent)
      .toBe("capability.scheduled_dialog_execution_location_help");
    await user.click(within(location).getByRole("button", { name: /room/ }));
    expect(actions[1].setTargetType).toHaveBeenCalledExactlyOnceWith("room");
    expect(actions[0].setTargetType).not.toHaveBeenCalled();
    fireEvent.change(second.getByRole("textbox", { name: "capability.scheduled_dialog_task_name" }), { target: { value: "  Keep task name  " } });
    expect(actions[1].setTaskName).toHaveBeenCalledExactlyOnceWith("  Keep task name  ");
    expect(actions[0].setTaskName).not.toHaveBeenCalled();
    for (const name of ["delivery", "delivery_target_type", "permission_mode"]) {
      expect(second.getAllByRole("group", { name: `capability.scheduled_dialog_${name}` })).toHaveLength(1);
    }
  });

  it("uses shared form chrome and preserves execution-mode selection", async () => {
    const actions = createActions();
    const user = userEvent.setup();
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <TaskBasicsAdvanced
          actions={actions}
          data={DATA}
          deliveryTarget={{
            ariaLabel: "delivery target",
            description: null,
            disabled: true,
            error: null,
            label: "delivery target",
            options: [],
            value: "",
          }}
          deliveryTargetActions={{ agent: vi.fn(), room: vi.fn() }}
          form={FORM}
          isEditing={false}
          needsSessionRebind
        />
      </I18N_CONTEXT.Provider>,
    );

    const warning = screen.getByRole("status");
    expect(warning).toBeTruthy();
    expect(warning.getAttribute("data-inline-notice-tone")).toBe("warning");
    expect(warning.getAttribute("data-inline-notice-width")).toBe("full");
    expect(
      screen.getByText("capability.scheduled_dialog_session_rebind_required").className,
    ).toContain("ui-type-metadata");
    expect(
      screen.getByText("capability.scheduled_dialog_session_rebind_description").className,
    ).toContain("ui-type-metadata");
    expect(container.querySelectorAll("section.surface-radius-md")).toHaveLength(2);

    const details = container.querySelector("details.surface-radius-md");
    expect(details).toBeTruthy();
    expect(details?.querySelector("summary")?.className).toContain("ui-type-control");

    await user.click(screen.getByRole("button", {
      name: "capability.scheduled_dialog_execution_mode_existing",
    }));
    expect(actions.setExecutionMode).toHaveBeenCalledWith("existing");
  });
});
