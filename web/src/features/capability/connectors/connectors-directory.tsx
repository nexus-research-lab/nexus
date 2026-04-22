"use client";

import { useEffect } from "react";

import { useConnectorController } from "@/hooks/capability/use-connector-controller";

import {
  FeedbackBannerStack,
  type FeedbackBannerItem,
} from "@/shared/ui/feedback/feedback-banner-stack";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { ConnectorDetailDialog } from "./connector-detail-dialog";
import { ConnectorsGrid } from "./connectors-grid";
import { ConnectorsHeader } from "./connectors-header";
import { ConnectorsSearchBar } from "./connectors-search-bar";

/* ── 连接器页面主编排组件 ────────────────────── */

export function ConnectorsDirectory() {
  const ctrl = useConnectorController();
  const {
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

  const feedback_items: FeedbackBannerItem[] = [];
  if (status_message) {
    feedback_items.push({
      key: "status",
      message: status_message,
      on_dismiss: () => set_status_message(null),
      title: "操作完成",
      tone: "success",
    });
  }
  if (error_message) {
    feedback_items.push({
      key: "error",
      message: error_message,
      on_dismiss: () => set_error_message(null),
      title: "操作失败",
      tone: "error",
    });
  }

  return (
    <>
      <WorkspaceSurfaceScaffold
        body_class_name="px-5 py-5 xl:px-6"
        body_scrollable
        header={<ConnectorsHeader ctrl={ctrl} />}
        stable_gutter
      >
        <ConnectorsSearchBar ctrl={ctrl} />
        <ConnectorsGrid ctrl={ctrl} />
      </WorkspaceSurfaceScaffold>

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
      <FeedbackBannerStack items={feedback_items} />
    </>
  );
}
