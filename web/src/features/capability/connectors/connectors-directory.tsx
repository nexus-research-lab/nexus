"use client";

import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { useConnectorController } from "@/hooks/capability/use-connector-controller";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ConnectorDetail } from "@/types/capability/connector";

import {
  FeedbackBannerStack,
  type FeedbackBannerItem,
} from "@/shared/ui/feedback/feedback-banner-stack";
import { WORKSPACE_DETAIL_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { ConnectorDetailView } from "./connector-detail-view";
import { ConnectorCredentialDialog } from "./connector-credential-dialog";
import { ConnectorDeviceAuthDialog } from "./connector-device-auth-dialog";
import { ConnectorOAuthClientDialog } from "./connector-oauth-client-dialog";
import { ConnectorsGrid } from "./connectors-grid";
import { ConnectorsHeader } from "./connectors-header";
import { ConnectorsSearchBar } from "./connectors-search-bar";
import { subscribe_connector_oauth_event } from "./connector-oauth-events";

/* ── 连接器页面主编排组件 ────────────────────── */

export function ConnectorsDirectory() {
  const { t } = useI18n();
  const ctrl = useConnectorController();
  const navigate = useNavigate();
  const { connector_id: connectorId } = useParams<{ connector_id?: string }>();
  const [credentialDetail, setCredentialDetail] = useState<ConnectorDetail | null>(null);
  const [oauthClientDetail, setOauthClientDetail] = useState<ConnectorDetail | null>(null);
  const {
    close_detail: closeDetail,
    set_error_message: setErrorMessage,
    status_message: statusMessage,
    error_message: errorMessage,
    open_detail: openDetail,
    set_status_message: setStatusMessage,
    refresh,
  } = ctrl;

  useEffect(() => {
    if (!connectorId) {
      closeDetail();
      return;
    }

    void openDetail(connectorId);
  }, [closeDetail, connectorId, openDetail]);

  useEffect(() => {
    return subscribe_connector_oauth_event((event) => {
      if (event.type === "connector-oauth:success") {
        setStatusMessage(event.message || "连接成功");
        void refresh();
        if (connectorId) {
          void openDetail(connectorId);
        }
      }

      if (event.type === "connector-oauth:error") {
        setErrorMessage(event.message || "OAuth 连接失败");
        void refresh();
        if (connectorId) {
          void openDetail(connectorId);
        }
      }
    });
  }, [connectorId, openDetail, refresh, setErrorMessage, setStatusMessage]);

  const closeOauthClientDialog = useCallback(() => {
    setOauthClientDetail(null);
  }, []);

  const closeCredentialDialog = useCallback(() => {
    setCredentialDetail(null);
  }, []);

  const handleSaveCredential = useCallback(
    async (connectorId: string, credential: string) => {
      const saved = await ctrl.handle_connect_with_credential(connectorId, credential);
      if (saved) {
        setCredentialDetail(null);
      }
    },
    [ctrl],
  );

  const handleSaveOauthClient = useCallback(
    async (connectorId: string, clientId: string, clientSecret: string) => {
      const saved = await ctrl.handle_save_oauth_client(connectorId, clientId, clientSecret);
      if (saved) {
        setOauthClientDetail(null);
      }
    },
    [ctrl],
  );

  const handleDeleteOauthClient = useCallback(
    async (connectorId: string) => {
      const deleted = await ctrl.handle_delete_oauth_client(connectorId);
      if (deleted) {
        setOauthClientDetail(null);
      }
    },
    [ctrl],
  );

  const openConnectorPage = useCallback(
    (id: string) => {
      navigate(AppRouteBuilders.connector_detail(id));
    },
    [navigate],
  );

  const backToConnectors = useCallback(() => {
    navigate(AppRouteBuilders.connectors());
  }, [navigate]);

  const feedbackItems: FeedbackBannerItem[] = [];
  if (statusMessage) {
    feedbackItems.push({
      key: "status",
      message: statusMessage,
      on_dismiss: () => setStatusMessage(null),
      title: "操作完成",
      tone: "success",
    });
  }
  if (errorMessage) {
    feedbackItems.push({
      key: "error",
      message: errorMessage,
      on_dismiss: () => setErrorMessage(null),
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
        {connectorId ? (
          <ConnectorDetailView
            busy={ctrl.busy_id !== null}
            detail={ctrl.selected_detail}
            loading={ctrl.detail_loading}
            on_back={backToConnectors}
            on_connect={(id) => void ctrl.handle_connect(id)}
            on_configure_credential={setCredentialDetail}
            on_configure_oauth_client={setOauthClientDetail}
            on_disconnect={(id) => void ctrl.handle_disconnect(id)}
          />
        ) : (
          <div className={WORKSPACE_DETAIL_PAGE_CLASS_NAME}>
            <div className="mb-5">
              <h1 className="text-[24px] font-semibold tracking-[-0.03em] text-(--text-strong)">
                {t("capability.connectors_intro_title")}
              </h1>
              <p className="mt-1 max-w-[680px] text-[13px] leading-6 text-(--text-muted)">
                {t("capability.connectors_intro_description")}
              </p>
            </div>
            <ConnectorsSearchBar ctrl={ctrl} />
            <ConnectorsGrid ctrl={ctrl} on_open_connector={openConnectorPage} />
          </div>
        )}
      </WorkspaceSurfaceScaffold>

      <ConnectorOAuthClientDialog
        busy={ctrl.busy_id !== null}
        detail={oauthClientDetail}
        on_close={closeOauthClientDialog}
        on_delete={(id) => void handleDeleteOauthClient(id)}
        on_save={(id, clientId, clientSecret) => void handleSaveOauthClient(id, clientId, clientSecret)}
      />

      <ConnectorCredentialDialog
        busy={ctrl.busy_id !== null}
        detail={credentialDetail}
        on_close={closeCredentialDialog}
        on_save={(id, credential) => void handleSaveCredential(id, credential)}
      />

      <ConnectorDeviceAuthDialog
        session={ctrl.device_auth_session}
        on_close={ctrl.close_device_auth_session}
        on_error={ctrl.set_error_message}
        on_connected={async (id) => {
          ctrl.set_status_message("GitHub 已连接");
          await ctrl.refresh();
          navigate(AppRouteBuilders.connector_detail(id));
          await ctrl.open_detail(id);
        }}
      />

      {/* 操作反馈 */}
      <FeedbackBannerStack items={feedbackItems} />
    </>
  );
}
