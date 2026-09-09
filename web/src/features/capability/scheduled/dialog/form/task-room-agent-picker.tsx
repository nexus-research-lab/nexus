// INPUT: 已选 Room 会话、该会话成员候选、默认房主与独立执行/回复变更命令。
// OUTPUT: 在对应运行/接收目标下展示成员选择，空选择保留房主默认语义。
// POS: 定时任务基础目标区的 Room 成员字段；不推断成员资格、不改写另一方向绑定。
import { useId } from "react";

import { includeUnavailableAgentSelection } from "@/lib/agent-selection-options";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiField } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { TaskFormDraft } from "../scheduled-task-dialog-types";
import type { TaskBasicsActions, TaskBasicsData } from "./task-basics-model";

export function TaskRoomAgentPicker({ kind, actions, data, form }: {
  kind: "execution" | "delivery";
  actions: TaskBasicsActions;
  data: TaskBasicsData;
  form: TaskFormDraft;
}) {
  const { t } = useI18n();
  const id = useId();
  const execution = kind === "execution";
  const visible = execution
    ? form.targetType === "room" && Boolean(form.selectedSessionKey)
    : form.replyMode === "selected" && form.deliveryTargetType === "room" && Boolean(form.selectedReplySessionKey);
  if (!visible) return null;
  const options = execution ? data.executionRoomAgentOptions : data.deliveryRoomAgentOptions;
  const defaultAgent = execution ? data.defaultExecutionRoomAgentId : data.defaultDeliveryRoomAgentId;
  const value = execution ? form.selectedAgentId : form.selectedDeliveryPresenterAgentId;
  const label = t(execution ? "capability.scheduled_dialog_execution_agent" : "capability.scheduled_dialog_delivery_room_agent");
  return (
    <UiField
      className="px-3 pb-3"
      htmlFor={id}
      label={label}
      description={t(options.length === 0 ? "capability.scheduled_dialog_no_room_agents"
        : execution ? "capability.scheduled_dialog_room_agent_default_help" : "capability.scheduled_dialog_delivery_room_agent_help")}
    >
      <UiSelectMenu
        id={id}
        ariaLabel={t(execution ? "capability.scheduled_dialog_select_room_agent" : "capability.scheduled_dialog_select_delivery_room_agent")}
        disabled={options.length === 0}
        options={[
          { value: "", label: t(defaultAgent ? "capability.scheduled_dialog_default_room_host" : "capability.scheduled_dialog_select_room_agent") },
          ...includeUnavailableAgentSelection(options, value, t),
        ]}
        value={value}
        onChange={execution ? actions.setSelectedAgentId : actions.setSelectedDeliveryPresenterAgentId}
      />
    </UiField>
  );
}
