// INPUT: 混合上线状态、分类和搜索文本的 Connector 目录夹具。
// OUTPUT: 证明未上线项永不进入目录，已接入项按稳定能力分类组织。
// POS: Connector 目录信息架构合同；不测试网络读取或卡片交互。

import { describe, expect, it } from "vitest";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { ConnectorInfo } from "@/types/capability/connector";

import {
  buildConnectorSections,
  filterConnectors,
} from "./connector-catalog-model";
import { getAvailableConnectorCategoryKeys } from "./connectors-categories";

function connector(
  connectorId: string,
  category: string,
  status: ConnectorInfo["status"] = "available",
): ConnectorInfo {
  return {
    auth_type: "oauth2",
    category,
    connection_state: "disconnected",
    connector_id: connectorId,
    description: `${connectorId} description`,
    icon: connectorId,
    is_configured: true,
    kind: "connector",
    name: connectorId,
    status,
    title: connectorId,
  };
}

const t = ((key: string) => ({
  "capability.connector_category_business": "地图与出行工具",
  "capability.connector_category_development": "开发协作工具",
  "capability.connector_category_productivity": "文档与效率工具",
  "capability.connector_section_search_results": "搜索结果",
}[key] ?? key)) as I18nContextValue["t"];

describe("Connector catalog model", () => {
  const catalog = [
    connector("RichMail", "productivity"),
    connector("高德地图", "business"),
    connector("GitHub", "development"),
    connector("Outlook", "productivity", "coming_soon"),
  ];

  it("filters out every unavailable connector before search or category matching", () => {
    expect(filterConnectors(catalog, "all", "").map((item) => item.title))
      .toEqual(["RichMail", "高德地图", "GitHub"]);
    expect(filterConnectors(catalog, "productivity", "outlook"))
      .toEqual([]);
  });

  it("groups available connectors by stable user-facing capability categories", () => {
    const sections = buildConnectorSections(catalog, "all", "", t);

    expect(sections.map((section) => ({
      connectors: section.connectors.map((item) => item.title),
      key: section.key,
      title: section.title,
    }))).toEqual([
      {
        connectors: ["RichMail"],
        key: "productivity",
        title: "文档与效率工具",
      },
      {
        connectors: ["高德地图"],
        key: "business",
        title: "地图与出行工具",
      },
      {
        connectors: ["GitHub"],
        key: "development",
        title: "开发协作工具",
      },
    ]);
  });

  it("offers only categories that contain available connectors", () => {
    expect(getAvailableConnectorCategoryKeys(catalog)).toEqual([
      "productivity",
      "business",
      "development",
    ]);
  });
});
