"use client";

import { useEffect, useRef, useState } from "react";
import { useLocation } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { get_connector_oauth_redirect_uri } from "@/config/desktop-runtime";
import { is_desktop_bridge_available, open_desktop_route } from "@/lib/desktop-bridge";
import { complete_connector_o_auth_api } from "@/lib/api/connector-api";
import {
  publish_connector_oauth_event,
  type ConnectorOAuthEventType,
} from "@/features/capability/connectors/connector-oauth-events";

/** OAuth 回调专用页面，位于弹窗内，负责把结果回传给 opener 并自行关闭。 */
export function ConnectorOAuthCallbackPage() {
  const location = useLocation();
  const completed_ref = useRef(false);
  const [message, set_message] = useState("正在完成连接……");

  useEffect(() => {
    if (completed_ref.current) {
      return;
    }
    completed_ref.current = true;

    const params = new URLSearchParams(location.search);
    const code = params.get("code");
    const state = params.get("state");
    const error = params.get("error");
    const error_description = params.get("error_description");

    const close_callback_window = (msg: string) => {
      set_message(`${msg}，正在关闭窗口……`);
      window.setTimeout(() => {
        window.close();
      }, 120);
      window.setTimeout(() => {
        set_message(`${msg}，可以手动关闭此窗口`);
      }, 800);
    };

    const post_and_close = (type: ConnectorOAuthEventType, msg: string) => {
      publish_connector_oauth_event(type, msg);
      close_callback_window(msg);
    };

    const complete_success = async () => {
      if (is_desktop_bridge_available()) {
        try {
          await open_desktop_route(AppRouteBuilders.connectors());
        } catch {
          // OAuth 已经完成，返回主窗口失败不应该阻止回调页关闭。
        }
      }
      post_and_close("connector-oauth:success", "连接成功");
    };

    if (error) {
      post_and_close("connector-oauth:error", `OAuth 授权失败: ${error_description || error}`);
      return;
    }
    if (!code || !state) {
      post_and_close("connector-oauth:error", "OAuth 回调参数不完整");
      return;
    }

    complete_connector_o_auth_api(code, state, get_connector_oauth_redirect_uri())
      .then(complete_success)
      .catch((err: unknown) => {
        const text = err instanceof Error ? err.message : "OAuth 连接失败";
        post_and_close("connector-oauth:error", text);
      });
  }, [location.pathname, location.search]);

  return (
    <div className="flex h-screen items-center justify-center text-sm text-muted-foreground">
      {message}
    </div>
  );
}
