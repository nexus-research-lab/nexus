// INPUT: OAuth 回调的受控结果、当前 Connector 读取入口与 mutation 对账锁。
// OUTPUT: 已完成、明确未连接或结果未知的三问反馈；未知结果只允许重新读取状态。
// POS: OAuth 弹窗事件到 Connector 目录恢复语义的唯一投影边界。
import { useCallback, useEffect } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";

import {
  clearPendingConnectorOauth,
  subscribeConnectorOauthEvent,
  type ConnectorOAuthEvent,
} from "../auth/connector-oauth-events";
import type { ReportConnectorFeedback } from "./connector-controller-types";
import type { ConnectorPendingAction } from "./use-connector-command";

interface UseConnectorOauthEventsOptions {
  completeReconciliation: (connectorId: string) => void;
  reconcileCatalog: () => Promise<boolean>;
  refreshConnector: (connectorId: string) => Promise<boolean>;
  reportFeedback: ReportConnectorFeedback;
  requireReconciliation: (action: ConnectorPendingAction) => void;
}

export function useConnectorOauthEvents({
  completeReconciliation,
  reconcileCatalog,
  refreshConnector,
  reportFeedback,
  requireReconciliation,
}: UseConnectorOauthEventsOptions) {
  const { t } = useI18n();
  const reconcileOauthState = useCallback(async (
    connectorId: string | null,
  ): Promise<void> => {
    const refreshed = connectorId
      ? await refreshConnector(connectorId)
      : await reconcileCatalog();
    if (refreshed) {
      if (connectorId) {
        completeReconciliation(connectorId);
        clearPendingConnectorOauth(connectorId);
      }
      reportFeedback({
        message: t("capability.connector_reconcile_success_message"),
        title: t("capability.connector_reconcile_success_title"),
        tone: "success",
      });
      return;
    }
    reportFeedback({
      action: {
        label: t("capability.connector_reconcile_action"),
        onClick: () => {
          void reconcileOauthState(connectorId);
        },
      },
      impact: t("capability.connector_reconcile_failed_impact"),
      message: t("capability.connector_reconcile_failed_message"),
      nextStep: t("capability.connector_reconcile_failed_next_step"),
      persistent: true,
      ...(connectorId ? { reconciliationConnectorId: connectorId } : {}),
      title: t("capability.connector_reconcile_failed_title"),
      tone: "warning",
    });
  }, [
    completeReconciliation,
    reconcileCatalog,
    refreshConnector,
    reportFeedback,
    t,
  ]);

  const handleOauthEvent = useCallback((event: ConnectorOAuthEvent) => {
    const connectorId = event.connector_id?.trim() || null;
    if (event.type === "connector-oauth:success") {
      reportFeedback({
        message: t("capability.connector_oauth_success_message"),
        title: t("capability.connector_oauth_success_title"),
        tone: "success",
      });
      void reconcileOauthState(connectorId);
      return;
    }

    if (event.failure_kind === "not_connected") {
      clearPendingConnectorOauth(connectorId);
      reportFeedback({
        impact: t("capability.connector_auth_not_completed_impact"),
        message: t("capability.connector_oauth_not_completed_message"),
        nextStep: t("capability.connector_auth_not_completed_next_step"),
        title: t("capability.connector_auth_not_completed_title"),
        tone: "error",
      });
      return;
    }

    if (connectorId) {
      requireReconciliation({ kind: "connect", connectorId });
    }
    reportFeedback({
      action: {
        label: t("capability.connector_reconcile_action"),
        onClick: () => {
          void reconcileOauthState(connectorId);
        },
      },
      impact: t("capability.connector_auth_unknown_impact"),
      message: t("capability.connector_oauth_unknown_message"),
      nextStep: t("capability.connector_auth_unknown_next_step"),
      persistent: true,
      ...(connectorId ? { reconciliationConnectorId: connectorId } : {}),
      title: t("capability.connector_auth_unknown_title"),
      tone: "warning",
    });
  }, [
    reconcileOauthState,
    reportFeedback,
    requireReconciliation,
    t,
  ]);

  useEffect(() => subscribeConnectorOauthEvent(handleOauthEvent), [
    handleOauthEvent,
  ]);
}
