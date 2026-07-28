import { useCallback, useRef, useState } from "react";

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
  ConnectorDeviceAuthMode,
  ConnectorDeviceAuthStart,
  ConnectorInfo,
} from "@/types/capability/connector";

import {
  buildDirectCredentialPayload,
  getDirectCredentialLabel,
  isDirectCredentialAuth,
  resolveConnectorConnectMode,
} from "../auth/connector-auth";
import { FeishuWebAuthorizationWindow } from "../auth/feishu/feishu-web-authorization-window";
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
  connectors,
  refreshCatalog,
  refreshConnector,
  reportFeedback,
  requestShopDomain,
  runCommand,
}: UseConnectorCommandsOptions) {
  const [deviceAuthSession, setDeviceAuthSession] =
    useState<ConnectorDeviceAuthStart | null>(null);
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

  const openFeishuWebAuthorizationUrl = useCallback((url: string) => (
    !isDesktopRuntime() && getFeishuWebAuthorizationWindow().open(url)
  ), [getFeishuWebAuthorizationWindow]);

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
  }, [reportFeedback, requestShopDomain]);

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
      title: "操作完成",
      message: connector.connector_id === "feishu-docx"
        ? mode === "official_qr"
          ? "请使用飞书扫描二维码选择或创建应用"
          : "请打开飞书授权链接完成连接"
        : "已生成 GitHub 授权码",
    });
    const authUrl = session.verification_uri_complete
      || session.verification_uri;
    if (authUrl && connector.connector_id === "github") {
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
          tone: "error",
          title: "操作失败",
          message: "飞书云文档连接器不存在",
        });
        return false;
      }
      try {
        return await openDeviceOauth(connector, "official_qr");
      } catch (error) {
        try {
          await refreshConnector("feishu-docx");
        } catch {
          // 保留扫码启动的原始失败；下一次目录刷新会同步服务端已清理状态。
        }
        reportFeedback({
          tone: "error",
          title: "操作失败",
          message: getErrorMessage(error, "启动飞书扫码连接失败"),
        });
        return false;
      }
    });
    return result ?? false;
  }, [
    connectors,
    openDeviceOauth,
    refreshConnector,
    reportFeedback,
    runCommand,
  ]);

  const handleConnectFeishuManually = useCallback(async (
    clientId: string,
    clientSecret: string,
  ): Promise<boolean> => {
    const connectorId = "feishu-docx";
    const result = await runCommand({
      kind: "connect",
      connectorId,
    }, async () => {
      const connector = connectors.find((item) => (
        item.connector_id === connectorId
      ));
      if (!connector) {
        reportFeedback({
          tone: "error",
          title: "操作失败",
          message: "飞书云文档连接器不存在",
        });
        return false;
      }
      try {
        await deleteConnectorOauthClientApi(connectorId);
        await saveConnectorOauthClientApi(connectorId, {
          client_id: clientId,
          client_secret: clientSecret,
        });
        return await openDeviceOauth(connector, "manual_credentials");
      } catch (error) {
        try {
          await deleteConnectorOauthClientApi(connectorId);
          await refreshConnector(connectorId);
        } catch {
          // 原始连接错误更能说明失败原因，清理失败留给下一次显式连接覆盖。
        }
        reportFeedback({
          tone: "error",
          title: "操作失败",
          message: getErrorMessage(error, "手动连接飞书应用失败"),
        });
        return false;
      }
    });
    return result ?? false;
  }, [
    connectors,
    openDeviceOauth,
    refreshConnector,
    reportFeedback,
    runCommand,
  ]);

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
      message: "连接器已连接",
    });
    await refreshCatalog();
  }, [refreshCatalog, reportFeedback]);

  const closeDeviceAuthSession = useCallback(() => {
    closeFeishuWebAuthorizationWindow();
    setDeviceAuthSession(null);
  }, [closeFeishuWebAuthorizationWindow]);

  const cancelDeviceAuthSession = useCallback(async () => {
    const session = deviceAuthSession;
    closeFeishuWebAuthorizationWindow();
    setDeviceAuthSession(null);
    if (session?.connector_id !== "feishu-docx") {
      return;
    }
    try {
      await deleteConnectorOauthClientApi(session.connector_id);
      await refreshConnector(session.connector_id);
    } catch (error) {
      reportFeedback({
        tone: "error",
        title: "清理失败",
        message: getErrorMessage(error, "未能清理飞书应用连接状态"),
      });
    }
  }, [
    closeFeishuWebAuthorizationWindow,
    deviceAuthSession,
    refreshConnector,
    reportFeedback,
  ]);

  const continueDeviceAuthSession = useCallback((
    session: ConnectorDeviceAuthStart,
  ) => {
    setDeviceAuthSession(session);
  }, []);

  return {
    cancelDeviceAuthSession,
    closeDeviceAuthSession,
    continueDeviceAuthSession,
    deviceAuthSession,
    handleConnect,
    handleConnectFeishuManually,
    handleConnectFeishuWithQr,
    handleConnectWithCredential,
    handleDeleteOauthClient,
    handleDeviceConnected,
    handleDisconnect,
    handleSaveOauthClient,
    openFeishuWebAuthorizationUrl,
  };
}
