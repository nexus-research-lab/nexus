/**
 * INPUT: runtime 返回的权限更新建议与当前工具名称。
 * OUTPUT: Composer 次级允许菜单的人话动作和生效范围提示。
 * POS: 权限确认面中不依赖 React 的展示策略。
 */
import type { PermissionUpdate } from "@/types/conversation/interaction/permission";

export type PermissionScopeActionTranslationKey =
  | "composer.permission_add_bash_allow_rule"
  | "composer.permission_add_tool_allow_rule";

export type PermissionScopeHintTranslationKey =
  | "composer.permission_scope_local_settings"
  | "composer.permission_scope_project_settings"
  | "composer.permission_scope_session"
  | "composer.permission_scope_user_settings";

export type PermissionToolTitleTranslationKey =
  | "composer.permission_tool_edit"
  | "composer.permission_tool_read"
  | "composer.permission_tool_terminal"
  | "composer.permission_tool_web_fetch"
  | "composer.permission_tool_web_search"
  | "composer.permission_tool_write";

export function getPermissionScopeActionLabelKey(
  toolName: string,
  update: PermissionUpdate | undefined,
): PermissionScopeActionTranslationKey | null {
  const ruleToolNames = update?.rules?.map((rule) => rule.tool_name) ?? [];
  if (
    update?.type === "addRules"
    && update.behavior === "allow"
    && (toolName === "Bash" || ruleToolNames.includes("Bash"))
  ) {
    return "composer.permission_add_bash_allow_rule";
  }
  if (update?.type === "addRules" && update.behavior === "allow") {
    return "composer.permission_add_tool_allow_rule";
  }
  return null;
}

export function getPermissionRuleContent(
  update: PermissionUpdate | undefined,
): string | null {
  const ruleContent = update?.rules
    ?.map((rule) => rule.rule_content?.trim())
    .find(Boolean);
  return ruleContent || null;
}

export function getPermissionScopeHintKey(
  update: PermissionUpdate | undefined,
): PermissionScopeHintTranslationKey | null {
  switch (update?.destination) {
    case "localSettings":
      return "composer.permission_scope_local_settings";
    case "projectSettings":
      return "composer.permission_scope_project_settings";
    case "userSettings":
      return "composer.permission_scope_user_settings";
    case "session":
      return "composer.permission_scope_session";
    default:
      return null;
  }
}

export function getPermissionToolTitleKey(
  toolName: string,
): PermissionToolTitleTranslationKey | null {
  switch (toolName) {
    case "Bash":
      return "composer.permission_tool_terminal";
    case "Edit":
    case "MultiEdit":
      return "composer.permission_tool_edit";
    case "Read":
      return "composer.permission_tool_read";
    case "WebFetch":
      return "composer.permission_tool_web_fetch";
    case "WebSearch":
      return "composer.permission_tool_web_search";
    case "Write":
      return "composer.permission_tool_write";
    default:
      return null;
  }
}
