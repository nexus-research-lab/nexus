// INPUT: 已完整判别的 Agent 创建/编辑来源与翻译函数。
// OUTPUT: 弹窗唯一可见标题；编辑态使用 Agent 名称，不投影内部 ID。
// POS: Agent Options 模态标题的纯模型。
import type { I18nContextValue } from "@/shared/i18n/i18n-context";

import type { AgentOptionsEditorSource } from "../agent-options-editor-model";

export type AgentOptionsDialogState =
  | { kind: "closed" }
  | AgentOptionsEditorSource;

export function getAgentOptionsDialogTitle(
  source: AgentOptionsEditorSource,
  t: I18nContextValue["t"],
): string {
  if (source.kind === "create") {
    return t("agent_options.title_create");
  }
  return source.initial.title;
}
