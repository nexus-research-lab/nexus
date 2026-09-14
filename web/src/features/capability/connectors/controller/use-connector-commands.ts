// INPUT: Connector mutation、当前目录与授权交互。
// OUTPUT: 分离写入结果和后续状态刷新的 Connector 命令反馈。
// POS: Connector 命令控制器；结果未知只允许刷新对账，不重复 mutation。
import { useCallback, useRef, useState } from "react";

import { getConnectorOauthRedirectUri, isDesktopRuntime } from "@/config/desktop-runtime";
import {
  connectConnectorApi,
  deleteConnectorOauthClientApi,
  disconnectConnectorApi,
  getConnectorAuthUrlApi,
  saveConnectorOauthClientApi,
  startConnectorDeviceAuthApi,
  startConnectorLocalPairingApi,
} from "@/lib/api/capability/connector-api";
import { getErrorMessage, projectMutationFailure } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  ConnectorDeviceAuthMode,
  ConnectorDeviceAuthStart,
  ConnectorInfo,
  ConnectorLocalPairingStart,
} from "@/types/capability/connector";

import {
  buildDirectCredentialPayload,
  getDirectCredentialLabel,
  isDirectCredentialAuth,
  resolveConnectorConnectMode,
} from "../auth/connector-auth";
import { FeishuWebAuthorizationWindow } from "../auth/feishu/feishu-web-authorization-window";
import type { ConnectorDeviceAuthFailureKind } from "../auth/device-flow/connector-device-auth-poller";
import {
  clearPendingConnectorOauth,
  rememberPendingConnectorOauth,
} from "../auth/connector-oauth-events";
import type { ReportConnectorFeedback } from "./connector-controller-types";
import type {
  ConnectorPendingAction,
  RunConnectorCommand,
} from "./use-connector-command";

interface UseConnectorCommandsOptions {
  completeReconciliation: (connectorId: string) => void;
  connectors: ConnectorInfo[];
  refreshCatalog: () => Promise<void>;
  refreshConnector: (connectorId: string) => Promise<boolean>;
  reportFeedback: ReportConnectorFeedback;
  requireReconciliation: (action: ConnectorPendingAction) => void;
  requestShopDomain: () => Promise<string | null>;
  runCommand: RunConnectorCommand;
}

interface MutationOptions {
  action: ConnectorPendingAction;
  errorFallback: string;
  request: () => Promise<unknown>;
  successMessage: string;
}

function requiresShopDomain(connector: ConnectorInfo): boolean {
  return connector.connector_id === "shopify"
    || connector.requires_extra?.includes("shop") === true;
}

export function useConnectorCommands({
  completeReconciliation,
  connectors,
  refreshCatalog,
  refreshConnector,
  reportFeedback,
  requireReconciliation,
  requestShopDomain,
  runCommand,
}: UseConnectorCommandsOptions) {
  const { t, locale } = useI18n();
  const [deviceAuthSession, setDeviceAuthSession] =
    useState<ConnectorDeviceAuthStart | null>(null);
  const [localPairingSession, setLocalPairingSession] =
    useState<ConnectorLocalPairingStart | null>(null);
  const feishuWebAuthorizationWindowRef =
    useRef<FeishuWebAuthorizationWindow | null>(null);

  const getFeishuWebAuthorizationWindow = useCallback(() => {
    feishuWebAuthorizationWindowRef.current ??=
      new FeishuWebAuthorizationWindow();
    return feishuWebAuthorizationWindowRef.current;
  }, []);

  const closeFeishuWebAuthorizationWindow = useCallback(() => {
    feishuWebAuthorizationWindowRef.current?.close();
    feishuWebAuthorizationWindowRef.current = null;
  }, []);

  const reconcileMutation = useCallback(async (
    action: ConnectorPendingAction,
    effect: "committed" | "accepted" | "unknown" = "unknown",
  ): Promise<void> => {
    const refreshed = await refreshConnector(action.connectorId);
    if (refreshed && effect === "committed") {
      completeReconciliation(action.connectorId);
      reportFeedback({
        message: t("capability.connector_reconcile_success_message"),
        title: t("capability.connector_reconcile_success_title"),
        tone: "success",
      });
      return;
    }
    if (refreshed && effect === "unknown") {
      reportFeedback({
        action: {
          label: t("capability.connector_new_intent_action"),
          onClick: () => completeReconciliation(action.connectorId),
        },
        impact: t("capability.connector_checked_unknown_impact"),
        nextStep: t("capability.connector_checked_unknown_next_step"),
        persistent: true,
        reconciliationConnectorId: action.connectorId,
        title: t("capability.connector_unknown_title"),
        tone: "warning",
      });
      return;
    }
    reportFeedback({
      action: {
        label: t("capability.connector_reconcile_action"),
        onClick: () => {
          void reconcileMutation(action, effect);
        },
      },
      impact: t("capability.connector_reconcile_failed_impact"),
      message: t("capability.connector_reconcile_failed_message"),
      nextStep: t("capability.connector_reconcile_failed_next_step"),
      persistent: true,
      reconciliationConnectorId: action.connectorId,
      title: t("capability.connector_reconcile_failed_title"),
      tone: "warning",
    });
  }, [completeReconciliation, refreshConnector, reportFeedback, t]);

  const openFeishuWebAuthorizationUrl = useCallback((url: string) => (
    !isDesktopRuntime() && getFeishuWebAuthorizationWindow().open(url, locale)
  ), [getFeishuWebAuthorizationWindow, locale]);

  const executeMutation = useCallback(async ({
    errorFallback,
    request,
    successMessage,
    action,
  }: MutationOptions): Promise<boolean> => {
    try {
      await request();
    } catch (error) {
      reportConnectorMutationFailure({
        action,
        error,
        errorFallback,
        reconcileMutation,
        reportFeedback,
        requireReconciliation,
        t,
      });
      return false;
    }
    const refreshed = await refreshConnector(action.connectorId);
    if (refreshed) {
      reportFeedback({
        tone: "success",
        title: t("capability.connector_flow_op_done"),
        message: successMessage,
      });
      return true;
    }
    reportFeedback({
      action: {
        label: t("capability.connector_reconcile_action"),
        onClick: () => {
          void reconcileMutation(action, "committed");
        },
      },
      impact: t("capability.connector_refresh_failed_impact"),
      message: t("capability.connector_refresh_failed_message"),
      nextStep: t("capability.connector_refresh_failed_next_step"),
      persistent: true,
      reconciliationConnectorId: action.connectorId,
      tone: "warning",
      title: t("capability.connector_refresh_failed_title"),
    });
    requireReconciliation(action);
    return true;
  }, [reconcileMutation, refreshConnector, reportFeedback, requireReconciliation, t]);

  const runMutation = useCallback(async (
    options: MutationOptions,
  ): Promise<boolean> => {
    const result = await runCommand(
      options.action,
      () => executeMutation(options),
    );
    return result ?? false;
  }, [executeMutation, runCommand]);

  const openBrowserOauth = useCallback(async (
    connector: ConnectorInfo,
  ): Promise<boolean> => {
    const needsShopDomain = requiresShopDomain(connector);
    const shop = needsShopDomain
      ? await requestShopDomain()
      : undefined;
    if (needsShopDomain && !shop) {
      return false;
    }
    const redirectUri = getConnectorOauthRedirectUri();
    const { auth_url: authUrl } = await getConnectorAuthUrlApi(
      connector.connector_id,
      redirectUri,
      shop ?? undefined,
    );
    if (!authUrl) {
      throw new Error(t("capability.connector_flow_auth_empty"));
    }
    rememberPendingConnectorOauth(connector.connector_id);
    const popup = window.open(
      authUrl,
      "_blank",
      "popup=yes,width=720,height=860",
    );
    if (!popup) {
      clearPendingConnectorOauth(connector.connector_id);
      throw new Error(t("capability.connector_flow_auth_popup_blocked"));
    }
    reportFeedback({
      tone: "success",
      title: t("capability.connector_flow_op_done"),
      message: t("capability.connector_flow_auth_opened"),
    });
    return true;
  }, [reportFeedback, requestShopDomain, t]);

  const openDeviceOauth = useCallback(async (
    connector: ConnectorInfo,
    mode?: ConnectorDeviceAuthMode,
  ): Promise<boolean> => {
    const session = await startConnectorDeviceAuthApi(
      connector.connector_id,
      mode,
    );
    setDeviceAuthSession(session);
    reportFeedback({
      tone: "success",
      title: t("capability.connector_flow_op_done"),
      message: connector.connector_id === "feishu-docx"
        ? mode === "official_qr"
          ? t("capability.connector_flow_feishu_scan_hint")
          : t("capability.connector_flow_feishu_link_hint")
        : t("capability.connector_flow_github_code_ready"),
    });
    const authUrl = session.verification_uri_complete
      || session.verification_uri;
    if (authUrl && connector.connector_id === "github") {
      window.open(authUrl, "_blank", "noopener,noreferrer");
    }
    return true;
  }, [reportFeedback, t]);

  const openLocalPairing = useCallback(async (
    connector: ConnectorInfo,
  ): Promise<boolean> => {
    const session = await startConnectorLocalPairingApi(connector.connector_id);
    setLocalPairingSession(session);
    return true;
  }, []);

  const handleConnect = useCallback(async (
    connectorId: string,
  ): Promise<boolean> => {
    const result = await runCommand({ kind: "connect", connectorId }, async () => {
      const connector = connectors.find((item) => (
        item.connector_id === connectorId
      ));
      if (!connector) {
        reportFeedback({
          impact: t("capability.connector_missing_impact"),
          nextStep: t("capability.connector_missing_next_step"),
          tone: "error",
          title: t("capability.connector_missing_title"),
          message: t("capability.connector_missing_message"),
        });
        return false;
      }
      try {
        const strategies: Record<
          ReturnType<typeof resolveConnectorConnectMode>,
          () => Promise<boolean>
        > = {
          direct: () => executeMutation({
            action: { kind: "connect", connectorId },
            errorFallback: t("capability.connector_flow_connect_failed"),
            request: () => connectConnectorApi(connectorId),
            successMessage: t("capability.connector_flow_connect_success"),
          }),
          "direct-credential": async () => {
            reportFeedback({
              impact: t("capability.connector_credential_required_impact"),
              message: t("capability.connector_credential_required_message", {
                credential: getDirectCredentialLabel(connector.auth_type),
              }),
              nextStep: t("capability.connector_credential_required_next_step"),
              title: t("capability.connector_credential_required_title"),
              tone: "error",
            });
            return false;
          },
          "local-pairing": () => openLocalPairing(connector),
          "oauth-browser": () => openBrowserOauth(connector),
          "oauth-device": () => openDeviceOauth(connector),
        };
        return await strategies[
          resolveConnectorConnectMode(connector, isDesktopRuntime())
        ]();
      } catch {
        const richMailPairing = connector.connector_id === "richmail"
          && connector.auth_type === "local_pairing";
        reportFeedback({
          impact: t(richMailPairing
            ? "capability.richmail_start_failed_impact"
            : "capability.connector_auth_start_failed_impact"),
          nextStep: t(richMailPairing
            ? "capability.richmail_start_failed_next_step"
            : "capability.connector_auth_start_failed_next_step"),
          tone: "error",
          title: t(richMailPairing
            ? "capability.richmail_start_failed_title"
            : "capability.connector_auth_start_failed_title"),
        });
        return false;
      }
    });
    return result ?? false;
  }, [
    connectors,
    executeMutation,
    openBrowserOauth,
    openDeviceOauth,
    openLocalPairing,
    reportFeedback,
    runCommand,
    t,
  ]);

  const handleConnectWithCredential = useCallback((
    connectorId: string,
    credential: string,
  ) => {
    const connector = connectors.find((item) => item.connector_id === connectorId);
    const authType = connector?.auth_type;
    if (!connector || !isDirectCredentialAuth(authType)) {
      reportFeedback({
        impact: t("capability.connector_unsupported_credential_impact"),
        nextStep: t("capability.connector_unsupported_credential_next_step"),
        tone: "error",
        title: t("capability.connector_unsupported_credential_title"),
        message: t("capability.connector_unsupported_credential_message"),
      });
      return Promise.resolve(false);
    }
    return runMutation({
      action: { kind: "connect-credential", connectorId },
      errorFallback: t("capability.connector_flow_connect_failed"),
      request: () => connectConnectorApi(
        connectorId,
        buildDirectCredentialPayload(authType, credential),
      ),
      successMessage: t("capability.connector_flow_connect_success"),
    });
  }, [connectors, reportFeedback, runMutation, t]);

  const handleDisconnect = useCallback((connectorId: string) => runMutation({
    action: { kind: "disconnect", connectorId },
    errorFallback: t("capability.connector_flow_disconnect_failed"),
    request: () => disconnectConnectorApi(connectorId),
    successMessage: t("capability.connector_flow_disconnected"),
  }), [runMutation, t]);

  const reportDeviceAuthFailure = useCallback((
    connectorId: string,
    message: string,
    kind: ConnectorDeviceAuthFailureKind,
  ) => {
    if (kind === "not_connected") {
      reportFeedback({
        impact: t("capability.connector_auth_not_completed_impact"),
        message,
        nextStep: t("capability.connector_auth_not_completed_next_step"),
        title: t("capability.connector_auth_not_completed_title"),
        tone: "error",
      });
      return;
    }
    const action: ConnectorPendingAction = { kind: "connect", connectorId };
    reportFeedback({
      action: {
        label: t("capability.connector_reconcile_action"),
        onClick: () => {
          void reconcileMutation(action);
        },
      },
      impact: t("capability.connector_auth_unknown_impact"),
      message,
      nextStep: t("capability.connector_auth_unknown_next_step"),
      persistent: true,
      reconciliationConnectorId: connectorId,
      title: t("capability.connector_auth_unknown_title"),
      tone: "warning",
    });
    requireReconciliation(action);
  }, [reconcileMutation, reportFeedback, requireReconciliation, t]);

  const handleConnectFeishuWithQr = useCallback(async (
  ): Promise<boolean> => {
    const result = await runCommand({
      kind: "connect",
      connectorId: "feishu-docx",
    }, async () => {
      const connector = connectors.find((item) => (
        item.connector_id === "feishu-docx"
      ));
      if (!connector) {
        reportFeedback({
          impact: t("capability.connector_missing_impact"),
          nextStep: t("capability.connector_missing_next_step"),
          tone: "error",
          title: t("capability.connector_missing_title"),
          message: t("capability.connector_missing_message"),
        });
        return false;
      }
      try {
        return await openDeviceOauth(connector, "official_qr");
      } catch (error) {
        reportDeviceAuthFailure(
          "feishu-docx",
          getErrorMessage(error, t("capability.connector_flow_feishu_scan_failed")),
          "outcome_unknown",
        );
        return false;
      }
    });
    return result ?? false;
  }, [
    connectors,
    openDeviceOauth,
    reportDeviceAuthFailure,
    reportFeedback,
    runCommand,
    t,
  ]);

  const handleConnectFeishuManually = useCallback(async (
    clientId: string,
    clientSecret: string,
  ): Promise<boolean> => {
    const connectorId = "feishu-docx";
    const connector = connectors.find((item) => (
      item.connector_id === connectorId
    ));
    if (!connector) {
      reportFeedback({
        impact: t("capability.connector_missing_impact"),
        nextStep: t("capability.connector_missing_next_step"),
        tone: "error",
        title: t("capability.connector_missing_title"),
        message: t("capability.connector_missing_message"),
      });
      return false;
    }
    const saved = await runMutation({
      action: { kind: "save-oauth-client", connectorId },
      errorFallback: t("capability.connector_flow_feishu_save_failed"),
      request: () => saveConnectorOauthClientApi(connectorId, {
        client_id: clientId,
        client_secret: clientSecret,
      }),
      successMessage: t("capability.connector_flow_feishu_saved"),
    });
    if (!saved) {
      return false;
    }
    const result = await runCommand({ kind: "connect", connectorId }, async () => {
      try {
        return await openDeviceOauth(connector, "manual_credentials");
      } catch (error) {
        reportDeviceAuthFailure(
          connectorId,
          getErrorMessage(error, t("capability.connector_flow_feishu_start_failed")),
          "outcome_unknown",
        );
        return false;
      }
    });
    return result ?? false;
  }, [
    connectors,
    openDeviceOauth,
    reportDeviceAuthFailure,
    reportFeedback,
    runCommand,
    runMutation,
    t,
  ]);

  const handleSaveOauthClient = useCallback((
    connectorId: string,
    clientId: string,
    clientSecret: string,
  ) => runMutation({
    action: { kind: "save-oauth-client", connectorId },
    errorFallback: t("capability.connector_flow_save_failed"),
    request: () => saveConnectorOauthClientApi(connectorId, {
      client_id: clientId,
      client_secret: clientSecret,
    }),
    successMessage: t("capability.connector_flow_saved"),
  }), [runMutation, t]);

  const handleDeleteOauthClient = useCallback((connectorId: string) => (
    runMutation({
      action: { kind: "delete-oauth-client", connectorId },
      errorFallback: t("capability.connector_flow_delete_failed"),
      request: () => deleteConnectorOauthClientApi(connectorId),
      successMessage: t("capability.connector_flow_deleted"),
    })
  ), [runMutation, t]);

  const handleDeviceConnected = useCallback(async () => {
    reportFeedback({
      tone: "success",
      title: t("capability.connector_flow_op_done"),
      message: t("capability.connector_flow_connected"),
    });
    await refreshCatalog();
  }, [refreshCatalog, reportFeedback, t]);

  const closeDeviceAuthSession = useCallback(() => {
    closeFeishuWebAuthorizationWindow();
    setDeviceAuthSession(null);
  }, [closeFeishuWebAuthorizationWindow]);

  const cancelDeviceAuthSession = useCallback(() => {
    closeFeishuWebAuthorizationWindow();
    setDeviceAuthSession(null);
  }, [
    closeFeishuWebAuthorizationWindow,
  ]);

  const handleDeviceAuthFailure = useCallback((
    message: string,
    kind: ConnectorDeviceAuthFailureKind,
  ) => {
    const connectorId = deviceAuthSession?.connector_id;
    if (!connectorId) {
      return;
    }
    reportDeviceAuthFailure(connectorId, message, kind);
  }, [deviceAuthSession, reportDeviceAuthFailure]);

  const continueDeviceAuthSession = useCallback((
    session: ConnectorDeviceAuthStart,
  ) => {
    setDeviceAuthSession(session);
  }, []);

  const closeLocalPairingSession = useCallback(() => {
    setLocalPairingSession(null);
  }, []);

  const handleLocalPairingFailure = useCallback((
    message: string,
    kind: ConnectorDeviceAuthFailureKind,
  ) => {
    const connectorId = localPairingSession?.connector_id;
    if (!connectorId) {
      return;
    }
    reportDeviceAuthFailure(connectorId, message, kind);
  }, [localPairingSession, reportDeviceAuthFailure]);

  return {
    cancelDeviceAuthSession,
    closeDeviceAuthSession,
    closeLocalPairingSession,
    continueDeviceAuthSession,
    deviceAuthSession,
    handleConnect,
    handleConnectFeishuManually,
    handleConnectFeishuWithQr,
    handleConnectWithCredential,
    handleDeleteOauthClient,
    handleDeviceAuthFailure,
    handleDeviceConnected,
    handleLocalPairingFailure,
    handleDisconnect,
    handleSaveOauthClient,
    localPairingSession,
    openFeishuWebAuthorizationUrl,
  };
}

function reportConnectorMutationFailure({
  action,
  error,
  errorFallback,
  reconcileMutation,
  reportFeedback,
  requireReconciliation,
  t,
}: {
  action: ConnectorPendingAction;
  error: unknown;
  errorFallback: string;
  reconcileMutation: (action: ConnectorPendingAction, effect: "committed" | "accepted" | "unknown") => Promise<void>;
  reportFeedback: ReportConnectorFeedback;
  requireReconciliation: (action: ConnectorPendingAction) => void;
  t: ReturnType<typeof useI18n>["t"];
}) {
  const failure = projectMutationFailure(error, errorFallback);
  const outcome = failure.effect === "accepted"
    || failure.effect === "committed"
    || failure.effect === "not_applied"
    ? failure.effect
    : "unknown";
  const notApplied = outcome === "not_applied";
  const copy = {
    accepted: {
      impact: "capability.connector_accepted_impact",
      nextStep: "capability.connector_accepted_next_step",
      title: "capability.connector_accepted_title",
    },
    committed: {
      impact: "capability.connector_committed_impact",
      nextStep: "capability.connector_committed_next_step",
      title: "capability.connector_committed_title",
    },
    not_applied: {
      impact: "capability.connector_not_applied_impact",
      nextStep: "capability.connector_not_applied_next_step",
      title: "capability.connector_not_applied_title",
    },
    unknown: {
      impact: "capability.connector_unknown_impact",
      nextStep: "capability.connector_unknown_next_step",
      title: "capability.connector_unknown_title",
    },
  } as const;
  const selectedCopy = copy[outcome];
  reportFeedback({
    action: notApplied
      ? undefined
      : {
          label: t("capability.connector_reconcile_action"),
          onClick: () => {
            void reconcileMutation(action, outcome);
          },
    },
    impact: t(selectedCopy.impact),
    nextStep: t(selectedCopy.nextStep),
    persistent: !notApplied,
    reconciliationConnectorId: notApplied ? undefined : action.connectorId,
    tone: notApplied ? "error" : "warning",
    title: t(selectedCopy.title),
  });
  if (!notApplied) {
    requireReconciliation(action);
  }
}
