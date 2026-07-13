import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { ConnectorInfo } from "@/types/capability/connector";

import { getConnectorCategoryLabel } from "./connectors-categories";

export interface ConnectorSection {
  connectors: ConnectorInfo[];
  key: string;
  title: string;
}

const COMING_SOON_CATEGORY_ORDER = [
  "development",
  "productivity",
  "business",
  "automation",
  "social",
  "marketing",
  "ecommerce",
];

export function filterConnectors(
  connectors: ConnectorInfo[],
  activeCategory: string,
  rawQuery: string,
): ConnectorInfo[] {
  const query = rawQuery.trim().toLowerCase();
  return connectors.filter((connector) => (
    (activeCategory === "all" || connector.category === activeCategory)
    && (!query || [
      connector.title,
      connector.name,
      connector.description,
    ].some((value) => value.toLowerCase().includes(query)))
  ));
}

export function countConnectedConnectors(connectors: ConnectorInfo[]): number {
  return connectors.filter((connector) => (
    connector.connection_state === "connected"
  )).length;
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

  const available = connectors.filter((connector) => (
    connector.status === "available"
  ));
  const comingSoon = connectors.filter((connector) => (
    connector.status === "coming_soon"
  ));
  const categorized = new Set<string>();
  const sections: ConnectorSection[] = available.length > 0 ? [{
    key: "featured",
    title: t("capability.connector_section_featured"),
    connectors: available,
  }] : [];

  for (const category of COMING_SOON_CATEGORY_ORDER) {
    const categoryConnectors = comingSoon.filter((connector) => (
      connector.category === category
    ));
    if (categoryConnectors.length === 0) {
      continue;
    }
    categorized.add(category);
    sections.push({
      key: category,
      title: getConnectorCategoryLabel(category, t),
      connectors: categoryConnectors,
    });
  }

  const remaining = comingSoon.filter((connector) => (
    !categorized.has(connector.category)
  ));
  if (remaining.length > 0) {
    sections.push({
      key: "other",
      title: t("capability.connector_section_other"),
      connectors: remaining,
    });
  }
  return sections;
}
