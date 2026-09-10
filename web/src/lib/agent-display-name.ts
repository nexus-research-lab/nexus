// INPUT: 可选的权威名称、当前翻译函数和 Agent/Subagent 展示角色。
// OUTPUT: 非空可读名称，缺失时使用当前语言的通称。
// POS: 跨页面名称兜底的唯一纯 owner；不接收 ID，不解析或改写资源身份。

import type { I18nContextValue } from "@/shared/i18n/i18n-context";

export function getAgentDisplayName(
  name: string | null | undefined,
  t: I18nContextValue["t"],
  kind: "agent" | "subagent" = "agent",
): string {
  return name?.trim() || t(kind === "subagent" ? "agent.subagent_name_fallback" : "agent.name_fallback");
}
