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
    .map((item) => item.agent_name || item.agent_id || "unknown")
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
