// INPUT: 任务目标草稿、已验证候选、浏览/筛选状态与翻译函数。
// OUTPUT: 目标标签、分组和当前可见会话；浏览与过滤不改写草稿。
// POS: 定时任务目标选择器的纯展示模型；资格判定、资源加载和提交由原所有者负责。
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TaskDestinationOption, TaskFormDraft } from "../scheduled-task-dialog-types";
import type { TaskBasicsData } from "./task-basics-model";
import { buildExecutionModeOptions } from "./task-form-options";

export function buildTaskDestinationPickerModel({
  execution, data, form, search, typeFilter, browsingGroup, t,
}: {
  execution: boolean;
  data: Pick<TaskBasicsData, "agentOptions" | "roomOptions" | "destinations">;
  form: TaskFormDraft;
  search: string;
  typeFilter: string;
  browsingGroup: string | null;
  t: I18nContextValue["t"];
}) {
  const independent = t("capability.scheduled_dialog_execution_mode_temporary");
  const options: TaskDestinationOption[] = execution ? [
    ...data.agentOptions.map((agent) => ({
      value: `new:${agent.value}`, label: independent, group: agent.label,
      targetType: "agent" as const, agentId: agent.value, roomId: "", sessionKey: "",
    })),
    ...data.destinations.filter((option) => option.targetType !== "room"
      || data.roomOptions.some((room) => room.value === option.roomId)),
  ] : data.destinations;
  const currentValue = execution
    ? form.executionMode === "temporary" ? `new:${form.selectedAgentId}` : form.selectedSessionKey
    : form.replyMode === "none" ? "none" : form.selectedReplySessionKey;
  const current = options.find((option) => option.value === currentValue);
  const legacyLabel = execution && (form.executionMode === "main" || form.executionMode === "dedicated")
    ? buildExecutionModeOptions(t).find((option) => option.key === form.executionMode)?.label
    : null;
  const currentLabel = legacyLabel || (currentValue === "none" ? t("capability.scheduled_dialog_reply_none")
    : current ? `${current.group} · ${current.label}` : t(currentValue ? "capability.scheduled_dialog_session_unavailable" : "capability.scheduled_choose_chat"));
  const filtered = options.filter((option) => (typeFilter === "all" || option.targetType === typeFilter)
    && `${option.group} ${option.label} ${option.badge ?? ""}`
      .toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()));
  const groups = new Map<string, TaskDestinationOption[]>();
  for (const option of filtered) {
    const key = `${option.targetType}:${option.agentId || option.roomId}`;
    const group = groups.get(key) ?? [];
    group.push(option);
    groups.set(key, group);
  }
  const currentGroup = current ? `${current.targetType}:${current.agentId || current.roomId}` : "";
  const activeGroup = [browsingGroup, currentGroup, ...groups.keys()].find((key) => key !== null && groups.has(key));
  const visibleItems = activeGroup ? groups.get(activeGroup) ?? [] : [];
  return { currentValue, currentLabel, groups, activeGroup, visibleItems, hasMatches: filtered.length > 0 };
}
