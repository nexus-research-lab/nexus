// INPUT: 技能更新响应的部署失败明细与当前语言。
// OUTPUT: 保留更新成功/部分部署失败事实的有限名称摘要，不回显内部 ID 或原始错误。
// POS: 技能操作控制器拥有的反馈纯投影，缺失名称复用共享通称所有者。

import { getAgentDisplayName } from "@/lib/agent-display-name";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { RedeployAgentFailure } from "@/types/capability/skill";

export function formatDeployFailureMessage(
  skillName: string,
  failures: RedeployAgentFailure[] | undefined,
  localization: Pick<I18nContextValue, "locale" | "t">,
): string | null {
  const items = failures?.filter((item) => item.agent_id || item.agent_name || item.error) ?? [];
  if (items.length === 0) return null;

  const agents = items
    .slice(0, 3)
    .map((item) => getAgentDisplayName(item.agent_name, localization.t))
    .join(localization.locale === "en" ? ", " : "、");
  const suffix = items.length > 3
    ? localization.t("capability.skills_agent_list_more", { agents })
    : agents;
  return localization.t("capability.skills_deploy_failed", {
    agents: suffix,
    count: items.length,
    name: skillName,
  });
}
