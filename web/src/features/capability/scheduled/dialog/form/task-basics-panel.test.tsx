// INPUT: Scheduled 高级表单草稿、资源状态与字段变更命令。
// OUTPUT: 证明表单分组复用共享 Panel/Typography/radius，且选择行为仍正确派发。
// POS: Scheduled 基础表单 DOM 合同；不覆盖资源加载与提交事务。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import type { TaskFormDraft } from "../scheduled-task-dialog-types";
import { TaskBasicsAdvanced } from "./task-basics-advanced";
import type { TaskBasicsActions, TaskBasicsData } from "./task-basics-model";

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
