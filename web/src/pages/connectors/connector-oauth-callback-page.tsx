// INPUT: OAuth provider 回调参数与服务端完成连接的 FailureCore。
// OUTPUT: 不含秘密/Provider 正文的回调页三问状态和同源受控事件。
// POS: 浏览器 OAuth 回调入口；code/state 只提交服务端，不进入展示或日志。
"use client";

import { useEffect, useRef, useState } from "react";
import { useLocation } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import {
  getConnectorOauthRedirectUri,
  getDesktopConnectorsReturnUri,
  isDesktopLoopbackOauthCallback,
} from "@/config/desktop-runtime";
import { isDesktopBridgeAvailable, openDesktopRoute } from "@/lib/desktop-bridge";
import { completeConnectorOAuthApi } from "@/lib/api/capability/connector-api";
import { projectMutationFailure } from "@/lib/error-message";
import {
  clearPendingConnectorOauth,
  publishConnectorOauthEvent,
  readPendingConnectorOauth,
  type ConnectorOAuthFailureKind,
  type ConnectorOAuthEventType,
} from "@/features/capability/connectors/auth/connector-oauth-events";

interface OAuthCallbackStatus {
  impact?: string;
  message: string;
  nextStep?: string;
  title: string;
}

const INITIAL_STATUS: OAuthCallbackStatus = {
  message: "Nexus 正在核对这次授权。",
  title: "正在完成连接",
};

/** OAuth 回调专用页面，只显示受控结果，不回显 Provider 或 HTTP 异常正文。 */
export function ConnectorOAuthCallbackPage() {
  const { pathname, search } = useLocation();
  const completedRef = useRef(false);
  const [status, setStatus] = useState<OAuthCallbackStatus>(INITIAL_STATUS);
  const [closingHint, setClosingHint] = useState("");

  useEffect(() => {
    if (completedRef.current) {
      return;
    }
    completedRef.current = true;

    const params = new URLSearchParams(search);
    const code = params.get("code");
    const state = params.get("state");
    const error = params.get("error");
    const pendingConnectorId = readPendingConnectorOauth();

    const closeCallbackWindow = (nextStatus: OAuthCallbackStatus) => {
      setStatus(nextStatus);
      setClosingHint("正在关闭窗口……");
      window.setTimeout(() => {
        window.close();
      }, 120);
      window.setTimeout(() => {
        setClosingHint("可以手动关闭此窗口。");
      }, 800);
    };

    const postAndClose = (
      type: ConnectorOAuthEventType,
      nextStatus: OAuthCallbackStatus,
      failureKind?: ConnectorOAuthFailureKind,
      connectorId: string | null = pendingConnectorId,
    ) => {
      publishConnectorOauthEvent(type, nextStatus.message, {
        connectorId,
        failureKind,
      });
      if (failureKind !== "outcome_unknown") {
        clearPendingConnectorOauth(connectorId);
      }
      closeCallbackWindow(nextStatus);
    };

    const returnToDesktop = (nextStatus: OAuthCallbackStatus) => {
      setStatus(nextStatus);
      setClosingHint("正在返回 Nexus……");
      window.setTimeout(() => {
        window.location.href = getDesktopConnectorsReturnUri();
      }, 120);
      window.setTimeout(() => {
        setClosingHint("请返回 Nexus，或手动关闭此窗口。");
      }, 1_000);
    };

    const completeSuccess = async (connectorId: string) => {
      if (isDesktopBridgeAvailable()) {
        try {
          await openDesktopRoute(AppRouteBuilders.connectors());
        } catch {
          // OAuth 已经完成，返回主窗口失败不应该阻止回调页关闭。
        }
      }
      const successStatus: OAuthCallbackStatus = {
        message: "授权信息已安全返回 Nexus。",
        title: "连接已完成",
      };
      publishConnectorOauthEvent(
        "connector-oauth:success",
        successStatus.message,
        { connectorId },
      );
      clearPendingConnectorOauth(connectorId);
      if (isDesktopLoopbackOauthCallback()) {
        returnToDesktop(successStatus);
        return;
      }
      closeCallbackWindow(successStatus);
    };

    if (error) {
      postAndClose(
        "connector-oauth:error",
        {
          impact: "没有保存新的连接；已有连接和应用配置没有被删除。",
          message: error === "access_denied"
            ? "你取消了授权，或服务拒绝了这次授权。"
            : "授权服务没有完成这次连接。",
          nextStep: "返回 Nexus 检查连接器设置；需要时可以重新开始授权。",
          title: "连接没有完成",
        },
        "not_connected",
      );
      return;
    }
    if (!code || !state) {
      postAndClose(
        "connector-oauth:error",
        {
          impact: "没有提交连接信息；已有连接和应用配置保持不变。",
          message: "授权服务返回的信息不完整，Nexus 无法完成连接。",
          nextStep: "返回 Nexus 检查连接器设置后，重新开始授权。",
          title: "连接没有完成",
        },
        "not_connected",
      );
      return;
    }

    completeConnectorOAuthApi(code, state, getConnectorOauthRedirectUri())
      .then((connector) => completeSuccess(connector.connector_id))
      .catch((err: unknown) => {
        const failure = projectMutationFailure(
          err,
          "Nexus 没有收到连接完成的确认。",
        );
        const notConnected = failure.effect === "not_applied";
        postAndClose(
          "connector-oauth:error",
          notConnected
            ? {
                impact: "这次连接没有保存；已有连接和应用配置保持不变。",
                message: failure.message,
                nextStep: "返回 Nexus 检查连接器设置后，可以重新开始授权。",
                title: "连接没有完成",
              }
            : {
                impact: "新的连接可能已经保存，也可能尚未完成；已有连接和应用配置没有被删除。",
                message: failure.message,
                nextStep: "返回 Nexus 并重新加载连接器状态；确认结果前不要再次授权。",
                title: "连接结果待确认",
              },
          notConnected ? "not_connected" : "outcome_unknown",
        );
      });
  }, [pathname, search]);

  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
      <section
        aria-atomic="true"
        className="surface-panel surface-radius-xl w-full max-w-[480px] border px-8 py-9"
        role={status.impact ? "alert" : "status"}
      >
        <h1 className="text-lg font-semibold text-(--text-strong)">
          {status.title}
        </h1>
        <p className="mt-2 text-sm leading-6 text-(--text-muted)">
          {status.message}
        </p>
        {status.impact ? (
          <p className="mt-3 text-sm leading-6 text-(--text-muted)">
            {status.impact}
          </p>
        ) : null}
        {status.nextStep ? (
          <p className="mt-2 text-sm font-medium leading-6 text-(--text-default)">
            {status.nextStep}
          </p>
        ) : null}
        {closingHint ? (
          <p className="mt-4 text-xs text-(--text-soft)">{closingHint}</p>
        ) : null}
      </section>
    </main>
  );
}
