import type { TranslationKey } from "@/shared/i18n/messages";

export type SettingsSectionKey =
  | "general"
  | "appearance"
  | "workspace"
  | "permissions"
  | "personal"
  | "providers"
  | "operations";

export interface SettingsNavigationGroup {
  key: "preferences" | "account" | "models" | "management";
  labelKey: TranslationKey;
  items: readonly SettingsNavigationItem[];
}

export interface SettingsNavigationItem {
  key: SettingsSectionKey;
  labelKey: TranslationKey;
}

export const DEFAULT_SETTINGS_SECTION: SettingsSectionKey = "general";

export const SETTINGS_NAVIGATION_GROUPS: readonly SettingsNavigationGroup[] = [
  {
    key: "preferences",
    labelKey: "settings.navigation.preferences",
    items: [
      { key: "general", labelKey: "settings.tabs.general" },
      { key: "appearance", labelKey: "settings.navigation.appearance" },
      { key: "workspace", labelKey: "settings.navigation.workspace" },
      { key: "permissions", labelKey: "settings.navigation.permissions" },
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
