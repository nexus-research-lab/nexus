import type { TranslationKey } from "@/shared/i18n/messages";

export const CONNECTOR_CATEGORY_OPTIONS: { key: string; label_key: TranslationKey }[] = [
  { key: "all", label_key: "capability.connector_category_all" },
  { key: "productivity", label_key: "capability.connector_category_productivity" },
  { key: "social", label_key: "capability.connector_category_social" },
  { key: "ecommerce", label_key: "capability.connector_category_ecommerce" },
  { key: "development", label_key: "capability.connector_category_development" },
  { key: "business", label_key: "capability.connector_category_business" },
  { key: "marketing", label_key: "capability.connector_category_marketing" },
  { key: "automation", label_key: "capability.connector_category_automation" },
];

export function get_connector_category_label(
  category: string,
  t: (key: TranslationKey) => string,
): string {
  const option = CONNECTOR_CATEGORY_OPTIONS.find((item) => item.key === category);
  return option ? t(option.label_key) : category;
}
