"use client";

import { useCallback, useEffect, useState } from "react";

import { useConnectorController } from "@/hooks/capability/use-connector-controller";
import type { ConnectorDetail } from "@/types/capability/connector";

import {
  FeedbackBannerStack,
  type FeedbackBannerItem,
} from "@/shared/ui/feedback/feedback-banner-stack";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { ConnectorDetailDialog } from "./connector-detail-dialog";
import { ConnectorDeviceAuthDialog } from "./connector-device-auth-dialog";
import { ConnectorOAuthClientDialog } from "./connector-oauth-client-dialog";
import { ConnectorsGrid } from "./connectors-grid";
import { ConnectorsHeader } from "./connectors-header";
import { ConnectorsSearchBar } from "./connectors-search-bar";
import { subscribe_connector_oauth_event } from "./connector-oauth-events";

/* ── 连接器页面主编排组件 ────────────────────── */

export function ConnectorsDirectory() {
  const ctrl = useConnectorController();
  const [oauth_client_detail, set_oauth_client_detail] = useState<ConnectorDetail | null>(null);
  const {
    set_error_message,
    status_message,
    error_message,
    set_status_message,
    refresh,
  } = ctrl;

  useEffect(() => {
    return subscribe_connector_oauth_event((event) => {
      if (event.type === "connector-oauth:success") {
        set_status_message(event.message || "连接成功");
        void refresh();
      }

      if (event.type === "connector-oauth:error") {
        set_error_message(event.message || "OAuth 连接失败");
        void refresh();
      }
    });
  }, [refresh, set_error_message, set_status_message]);

  const close_oauth_client_dialog = useCallback(() => {
    set_oauth_client_detail(null);
  }, []);

  const handle_save_oauth_client = useCallback(
    async (connector_id: string, client_id: string, client_secret: string) => {
      const saved = await ctrl.handle_save_oauth_client(connector_id, client_id, client_secret);
      if (saved) {
        set_oauth_client_detail(null);
      }
    },
    [ctrl],
  );

  const handle_delete_oauth_client = useCallback(
    async (connector_id: string) => {
      const deleted = await ctrl.handle_delete_oauth_client(connector_id);
      if (deleted) {
        set_oauth_client_detail(null);
      }
    },
    [ctrl],
  );

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
        body_scrollable
        header={<ConnectorsHeader ctrl={ctrl} />}
        stable_gutter
      >
        <div className="mx-auto w-full max-w-[980px] px-5 py-6 xl:px-6">
          <div className="mb-5">
            <h1 className="text-[24px] font-semibold tracking-[-0.03em] text-(--text-strong)">
              让 Nexus 接入你的工作工具
            </h1>
            <p className="mt-1 max-w-[680px] text-[13px] leading-6 text-(--text-muted)">
              连接文档、代码仓库和业务系统，让 Agent 在对话里读取、创建和更新真实工作内容。
            </p>
          </div>
          <ConnectorsSearchBar ctrl={ctrl} />
          <ConnectorsGrid ctrl={ctrl} />
        </div>
      </WorkspaceSurfaceScaffold>

      {/* 详情弹窗 */}
      <ConnectorDetailDialog
        busy={ctrl.busy_id !== null}
        detail={ctrl.selected_detail}
        loading={ctrl.detail_loading}
        on_close={ctrl.close_detail}
        on_connect={(id) => void ctrl.handle_connect(id)}
        on_configure_oauth_client={set_oauth_client_detail}
        on_disconnect={(id) => void ctrl.handle_disconnect(id)}
      />

      <ConnectorOAuthClientDialog
        busy={ctrl.busy_id !== null}
        detail={oauth_client_detail}
        on_close={close_oauth_client_dialog}
        on_delete={(id) => void handle_delete_oauth_client(id)}
        on_save={(id, client_id, client_secret) => void handle_save_oauth_client(id, client_id, client_secret)}
      />

      <ConnectorDeviceAuthDialog
        session={ctrl.device_auth_session}
        on_close={ctrl.close_device_auth_session}
        on_error={ctrl.set_error_message}
        on_connected={async (id) => {
          ctrl.set_status_message("GitHub 已连接");
          await ctrl.refresh();
          await ctrl.open_detail(id);
        }}
      />

      {/* 操作反馈 */}
      <FeedbackBannerStack items={feedback_items} />
    </>
  );
}
