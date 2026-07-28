"use client";

import { Link2 } from "lucide-react";

import { CapabilitySectionHeader } from "@/features/capability/shared/capability-page-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ConnectorInfo } from "@/types/capability/connector";

import type { ConnectorPendingAction } from "../controller/use-connector-command";
import { ConnectorCard } from "./connector-card";
import { buildConnectorSections } from "./connector-catalog-model";

interface ConnectorsGridProps {
  activeCategory: string;
  connectors: ConnectorInfo[];
  loading: boolean;
  onConnect: (connectorId: string) => void;
  onDisconnect: (connectorId: string) => void;
  onOpenConnector: (connectorId: string) => void;
  pendingAction: ConnectorPendingAction | null;
  searchQuery: string;
}

/** 连接器卡片网格 */
export function ConnectorsGrid({
  activeCategory,
  connectors,
  loading,
  onConnect,
  onDisconnect,
  onOpenConnector,
  pendingAction,
  searchQuery,
}: ConnectorsGridProps) {
  const { t } = useI18n();

  if (loading) {
    return (
      <div className="flex min-h-40 items-center justify-center text-sm text-(--text-muted)">
        {t("capability.connectors_loading")}
      </div>
    );
  }

  if (connectors.length === 0) {
    return (
      <div className="flex min-h-48 flex-col items-center justify-center gap-2 text-(--text-muted)">
        <div className="flex h-10 w-10 items-center justify-center rounded-[8px] border border-(--divider-subtle-color) bg-transparent">
          <Link2 className="h-4 w-4" />
        </div>
        <p className="text-compact">{t("capability.connectors_empty")}</p>
      </div>
    );
  }

  const sections = buildConnectorSections(
    connectors,
    activeCategory,
    searchQuery,
    t,
  );

  return (
    <div className="space-y-6">
      {sections.map((section) => (
        <section key={section.key}>
          <CapabilitySectionHeader
            count={t("capability.result_count", {
              count: section.connectors.length,
            })}
            title={section.title}
          />
          <div className="grid grid-cols-1 gap-x-8 gap-y-2 md:grid-cols-2">
            {section.connectors.map((connector) => (
              <ConnectorCard
                key={connector.connector_id}
                busy={pendingAction?.connectorId === connector.connector_id}
                connector={connector}
                onConnect={() => onConnect(connector.connector_id)}
                onDisconnect={() => onDisconnect(connector.connector_id)}
                onSelect={() => onOpenConnector(connector.connector_id)}
              />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
