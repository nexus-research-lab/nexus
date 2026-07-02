"use client";

import { useEffect, useRef, useState } from "react";
import { useLocation } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import {
  get_connector_oauth_redirect_uri,
  get_desktop_connectors_return_uri,
  is_desktop_loopback_oauth_callback,
} from "@/config/desktop-runtime";
import { is_desktop_bridge_available, open_desktop_route } from "@/lib/desktop-bridge";
import { complete_connector_o_auth_api } from "@/lib/api/connector-api";
import {
  publish_connector_oauth_event,
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
      publish_connector_oauth_event(type, msg);
      closeCallbackWindow(msg);
    };

    const returnToDesktop = (msg: string) => {
      setMessage(`${msg}，正在返回 Nexus……`);
      window.setTimeout(() => {
        window.location.href = get_desktop_connectors_return_uri();
      }, 120);
      window.setTimeout(() => {
        setMessage(`${msg}，请返回 Nexus 或手动关闭此窗口`);
      }, 1_000);
    };

    const completeSuccess = async () => {
      if (is_desktop_bridge_available()) {
        try {
          await open_desktop_route(AppRouteBuilders.connectors());
        } catch {
          // OAuth 已经完成，返回主窗口失败不应该阻止回调页关闭。
        }
      }
      publish_connector_oauth_event("connector-oauth:success", "连接成功");
      if (is_desktop_loopback_oauth_callback()) {
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

    complete_connector_o_auth_api(code, state, get_connector_oauth_redirect_uri())
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
