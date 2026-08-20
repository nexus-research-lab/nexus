import type { TranslationKey } from "@/shared/i18n/messages";

export type SettingsSectionKey =
  | "general"
  | "runtime"
  | "appearance"
  | "workspace"
  | "permissions"
	| "computer-use"
  | "personal"
  | "providers"
  | "operations";

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
		{ key: "computer-use", labelKey: "settings.navigation.computer_use" },
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
    labelKey: "settings.navigation.management",
    items: [
      { key: "operations", labelKey: "operations.title" },
    ],
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
