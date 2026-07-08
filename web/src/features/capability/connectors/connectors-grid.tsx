"use client";

import { Link2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type { ConnectorInfo } from "@/types/capability/connector";

import { ConnectorCard } from "./connector-card";
import { getConnectorCategoryLabel } from "./connectors-categories";
import type { ConnectorDirectoryController } from "./connectors-view-model";

interface ConnectorsGridProps {
  ctrl: ConnectorDirectoryController;
  onOpenConnector: (connectorId: string) => void;
}

interface ConnectorSection {
  key: string;
  title: string;
  connectors: ConnectorInfo[];
}

function buildConnectorSections(
  ctrl: ConnectorDirectoryController,
  t: (key: TranslationKey) => string,
): ConnectorSection[] {
  const isScopedView = ctrl.activeCategory !== "all" || ctrl.searchQuery.trim() !== "";
  if (isScopedView) {
    return [{
      key: "filtered",
      title: ctrl.searchQuery.trim()
        ? t("capability.connector_section_search_results")
        : getConnectorCategoryLabel(ctrl.activeCategory, t),
      connectors: ctrl.connectors,
    }];
  }

  const available = ctrl.connectors.filter((connector) => connector.status === "available");
  const comingSoon = ctrl.connectors.filter((connector) => connector.status === "coming_soon");
  const sections: ConnectorSection[] = [];

  if (available.length > 0) {
    sections.push({
      key: "featured",
      title: t("capability.connector_section_featured"),
      connectors: available,
    });
  }

  const categoryOrder = ["development", "productivity", "business", "automation", "social", "marketing", "ecommerce"];
  categoryOrder.forEach((category) => {
    const connectors = comingSoon.filter((connector) => connector.category === category);
    if (connectors.length > 0) {
      sections.push({
        key: category,
        title: getConnectorCategoryLabel(category, t),
        connectors,
      });
    }
  });

  const knownCategories = new Set(categoryOrder);
  const remaining = comingSoon.filter((connector) => !knownCategories.has(connector.category));
  if (remaining.length > 0) {
    sections.push({
      key: "other",
      title: t("capability.connector_section_other"),
      connectors: remaining,
    });
  }

  return sections;
}

/** 连接器卡片网格 */
export function ConnectorsGrid({ ctrl, onOpenConnector: onOpenConnector }: ConnectorsGridProps) {
  const { t } = useI18n();

  if (ctrl.loading) {
    return (
      <div className="flex min-h-40 items-center justify-center text-sm text-(--text-muted)">
        {t("capability.connectors_loading")}
      </div>
    );
  }

  if (ctrl.connectors.length === 0) {
    return (
      <div className="flex min-h-60 flex-col items-center justify-center gap-3 text-(--text-muted)">
        <div className="flex h-14 w-14 items-center justify-center rounded-full border border-(--divider-subtle-color) bg-transparent">
          <Link2 className="h-6 w-6" />
        </div>
        <p className="text-sm">{t("capability.connectors_empty")}</p>
      </div>
    );
  }

  const sections = buildConnectorSections(ctrl, t);

  return (
    <div className="space-y-9">
      {sections.map((section) => (
        <section key={section.key}>
          <div className="mb-3 flex items-end justify-between border-b border-(--divider-subtle-color) pb-2">
            <h2 className="text-[18px] font-medium tracking-[-0.025em] text-(--text-strong)">
              {section.title}
            </h2>
            <span className="text-[12px] font-medium text-(--text-soft)">
              {section.connectors.length} 个
            </span>
          </div>
          <div className="grid grid-cols-1 gap-x-12 gap-y-4 md:grid-cols-2">
            {section.connectors.map((connector) => (
              <ConnectorCard
                key={connector.connector_id}
                busy={ctrl.busyId === connector.connector_id}
                connector={connector}
                onConnect={() => void ctrl.handleConnect(connector.connector_id)}
                onSelect={() => onOpenConnector(connector.connector_id)}
              />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
