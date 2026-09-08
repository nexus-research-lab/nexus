/**
 * INPUT: 当前 permission、可读建议、请求 Agent 与本地化函数。
 * OUTPUT: Composer 允许范围菜单项，包括 Automation 的任务级持续授权。
 * POS: 权限确认面范围菜单的纯展示模型。
 */
import type { getReadablePermissionSuggestions } from "@/features/conversation/shared/message/blocks/tool/tool-block-model";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { UiActionMenuItem } from "@/shared/ui/menu/action-menu";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import {
  getPermissionScopeActionLabelKey,
  getPermissionScopeHintKey,
  getPermissionRuleContent,
} from "./composer-permission-model";

export const ALLOW_ONCE_MENU_VALUE = "allow-once";
export const ALLOW_TASK_MENU_VALUE = "allow-task";

export function buildComposerPermissionScopeItems(
  permission: PendingPermission,
  suggestions: ReturnType<typeof getReadablePermissionSuggestions>,
  requesterName: string | undefined,
  t: I18nContextValue["t"],
): UiActionMenuItem[] {
  return [
    {
      label: t("composer.permission_allow_once_menu"),
      description: t("composer.permission_allow_once_description"),
      value: ALLOW_ONCE_MENU_VALUE,
    },
    ...(permission.source === "automation" && permission.automation?.allow_task
      ? [{
        label: t("composer.permission_allow_task"),
        description: t("composer.permission_allow_task_description", {
          name: permission.automation.task_name,
        }),
        value: ALLOW_TASK_MENU_VALUE,
      } satisfies UiActionMenuItem]
      : []),
    ...suggestions.map((suggestion) => {
      const update = permission.suggestions?.[suggestion.index];
      const scopeHintKey = getPermissionScopeHintKey(update);
      const scopeHint = update?.destination === "localSettings" && requesterName
        ? t("composer.permission_scope_agent_local_settings", {
          name: requesterName,
        })
        : scopeHintKey
          ? t(scopeHintKey)
          : null;
      const actionLabelKey = getPermissionScopeActionLabelKey(
        permission.tool_name,
        update,
      );
      const ruleContent = getPermissionRuleContent(update);
      const scopeDescription = scopeHint && ruleContent
        ? t("composer.permission_rule_scope_description", {
          rule: ruleContent,
          scope: scopeHint,
        })
        : scopeHint;
      return {
        label: actionLabelKey ? t(actionLabelKey) : suggestion.label,
        description: scopeDescription,
        value: String(suggestion.index),
      } satisfies UiActionMenuItem;
    }),
  ];
}
