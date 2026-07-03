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
import { subscribeConnectorOauthEvent } from "./connector-oauth-events";

/* ── 连接器页面主编排组件 ────────────────────── */

export function ConnectorsDirectory() {
  const { t } = useI18n();
  const ctrl = useConnectorController();
  const navigate = useNavigate();
  const { connector_id: connectorId } = useParams<{ connector_id?: string }>();
  const [credentialDetail, setCredentialDetail] = useState<ConnectorDetail | null>(null);
  const [oauthClientDetail, setOauthClientDetail] = useState<ConnectorDetail | null>(null);
  const {
    closeDetail,
    setErrorMessage,
    statusMessage,
    errorMessage,
    openDetail,
    setStatusMessage,
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
    return subscribeConnectorOauthEvent((event) => {
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
      const saved = await ctrl.handleConnectWithCredential(connectorId, credential);
      if (saved) {
        setCredentialDetail(null);
      }
    },
    [ctrl],
  );

  const handleSaveOauthClient = useCallback(
    async (connectorId: string, clientId: string, clientSecret: string) => {
      const saved = await ctrl.handleSaveOauthClient(connectorId, clientId, clientSecret);
      if (saved) {
        setOauthClientDetail(null);
      }
    },
    [ctrl],
  );

  const handleDeleteOauthClient = useCallback(
    async (connectorId: string) => {
      const deleted = await ctrl.handleDeleteOauthClient(connectorId);
      if (deleted) {
        setOauthClientDetail(null);
      }
    },
    [ctrl],
  );

  const openConnectorPage = useCallback(
    (id: string) => {
      navigate(AppRouteBuilders.connectorDetail(id));
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
      onDismiss: () => setStatusMessage(null),
      title: "操作完成",
      tone: "success",
    });
  }
  if (errorMessage) {
    feedbackItems.push({
      key: "error",
      message: errorMessage,
      onDismiss: () => setErrorMessage(null),
      title: "操作失败",
      tone: "error",
    });
  }

  return (
    <>
      <WorkspaceSurfaceScaffold
        bodyScrollable
        header={<ConnectorsHeader ctrl={ctrl} />}
        stableGutter
      >
        {connectorId ? (
          <ConnectorDetailView
            busy={ctrl.busyId !== null}
            detail={ctrl.selectedDetail}
            loading={ctrl.detailLoading}
            onBack={backToConnectors}
            onConnect={(id) => void ctrl.handleConnect(id)}
            onConfigureCredential={setCredentialDetail}
            onConfigureOauthClient={setOauthClientDetail}
            onDisconnect={(id) => void ctrl.handleDisconnect(id)}
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
            <ConnectorsGrid ctrl={ctrl} onOpenConnector={openConnectorPage} />
          </div>
        )}
      </WorkspaceSurfaceScaffold>

      <ConnectorOAuthClientDialog
        busy={ctrl.busyId !== null}
        detail={oauthClientDetail}
        onClose={closeOauthClientDialog}
        onDelete={(id) => void handleDeleteOauthClient(id)}
        onSave={(id, clientId, clientSecret) => void handleSaveOauthClient(id, clientId, clientSecret)}
      />

      <ConnectorCredentialDialog
        busy={ctrl.busyId !== null}
        detail={credentialDetail}
        onClose={closeCredentialDialog}
        onSave={(id, credential) => void handleSaveCredential(id, credential)}
      />

      <ConnectorDeviceAuthDialog
        session={ctrl.deviceAuthSession}
        onClose={ctrl.closeDeviceAuthSession}
        onError={ctrl.setErrorMessage}
        onConnected={async (id) => {
          ctrl.setStatusMessage("GitHub 已连接");
          await ctrl.refresh();
          navigate(AppRouteBuilders.connectorDetail(id));
          await ctrl.openDetail(id);
        }}
      />

      {/* 操作反馈 */}
      <FeedbackBannerStack items={feedbackItems} />
    </>
  );
}
