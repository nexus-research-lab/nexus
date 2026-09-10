// INPUT: 设置分区名称与 URL 查询参数。
// OUTPUT: 侧栏分组、运营子页识别与当前页面名称。
// POS: 设置导航单一目录；不读取权限或持有渲染状态。
import type { TranslationKey } from "@/shared/i18n/messages";

export const OPERATIONS_SECTIONS = [
  { key: "operations-members", labelKey: "operations.tabs.members" },
  { key: "operations-subscriptions", labelKey: "operations.tabs.user_subscriptions" },
  { key: "operations-plans", labelKey: "operations.tabs.subscription_plans" },
  { key: "operations-providers", labelKey: "operations.tabs.subscription_providers" },
  { key: "operations-projects", labelKey: "operations.tabs.projects" },
] as const;

export type OperationsSectionKey = typeof OPERATIONS_SECTIONS[number]["key"];

export function isOperationsSection(section: string): section is OperationsSectionKey {
  return OPERATIONS_SECTIONS.some((item) => item.key === section);
}

export type SettingsSectionKey =
  | "general"
  | "runtime"
  | "appearance"
  | "workspace"
  | "permissions"
	| "browser"
  | "personal"
  | "providers"
  | OperationsSectionKey;

export interface SettingsNavigationGroup {
  key: "preferences" | "account" | "models" | "management";
  labelKey: TranslationKey;
  items: readonly SettingsNavigationItem[];
}

interface SettingsNavigationItem {
  key: SettingsSectionKey;
  labelKey: TranslationKey;
}

const DEFAULT_SETTINGS_SECTION: SettingsSectionKey = "general";

export const SETTINGS_NAVIGATION_GROUPS: readonly SettingsNavigationGroup[] = [
  {
    key: "preferences",
    labelKey: "settings.navigation.preferences",
    items: [
      { key: "general", labelKey: "settings.tabs.general" },
      { key: "runtime", labelKey: "settings.tabs.runtime" },
      { key: "appearance", labelKey: "settings.navigation.appearance" },
      { key: "workspace", labelKey: "settings.navigation.workspace" },
      { key: "permissions", labelKey: "settings.navigation.permissions" },
		{ key: "browser", labelKey: "settings.navigation.browser" },
    ],
  },
  {
    key: "account",
    labelKey: "settings.navigation.account",
    items: [
      { key: "personal", labelKey: "settings.tabs.personal" },
    ],
  },
  {
    key: "models",
    labelKey: "settings.navigation.models",
    items: [
      { key: "providers", labelKey: "settings.tabs.providers" },
    ],
  },
  {
    key: "management",
    labelKey: "operations.page_title",
    items: OPERATIONS_SECTIONS,
  },
] as const;

const SETTINGS_SECTION_KEYS = new Set<SettingsSectionKey>(
  SETTINGS_NAVIGATION_GROUPS.flatMap((group) =>
    group.items.map((item) => item.key),
  ),
);

export function parseSettingsSection(
  searchParams: URLSearchParams,
): SettingsSectionKey {
  const section = searchParams.get("section");
  if (section === "operations") return "operations-members";
  return section && SETTINGS_SECTION_KEYS.has(section as SettingsSectionKey)
    ? (section as SettingsSectionKey)
    : DEFAULT_SETTINGS_SECTION;
}

export function getSettingsSectionLabelKey(
  section: SettingsSectionKey,
): TranslationKey {
  for (const group of SETTINGS_NAVIGATION_GROUPS) {
    const item = group.items.find((entry) => entry.key === section);
    if (item) {
      return item.labelKey;
    }
  }
  return "settings.title";
}
