// INPUT: 服务端 Connector 目录、可用状态、搜索词与分类筛选。
// OUTPUT: 只包含已接入 Connector 的过滤结果及按能力类别组织的目录分区。
// POS: Connector 目录信息架构纯模型；页面不解释上线状态或自行决定分组。

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import type { ConnectorInfo } from "@/types/capability/connector";

import {
  getAvailableConnectorCategoryKeys,
  getConnectorCategoryLabel,
} from "./connectors-categories";

export interface ConnectorSection {
  connectors: ConnectorInfo[];
  key: string;
  title: string;
}

export function filterConnectors(
  connectors: ConnectorInfo[],
  activeCategory: string,
  rawQuery: string,
): ConnectorInfo[] {
  const search = createUiSearchMatcher(rawQuery);
  return connectors.filter((connector) => (
    connector.status === "available"
    && (activeCategory === "all" || connector.category === activeCategory)
    && search.matches([
      connector.title,
      connector.name,
      connector.description,
    ])
  ));
}

export function buildConnectorSections(
  connectors: ConnectorInfo[],
  activeCategory: string,
  rawQuery: string,
  t: I18nContextValue["t"],
): ConnectorSection[] {
  const query = rawQuery.trim();
  if (activeCategory !== "all" || query) {
    return [{
      key: "filtered",
      title: query
        ? t("capability.connector_section_search_results")
        : getConnectorCategoryLabel(activeCategory, t),
      connectors,
    }];
  }

  const availableConnectors = connectors.filter((connector) => (
    connector.status === "available"
  ));
  return getAvailableConnectorCategoryKeys(availableConnectors).map((category) => ({
    key: category,
    title: getConnectorCategoryLabel(category, t),
    connectors: availableConnectors.filter((connector) => (
      connector.category === category
    )),
  }));
}
