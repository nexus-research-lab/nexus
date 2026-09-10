// INPUT: Connector catalog/detail、mutation 命令与对账反馈。
// OUTPUT: 目录页使用的窄控制器和按 Connector 隔离的恢复锁。
// POS: Connector 页面编排边界；读取失败和 mutation 结果不可互相推断。
"use client";

import { useCallback, useRef, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";

import { useShopDomainPrompt } from "../auth/shop-domain/use-shop-domain-prompt";
import type { ConnectorFeedback } from "./connector-controller-types";
import {
  removeConnectorReconciliationFeedback,
  upsertConnectorReconciliationFeedback,
} from "./connector-reconciliation-model";
import { useConnectorCatalog } from "./use-connector-catalog";
import { useConnectorCommand } from "./use-connector-command";
import { useConnectorCommands } from "./use-connector-commands";
import { useConnectorDetail } from "./use-connector-detail";

export function useConnectorController() {
  const { t } = useI18n();
  const [feedback, setFeedback] = useState<ConnectorFeedback | null>(null);
  const reconciliationFeedbackRef = useRef(
    new Map<string, ConnectorFeedback>(),
  );
  const [reconciliationFeedbacks, setReconciliationFeedbacks] =
    useState<ConnectorFeedback[]>([]);
  const reportFeedback = useCallback((nextFeedback: ConnectorFeedback) => {
    setFeedback(nextFeedback);
    const connectorId = nextFeedback.reconciliationConnectorId;
    if (!connectorId) {
      return;
    }
    reconciliationFeedbackRef.current = upsertConnectorReconciliationFeedback(
      reconciliationFeedbackRef.current,
      connectorId,
      nextFeedback,
    );
    setReconciliationFeedbacks([
      ...reconciliationFeedbackRef.current.values(),
    ]);
  }, []);
  const catalog = useConnectorCatalog({
    failureFallback: t("capability.connector_catalog_load_failed_message"),
  });
  const detail = useConnectorDetail({
    failureFallback: t("capability.connector_detail_load_failed_message"),
  });
  const shopDomainPrompt = useShopDomainPrompt();
  const {
    completeReconciliation: completeCommandReconciliation,
    pendingAction,
    reconciliationActions,
    requireReconciliation,
    runCommand,
  } = useConnectorCommand();
  const completeReconciliation = useCallback((connectorId: string) => {
    completeCommandReconciliation(connectorId);
    if (!reconciliationFeedbackRef.current.has(connectorId)) {
      return;
    }
    reconciliationFeedbackRef.current = removeConnectorReconciliationFeedback(
      reconciliationFeedbackRef.current,
      connectorId,
    );
    setReconciliationFeedbacks([
      ...reconciliationFeedbackRef.current.values(),
    ]);
  }, [completeCommandReconciliation]);
  const { refresh: refreshCatalog } = catalog;
  const { reconcile: reconcileCatalog } = catalog;
  const { reconcileDetail } = detail;

  const refreshConnector = useCallback(async (connectorId: string) => {
    const results = await Promise.all([
      reconcileCatalog(),
      reconcileDetail(connectorId),
    ]);
    return results.every(Boolean);
  }, [reconcileCatalog, reconcileDetail]);

  const commands = useConnectorCommands({
    completeReconciliation,
    connectors: catalog.allConnectors,
    refreshCatalog,
    refreshConnector,
    reportFeedback,
    requireReconciliation,
    requestShopDomain: shopDomainPrompt.request,
    runCommand,
  });

  const clearFeedback = useCallback(() => setFeedback(null), []);

  return {
    activeCategory: catalog.activeCategory,
    categoryKeys: catalog.categoryKeys,
    clearFeedback,
    closeDetail: detail.closeDetail,
    connectors: catalog.connectors,
    catalogFailure: catalog.failure,
    completeReconciliation,
    feedback,
    loading: catalog.loading,
    openDetail: detail.openDetail,
    pendingAction,
    reconciliationActions,
    reconciliationFeedbacks,
    requireReconciliation,
    refreshCatalog,
    reconcileCatalog,
    refreshConnector,
    reportFeedback,
    searchQuery: catalog.searchQuery,
    shopDomainPrompt: shopDomainPrompt.state,
    cancelShopDomainPrompt: shopDomainPrompt.cancel,
    confirmShopDomainPrompt: shopDomainPrompt.confirm,
    selectedDetail: detail.selectedDetail,
    detailFailure: detail.failure,
    detailLoading: detail.detailLoading,
    setActiveCategory: catalog.setActiveCategory,
    setSearchQuery: catalog.setSearchQuery,
    ...commands,
  };
}
