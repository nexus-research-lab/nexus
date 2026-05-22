"use client";

import { Link2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type { ConnectorInfo } from "@/types/capability/connector";

import { ConnectorCard } from "./connector-card";
import { get_connector_category_label } from "./connectors-categories";
import type { ConnectorDirectoryController } from "./connectors-view-model";

interface ConnectorsGridProps {
  ctrl: ConnectorDirectoryController;
  on_open_connector: (connector_id: string) => void;
}

interface ConnectorSection {
  key: string;
  title: string;
  connectors: ConnectorInfo[];
}

function build_connector_sections(
  ctrl: ConnectorDirectoryController,
  t: (key: TranslationKey) => string,
): ConnectorSection[] {
  const is_scoped_view = ctrl.active_category !== "all" || ctrl.search_query.trim() !== "";
  if (is_scoped_view) {
    return [{
      key: "filtered",
      title: ctrl.search_query.trim()
        ? t("capability.connector_section_search_results")
        : get_connector_category_label(ctrl.active_category, t),
      connectors: ctrl.connectors,
    }];
  }

  const available = ctrl.connectors.filter((connector) => connector.status === "available");
  const coming_soon = ctrl.connectors.filter((connector) => connector.status === "coming_soon");
  const sections: ConnectorSection[] = [];

  if (available.length > 0) {
    sections.push({
      key: "featured",
      title: t("capability.connector_section_featured"),
      connectors: available,
    });
  }

  const category_order = ["development", "productivity", "business", "automation", "social", "marketing", "ecommerce"];
  category_order.forEach((category) => {
    const connectors = coming_soon.filter((connector) => connector.category === category);
    if (connectors.length > 0) {
      sections.push({
        key: category,
        title: get_connector_category_label(category, t),
        connectors,
      });
    }
  });

  const known_categories = new Set(category_order);
  const remaining = coming_soon.filter((connector) => !known_categories.has(connector.category));
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
export function ConnectorsGrid({ ctrl, on_open_connector }: ConnectorsGridProps) {
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
        <div className="flex h-14 w-14 items-center justify-center rounded-full border border-(--divider-subtle-color) bg-(--surface-inset-background)">
          <Link2 className="h-6 w-6" />
        </div>
        <p className="text-sm">{t("capability.connectors_empty")}</p>
      </div>
    );
  }

  const sections = build_connector_sections(ctrl, t);

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
                busy={ctrl.busy_id === connector.connector_id}
                connector={connector}
                on_connect={() => void ctrl.handle_connect(connector.connector_id)}
                on_select={() => on_open_connector(connector.connector_id)}
              />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
