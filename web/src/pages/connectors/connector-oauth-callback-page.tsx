"use client";

import { useEffect, useRef, useState } from "react";
import { useLocation } from "react-router-dom";

import { complete_connector_o_auth_api } from "@/lib/api/connector-api";

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

    const post_and_close = (
      type: "connector-oauth:success" | "connector-oauth:error",
      msg: string,
    ) => {
      if (window.opener && !window.opener.closed) {
        window.opener.postMessage({ type, message: msg }, window.location.origin);
        window.close();
        return;
      }
      set_message(msg);
    };

    if (error) {
      post_and_close("connector-oauth:error", `OAuth 授权失败: ${error_description || error}`);
      return;
    }
    if (!code || !state) {
      post_and_close("connector-oauth:error", "OAuth 回调参数不完整");
      return;
    }

    complete_connector_o_auth_api(code, state, `${window.location.origin}${location.pathname}`)
      .then(() => post_and_close("connector-oauth:success", "连接成功"))
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
