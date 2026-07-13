import type { TranslationKey } from "@/shared/i18n/messages";

export const CONNECTOR_CATEGORY_OPTIONS: { key: string; labelKey: TranslationKey }[] = [
  { key: "all", labelKey: "capability.connector_category_all" },
  { key: "productivity", labelKey: "capability.connector_category_productivity" },
  { key: "social", labelKey: "capability.connector_category_social" },
  { key: "ecommerce", labelKey: "capability.connector_category_ecommerce" },
  { key: "development", labelKey: "capability.connector_category_development" },
  { key: "business", labelKey: "capability.connector_category_business" },
  { key: "marketing", labelKey: "capability.connector_category_marketing" },
  { key: "automation", labelKey: "capability.connector_category_automation" },
];

export function getConnectorCategoryLabel(
  category: string,
  t: (key: TranslationKey) => string,
): string {
  const option = CONNECTOR_CATEGORY_OPTIONS.find((item) => item.key === category);
  return option ? t(option.labelKey) : category;
}
