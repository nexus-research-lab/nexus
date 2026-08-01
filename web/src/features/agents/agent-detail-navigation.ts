/**
 * INPUT: Agent 详情页可见栏目与翻译键。
 * OUTPUT: Room 与联系人详情共用的无图标栏目顺序和稳定联合类型。
 * POS: Agent 详情导航的信息架构真相源。
 */
import type { TranslationKey } from "@/shared/i18n/messages";

export const AGENT_DETAIL_TABS = [
  { key: "identity", labelKey: "agent_options.nav.identity" },
  { key: "skills", labelKey: "agent_options.nav.skills" },
  { key: "memory", labelKey: "agent_options.nav.memory" },
  { key: "advanced", labelKey: "agent_options.nav.tools" },
  { key: "private_domain", labelKey: "agent_options.nav.contact" },
] as const satisfies ReadonlyArray<{
  key: string;
  labelKey: TranslationKey;
}>;

export type AgentDetailTabKey = (typeof AGENT_DETAIL_TABS)[number]["key"];
