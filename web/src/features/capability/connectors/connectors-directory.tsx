"use client";

import { useEffect, useRef } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { useConnectorController } from "@/hooks/use-connector-controller";

import { FeedbackBanner } from "@/features/capability/skills/feedback-banner";

import { ConnectorDetailDialog } from "./connector-detail-dialog";
import { ConnectorsGrid } from "./connectors-grid";
import { ConnectorsHeader } from "./connectors-header";
import { ConnectorsSearchBar } from "./connectors-search-bar";

/* ── 连接器页面主编排组件 ────────────────────── */

export function ConnectorsDirectory() {
  const ctrl = useConnectorController();
  const location = useLocation();
  const navigate = useNavigate();
  const handled_callback_ref = useRef<string | null>(null);
  const {
    handle_oauth_callback,
    set_error_message,
    status_message,
    error_message,
    set_status_message,
  } = ctrl;

  useEffect(() => {
    const handle_message = (event: MessageEvent) => {
      if (event.origin !== window.location.origin) {
        return;
      }
      const data = event.data as
        | { type?: string; message?: string }
        | undefined;
      if (!data?.type?.startsWith("connector-oauth:")) {
        return;
      }

      if (data.type === "connector-oauth:success") {
        set_status_message(data.message || "连接成功");
        void ctrl.refresh();
      }

      if (data.type === "connector-oauth:error") {
        set_error_message(data.message || "OAuth 连接失败");
        void ctrl.refresh();
      }
    };

    window.addEventListener("message", handle_message);
    return () => {
      window.removeEventListener("message", handle_message);
    };
  }, [ctrl, set_error_message, set_status_message]);

  useEffect(() => {
    const params = new URLSearchParams(location.search);
    const code = params.get("code");
    const state = params.get("state");
    const error = params.get("error");
    const callback_key = `${code || ""}:${state || ""}:${error || ""}`;
    if (!code && !state && !error) {
      handled_callback_ref.current = null;
      return;
    }
    if (handled_callback_ref.current === callback_key) {
      return;
    }
    handled_callback_ref.current = callback_key;

    if (error) {
      const error_message = `OAuth 授权失败: ${error}`;
      if (window.opener && !window.opener.closed) {
        window.opener.postMessage(
          { type: "connector-oauth:error", message: error_message },
          window.location.origin,
        );
        window.close();
        return;
      }
      set_error_message(error_message);
      navigate(location.pathname, { replace: true });
      return;
    }

    if (!code || !state) {
      const error_message = "OAuth 回调参数不完整";
      if (window.opener && !window.opener.closed) {
        window.opener.postMessage(
          { type: "connector-oauth:error", message: error_message },
          window.location.origin,
        );
        window.close();
        return;
      }
      set_error_message(error_message);
      navigate(location.pathname, { replace: true });
      return;
    }

    void handle_oauth_callback({
      code,
      state,
      redirect_uri: `${window.location.origin}${location.pathname}`,
    })
      .then(() => {
        if (window.opener && !window.opener.closed) {
          window.opener.postMessage(
            { type: "connector-oauth:success", message: "连接成功" },
            window.location.origin,
          );
          window.close();
          return;
        }
      })
      .catch((err: unknown) => {
        const error_message = err instanceof Error ? err.message : "OAuth 连接失败";
        if (window.opener && !window.opener.closed) {
          window.opener.postMessage(
            { type: "connector-oauth:error", message: error_message },
            window.location.origin,
          );
          window.close();
          return;
        }
      })
      .finally(() => {
        if (window.opener && !window.opener.closed) {
          return;
        }
        navigate(location.pathname, { replace: true });
    });
  }, [handle_oauth_callback, location.pathname, location.search, navigate, set_error_message]);

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <ConnectorsHeader ctrl={ctrl} />

      {/* 内容区 */}
      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-5 py-5 xl:px-6">
        <ConnectorsSearchBar ctrl={ctrl} />
        <ConnectorsGrid ctrl={ctrl} />
      </div>

      {/* 详情弹窗 */}
      <ConnectorDetailDialog
        busy={ctrl.busy_id !== null}
        detail={ctrl.selected_detail}
        loading={ctrl.detail_loading}
        on_close={ctrl.close_detail}
        on_connect={(id) => void ctrl.handle_connect(id)}
        on_disconnect={(id) => void ctrl.handle_disconnect(id)}
      />

      {/* 操作反馈 */}
      {(status_message || error_message) && (
        <div className="pointer-events-none fixed right-6 top-24 z-40 flex flex-col gap-2">
          {status_message && (
            <FeedbackBanner
              message={status_message}
              on_dismiss={() => set_status_message(null)}
              title="操作完成"
              tone="success"
            />
          )}
          {error_message && (
            <FeedbackBanner
              message={error_message}
              on_dismiss={() => set_error_message(null)}
              title="操作失败"
              tone="error"
            />
          )}
        </div>
      )}
    </div>
  );
}
