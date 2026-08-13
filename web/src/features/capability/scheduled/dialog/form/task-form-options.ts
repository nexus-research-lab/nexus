import { AGENT_PERMISSION_MODES } from "@/lib/agent-options";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";

import type {
  ChoiceDef,
  ExecutionMode,
  PermissionMode,
  ReplyMode,
  TargetType,
} from "../scheduled-task-dialog-types";

type Translate = I18nContextValue["t"];

export function buildTargetTypeOptions(t: Translate): ChoiceDef<TargetType>[] {
  return [
    { key: "agent", label: t("capability.scheduled_dialog_target_type_agent") },
    { key: "room", label: t("capability.scheduled_dialog_target_type_room") },
  ];
}

export function buildDeliveryTargetTypeOptions(
  t: Translate,
): ChoiceDef<TargetType>[] {
  return [
    {
      key: "agent",
      label: t("capability.scheduled_dialog_delivery_target_type_agent"),
    },
    {
      key: "room",
      label: t("capability.scheduled_dialog_delivery_target_type_room"),
    },
  ];
}

export function buildExecutionModeOptions(
  t: Translate,
): ChoiceDef<ExecutionMode>[] {
  return [
    { key: "main", label: t("capability.scheduled_dialog_execution_mode_main") },
    { key: "existing", label: t("capability.scheduled_dialog_execution_mode_existing") },
    { key: "temporary", label: t("capability.scheduled_dialog_execution_mode_temporary") },
    { key: "dedicated", label: t("capability.scheduled_dialog_execution_mode_dedicated") },
  ];
}

export function buildReplyModeOptions(t: Translate): ChoiceDef<ReplyMode>[] {
  return [
    { key: "none", label: t("capability.scheduled_dialog_reply_none") },
    { key: "selected", label: t("capability.scheduled_dialog_reply_selected") },
  ];
}

export function buildPermissionModeOptions(
  t: Translate,
  includeCopy = true,
): ChoiceDef<PermissionMode>[] {
  const persisted = AGENT_PERMISSION_MODES.map((option) => ({
    key: option.value,
    label: t(option.labelKey),
  }));
  return includeCopy
    ? [{
        key: "copy",
        label: t("capability.scheduled_dialog_permission_copy"),
      }, ...persisted]
    : persisted;
}

export function getPermissionModeHelp(
  permissionMode: PermissionMode,
  t: Translate,
): string {
  if (permissionMode === "copy") {
    return t("capability.scheduled_dialog_permission_copy_help");
  }
  const option = AGENT_PERMISSION_MODES.find(
    (candidate) => candidate.value === permissionMode,
  );
  return option ? t(option.descriptionKey) : "";
}

export function getExecutionModeHelp(
  executionMode: ExecutionMode,
  t: Translate,
): string {
  const keys = {
    dedicated: "capability.scheduled_dialog_execution_mode_dedicated_help",
    existing: "capability.scheduled_dialog_execution_mode_existing_help",
    main: "capability.scheduled_dialog_execution_mode_main_help",
    temporary: "capability.scheduled_dialog_execution_mode_temporary_help",
  } as const;
  return t(keys[executionMode]);
}

export function getReplyModeHelp(replyMode: ReplyMode, t: Translate): string {
  const keys = {
    none: "capability.scheduled_dialog_reply_none_help",
    selected: "capability.scheduled_dialog_reply_selected_help",
  } as const;
  return t(keys[replyMode]);
}
