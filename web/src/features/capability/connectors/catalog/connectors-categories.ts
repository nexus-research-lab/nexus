// INPUT: Connector 目录项中的服务端分类键与当前语言翻译函数。
// OUTPUT: 稳定分类顺序、仅含当前可用分类的筛选键，以及用户可读名称。
// POS: Connector 分类语义的唯一前端投影；页面不得自行维护分类名单或顺序。

import type { TranslationKey } from "@/shared/i18n/messages";
import type { ConnectorInfo } from "@/types/capability/connector";

const CONNECTOR_CATEGORY_OPTIONS: { key: string; labelKey: TranslationKey }[] = [
  { key: "all", labelKey: "capability.connector_category_all" },
  { key: "productivity", labelKey: "capability.connector_category_productivity" },
  { key: "business", labelKey: "capability.connector_category_business" },
  { key: "development", labelKey: "capability.connector_category_development" },
  { key: "social", labelKey: "capability.connector_category_social" },
  { key: "ecommerce", labelKey: "capability.connector_category_ecommerce" },
  { key: "marketing", labelKey: "capability.connector_category_marketing" },
  { key: "automation", labelKey: "capability.connector_category_automation" },
];

const CONNECTOR_CATEGORY_ORDER = CONNECTOR_CATEGORY_OPTIONS
  .filter((item) => item.key !== "all")
  .map((item) => item.key);

export function getAvailableConnectorCategoryKeys(
  connectors: ConnectorInfo[],
): string[] {
  const availableKeys = new Set(
    connectors
      .filter((connector) => connector.status === "available")
      .map((connector) => connector.category),
  );
  const knownKeys = CONNECTOR_CATEGORY_ORDER.filter((key) => (
    availableKeys.delete(key)
  ));
  return [...knownKeys, ...Array.from(availableKeys).sort()];
}

export function getConnectorCategoryLabel(
  category: string,
  t: (key: TranslationKey) => string,
): string {
  const option = CONNECTOR_CATEGORY_OPTIONS.find((item) => item.key === category);
  return option ? t(option.labelKey) : category;
}
