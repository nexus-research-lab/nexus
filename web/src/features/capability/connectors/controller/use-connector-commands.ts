import { useCallback, useState } from "react";

import { getConnectorOauthRedirectUri, isDesktopRuntime } from "@/config/desktop-runtime";
import {
  connectConnectorApi,
  deleteConnectorOauthClientApi,
  disconnectConnectorApi,
  getConnectorAuthUrlApi,
  saveConnectorOauthClientApi,
  startConnectorDeviceAuthApi,
} from "@/lib/api/capability/connector-api";
import { getErrorMessage } from "@/lib/error-message";
import type {
  ConnectorDeviceAuthStart,
  ConnectorInfo,
} from "@/types/capability/connector";

import {
  buildDirectCredentialPayload,
  getDirectCredentialLabel,
  isDirectCredentialAuth,
  resolveConnectorConnectMode,
} from "../auth/connector-auth";
import { openShopPrompt } from "../auth/shop-domain-prompt";
import type { ReportConnectorFeedback } from "./connector-controller-types";
import type {
  ConnectorPendingAction,
  RunConnectorCommand,
} from "./use-connector-command";

interface UseConnectorCommandsOptions {
  connectors: ConnectorInfo[];
  refreshCatalog: () => Promise<void>;
  refreshConnector: (connectorId: string) => Promise<void>;
  reportFeedback: ReportConnectorFeedback;
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
  connectors,
  refreshCatalog,
  refreshConnector,
  reportFeedback,
  runCommand,
}: UseConnectorCommandsOptions) {
  const [deviceAuthSession, setDeviceAuthSession] =
    useState<ConnectorDeviceAuthStart | null>(null);

  const executeMutation = useCallback(async ({
    errorFallback,
    request,
    successMessage,
    action,
  }: MutationOptions): Promise<boolean> => {
    try {
      await request();
      reportFeedback({
        tone: "success",
        title: "操作完成",
        message: successMessage,
      });
      await refreshConnector(action.connectorId);
      return true;
    } catch (error) {
      reportFeedback({
        tone: "error",
        title: "操作失败",
        message: getErrorMessage(error, errorFallback),
      });
      return false;
    }
  }, [refreshConnector, reportFeedback]);

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
      ? await openShopPrompt()
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
      throw new Error("授权地址为空，请检查连接器配置");
    }
    const popup = window.open(
      authUrl,
      "_blank",
      "popup=yes,width=720,height=860",
    );
    if (!popup) {
      throw new Error("授权窗口被浏览器拦截，请允许弹窗后重试");
    }
    reportFeedback({
      tone: "success",
      title: "操作完成",
      message: "已打开授权页面，请在新窗口完成授权",
    });
    return true;
  }, [reportFeedback]);

  const openDeviceOauth = useCallback(async (
    connector: ConnectorInfo,
  ): Promise<boolean> => {
    const session = await startConnectorDeviceAuthApi(connector.connector_id);
    setDeviceAuthSession(session);
    reportFeedback({
      tone: "success",
      title: "操作完成",
      message: "已生成 GitHub 授权码",
    });
    const authUrl = session.verification_uri_complete
      || session.verification_uri;
    if (authUrl) {
      window.open(authUrl, "_blank", "noopener,noreferrer");
    }
    return true;
  }, [reportFeedback]);

  const handleConnect = useCallback(async (
    connectorId: string,
  ): Promise<boolean> => {
    const result = await runCommand({ kind: "connect", connectorId }, async () => {
      const connector = connectors.find((item) => (
        item.connector_id === connectorId
      ));
      if (!connector) {
        reportFeedback({
          tone: "error",
          title: "操作失败",
          message: "连接器不存在",
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
            errorFallback: "连接失败",
            request: () => connectConnectorApi(connectorId),
            successMessage: "连接成功",
          }),
          "direct-credential": async () => {
            throw new Error(
              `请填写 ${getDirectCredentialLabel(connector.auth_type)} 后连接`,
            );
          },
          "oauth-browser": () => openBrowserOauth(connector),
          "oauth-device": () => openDeviceOauth(connector),
        };
        return await strategies[
          resolveConnectorConnectMode(connector, isDesktopRuntime())
        ]();
      } catch (error) {
        reportFeedback({
          tone: "error",
          title: "操作失败",
          message: getErrorMessage(error, "连接失败"),
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
    reportFeedback,
    runCommand,
  ]);

  const handleConnectWithCredential = useCallback((
    connectorId: string,
    credential: string,
  ) => {
    const connector = connectors.find((item) => item.connector_id === connectorId);
    const authType = connector?.auth_type;
    if (!connector || !isDirectCredentialAuth(authType)) {
      reportFeedback({
        tone: "error",
        title: "操作失败",
        message: "当前连接器不支持直接凭证连接",
      });
      return Promise.resolve(false);
    }
    return runMutation({
      action: { kind: "connect-credential", connectorId },
      errorFallback: "连接失败",
      request: () => connectConnectorApi(
        connectorId,
        buildDirectCredentialPayload(authType, credential),
      ),
      successMessage: "连接成功",
    });
  }, [connectors, reportFeedback, runMutation]);

  const handleDisconnect = useCallback((connectorId: string) => runMutation({
    action: { kind: "disconnect", connectorId },
    errorFallback: "断开失败",
    request: () => disconnectConnectorApi(connectorId),
    successMessage: "已断开连接",
  }), [runMutation]);

  const handleSaveOauthClient = useCallback((
    connectorId: string,
    clientId: string,
    clientSecret: string,
  ) => runMutation({
    action: { kind: "save-oauth-client", connectorId },
    errorFallback: "保存配置失败",
    request: () => saveConnectorOauthClientApi(connectorId, {
      client_id: clientId,
      client_secret: clientSecret,
    }),
    successMessage: "应用配置已保存",
  }), [runMutation]);

  const handleDeleteOauthClient = useCallback((connectorId: string) => (
    runMutation({
      action: { kind: "delete-oauth-client", connectorId },
      errorFallback: "删除配置失败",
      request: () => deleteConnectorOauthClientApi(connectorId),
      successMessage: "应用配置已删除",
    })
  ), [runMutation]);

  const handleDeviceConnected = useCallback(async () => {
    reportFeedback({
      tone: "success",
      title: "操作完成",
      message: "GitHub 已连接",
    });
    await refreshCatalog();
  }, [refreshCatalog, reportFeedback]);

  const closeDeviceAuthSession = useCallback(() => {
    setDeviceAuthSession(null);
  }, []);

  return {
    closeDeviceAuthSession,
    deviceAuthSession,
    handleConnect,
    handleConnectWithCredential,
    handleDeleteOauthClient,
    handleDeviceConnected,
    handleDisconnect,
    handleSaveOauthClient,
  };
}
