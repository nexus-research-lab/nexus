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
import { completeConnectorOAuthApi } from "@/lib/api/connector-api";
import {
  publishConnectorOauthEvent,
  type ConnectorOAuthEventType,
} from "@/features/capability/connectors/connector-oauth-events";

/** OAuth 回调专用页面，位于弹窗内，负责把结果回传给 opener 并自行关闭。 */
export function ConnectorOAuthCallbackPage() {
  const { pathname, search } = useLocation();
  const completedRef = useRef(false);
  const [message, setMessage] = useState("正在完成连接……");

  useEffect(() => {
    if (completedRef.current) {
      return;
    }
    completedRef.current = true;

    const params = new URLSearchParams(search);
    const code = params.get("code");
    const state = params.get("state");
    const error = params.get("error");
    const errorDescription = params.get("error_description");

    const closeCallbackWindow = (msg: string) => {
      setMessage(`${msg}，正在关闭窗口……`);
      window.setTimeout(() => {
        window.close();
      }, 120);
      window.setTimeout(() => {
        setMessage(`${msg}，可以手动关闭此窗口`);
      }, 800);
    };

    const postAndClose = (type: ConnectorOAuthEventType, msg: string) => {
      publishConnectorOauthEvent(type, msg);
      closeCallbackWindow(msg);
    };

    const returnToDesktop = (msg: string) => {
      setMessage(`${msg}，正在返回 Nexus……`);
      window.setTimeout(() => {
        window.location.href = getDesktopConnectorsReturnUri();
      }, 120);
      window.setTimeout(() => {
        setMessage(`${msg}，请返回 Nexus 或手动关闭此窗口`);
      }, 1_000);
    };

    const completeSuccess = async () => {
      if (isDesktopBridgeAvailable()) {
        try {
          await openDesktopRoute(AppRouteBuilders.connectors());
        } catch {
          // OAuth 已经完成，返回主窗口失败不应该阻止回调页关闭。
        }
      }
      publishConnectorOauthEvent("connector-oauth:success", "连接成功");
      if (isDesktopLoopbackOauthCallback()) {
        returnToDesktop("连接成功");
        return;
      }
      closeCallbackWindow("连接成功");
    };

    if (error) {
      postAndClose("connector-oauth:error", `OAuth 授权失败: ${errorDescription || error}`);
      return;
    }
    if (!code || !state) {
      postAndClose("connector-oauth:error", "OAuth 回调参数不完整");
      return;
    }

    completeConnectorOAuthApi(code, state, getConnectorOauthRedirectUri())
      .then(completeSuccess)
      .catch((err: unknown) => {
        const text = err instanceof Error ? err.message : "OAuth 连接失败";
        postAndClose("connector-oauth:error", text);
      });
  }, [pathname, search]);

  return (
    <div className="flex h-screen items-center justify-center text-sm text-muted-foreground">
      {message}
    </div>
  );
}
