"use client";

import { useCallback, useState } from "react";

import type { ConnectorFeedback } from "./connector-controller-types";
import { useConnectorCatalog } from "./use-connector-catalog";
import { useConnectorCommand } from "./use-connector-command";
import { useConnectorCommands } from "./use-connector-commands";
import { useConnectorDetail } from "./use-connector-detail";

export function useConnectorController() {
  const [feedback, setFeedback] = useState<ConnectorFeedback | null>(null);
  const reportFeedback = useCallback((nextFeedback: ConnectorFeedback) => {
    setFeedback(nextFeedback);
  }, []);
  const reportError = useCallback((message: string) => {
    setFeedback({ tone: "error", title: "操作失败", message });
  }, []);
  const catalog = useConnectorCatalog({ onError: reportError });
  const detail = useConnectorDetail({ onError: reportError });
  const { pendingAction, runCommand } = useConnectorCommand();
  const { refresh: refreshCatalog } = catalog;
  const { refreshDetail } = detail;

  const refreshConnector = useCallback(async (connectorId: string) => {
    await Promise.all([
      refreshCatalog(),
      refreshDetail(connectorId),
    ]);
  }, [refreshCatalog, refreshDetail]);

  const commands = useConnectorCommands({
    connectors: catalog.allConnectors,
    refreshCatalog,
    refreshConnector,
    reportFeedback,
    runCommand,
  });

  const clearFeedback = useCallback(() => setFeedback(null), []);

  return {
    activeCategory: catalog.activeCategory,
    clearFeedback,
    closeDetail: detail.closeDetail,
    connectedCount: catalog.connectedCount,
    connectors: catalog.connectors,
    feedback,
    loading: catalog.loading,
    openDetail: detail.openDetail,
    pendingAction,
    refreshCatalog,
    reportFeedback,
    searchQuery: catalog.searchQuery,
    selectedDetail: detail.selectedDetail,
    detailLoading: detail.detailLoading,
    setActiveCategory: catalog.setActiveCategory,
    setSearchQuery: catalog.setSearchQuery,
    ...commands,
  };
}
