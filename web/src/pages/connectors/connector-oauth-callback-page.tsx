// INPUT: OAuth provider 回调参数与服务端完成连接的 FailureCore。
// OUTPUT: 不含秘密/Provider 正文的回调页三问状态和同源受控事件。
// POS: 浏览器 OAuth 回调入口；code/state 只提交服务端，不进入展示或日志。
"use client";

import { useEffect, useRef, useState } from "react";
import { useLocation } from "react-router-dom";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
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

type OAuthCallbackStatus =
  | { impact: TranslationKey; message?: never; title: TranslationKey }
  | { impact?: never; message: TranslationKey; title: TranslationKey };

const INITIAL_STATUS: OAuthCallbackStatus = {
  message: "capability.oauth_callback.checking_message",
  title: "capability.oauth_callback.checking_title",
};

/** OAuth 回调专用页面，只显示受控结果，不回显 Provider 或 HTTP 异常正文。 */
export function ConnectorOAuthCallbackPage() {
  const { pathname, search } = useLocation();
  const { t } = useI18n();
  const completedRef = useRef(false);
  const [status, setStatus] = useState<OAuthCallbackStatus>(INITIAL_STATUS);
  const [closingHint, setClosingHint] = useState<TranslationKey | null>(null);

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
      setClosingHint("capability.oauth_callback.closing");
      window.setTimeout(() => {
        window.close();
      }, 120);
      window.setTimeout(() => {
        setClosingHint("capability.oauth_callback.manual_close");
      }, 800);
    };

    const postAndClose = (
      type: ConnectorOAuthEventType,
      nextStatus: OAuthCallbackStatus,
      failureKind?: ConnectorOAuthFailureKind,
      connectorId: string | null = pendingConnectorId,
    ) => {
      publishConnectorOauthEvent(
        type,
        t(nextStatus.message ?? nextStatus.title),
        {
          connectorId,
          failureKind,
        },
      );
      if (failureKind !== "outcome_unknown") {
        clearPendingConnectorOauth(connectorId);
      }
      closeCallbackWindow(nextStatus);
    };

    const returnToDesktop = (nextStatus: OAuthCallbackStatus) => {
      setStatus(nextStatus);
      setClosingHint("capability.oauth_callback.returning");
      window.setTimeout(() => {
        window.location.href = getDesktopConnectorsReturnUri();
      }, 120);
      window.setTimeout(() => {
        setClosingHint("capability.oauth_callback.manual_return");
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
        message: "capability.oauth_callback.success_message",
        title: "capability.oauth_callback.success_title",
      };
      publishConnectorOauthEvent(
        "connector-oauth:success",
        t(successStatus.message),
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
          impact: "capability.oauth_callback.cancel_impact",
          title: error === "access_denied" ? "capability.oauth_callback.cancel_title" : "capability.oauth_callback.failed_title",
        },
        "not_connected",
      );
      return;
    }
    if (!code || !state) {
      postAndClose(
        "connector-oauth:error",
        {
          impact: "capability.oauth_callback.incomplete_impact",
          title: "capability.oauth_callback.incomplete_title",
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
          t("capability.oauth_callback.unknown_fallback"),
        );
        const notConnected = failure.effect === "not_applied";
        postAndClose(
          "connector-oauth:error",
          notConnected
            ? {
                impact: "capability.oauth_callback.failed_impact",
                title: "capability.oauth_callback.failed_title",
              }
            : {
                impact: "capability.oauth_callback.unknown_impact",
                title: "capability.oauth_callback.unknown_title",
              },
          notConnected ? "not_connected" : "outcome_unknown",
        );
      });
  }, [pathname, search, t]);

  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
      <section
        aria-atomic="true"
        className="surface-panel surface-radius-xl w-full max-w-[480px] border px-8 py-9"
        role={status.impact ? "alert" : "status"}
      >
        <h1 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
          {t(status.title)}
        </h1>
        {"message" in status ? (
          <p className={cn("mt-2", getUiTypographyClassName({ role: "supporting", tone: "muted" }))}>
            {status.message ? t(status.message) : null}
          </p>
        ) : null}
        {status.impact ? (
          <p className={cn("mt-3", getUiTypographyClassName({ role: "supporting", tone: "muted" }))}>
            {t(status.impact)}
          </p>
        ) : null}
        {closingHint ? (
          <p className={cn("mt-4", getUiTypographyClassName({ role: "metadata", tone: "soft" }))}>{t(closingHint)}</p>
        ) : null}
      </section>
    </main>
  );
}
