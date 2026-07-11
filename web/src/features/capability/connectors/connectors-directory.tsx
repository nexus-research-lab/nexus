"use client";

import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { useI18n } from "@/shared/i18n/i18n-context";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { WORKSPACE_DETAIL_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { ConnectorsRouteParams } from "@/types/app/route";
import type { ConnectorDetail } from "@/types/capability/connector";

import { ConnectorCredentialDialog } from "./auth/connector-credential-dialog";
import { ConnectorDeviceAuthDialog } from "./auth/connector-device-auth-dialog";
import { ConnectorOAuthClientDialog } from "./auth/connector-oauth-client-dialog";
import { ConnectorsGrid } from "./catalog/connectors-grid";
import { ConnectorsHeader } from "./catalog/connectors-header";
import { ConnectorsSearchBar } from "./catalog/connectors-search-bar";
import { useConnectorController } from "./controller/use-connector-controller";
import { useConnectorOauthEvents } from "./controller/use-connector-oauth-events";
import { ConnectorDetailView } from "./detail/connector-detail-view";

type ConnectorConfigDialog = {
  detail: ConnectorDetail;
  kind: "credential" | "oauth-client";
} | null;

export function ConnectorsDirectory() {
  const { t } = useI18n();
  const controller = useConnectorController();
  const navigate = useNavigate();
  const { connectorId } = useParams<ConnectorsRouteParams>();
  const [configDialog, setConfigDialog] =
    useState<ConnectorConfigDialog>(null);
  const {
    clearFeedback,
    closeDetail,
    handleConnect,
    handleConnectWithCredential,
    handleDeleteOauthClient: deleteOauthClient,
    handleDeviceConnected,
    handleDisconnect,
    handleSaveOauthClient: saveOauthClient,
    openDetail,
    refreshCatalog,
    reportFeedback,
  } = controller;

  useEffect(() => {
    if (!connectorId) {
      closeDetail();
      return;
    }
    void openDetail(connectorId);
  }, [closeDetail, connectorId, openDetail]);

  useConnectorOauthEvents({
    connectorId,
    openDetail,
    refreshCatalog,
    reportFeedback,
  });

  const openConnectorPage = useCallback((id: string) => {
    navigate(AppRouteBuilders.connectorDetail(id));
  }, [navigate]);
  const backToConnectors = useCallback(() => {
    navigate(AppRouteBuilders.connectors());
  }, [navigate]);
  const closeConfigDialog = useCallback(() => setConfigDialog(null), []);

  const handleSaveCredential = useCallback(async (
    id: string,
    credential: string,
  ) => {
    if (await handleConnectWithCredential(id, credential)) {
      closeConfigDialog();
    }
  }, [closeConfigDialog, handleConnectWithCredential]);

  const handleSaveOauthClient = useCallback(async (
    id: string,
    clientId: string,
    clientSecret: string,
  ) => {
    if (await saveOauthClient(id, clientId, clientSecret)) {
      closeConfigDialog();
    }
  }, [closeConfigDialog, saveOauthClient]);

  const handleDeleteOauthClient = useCallback(async (id: string) => {
    if (await deleteOauthClient(id)) {
      closeConfigDialog();
    }
  }, [closeConfigDialog, deleteOauthClient]);

  const busy = controller.pendingAction !== null;
  const credentialDetail = configDialog?.kind === "credential"
    ? configDialog.detail
    : null;
  const oauthClientDetail = configDialog?.kind === "oauth-client"
    ? configDialog.detail
    : null;

  return (
    <>
      <WorkspaceSurfaceScaffold
        bodyScrollable
        header={(
          <ConnectorsHeader connectedCount={controller.connectedCount} />
        )}
        stableGutter
      >
        {connectorId ? (
          <ConnectorDetailView
            busy={busy}
            detail={controller.selectedDetail}
            loading={controller.detailLoading}
            onBack={backToConnectors}
            onConfigureCredential={(detail) => setConfigDialog({
              detail,
              kind: "credential",
            })}
            onConfigureOauthClient={(detail) => setConfigDialog({
              detail,
              kind: "oauth-client",
            })}
            onConnect={(id) => void handleConnect(id)}
            onDisconnect={(id) => void handleDisconnect(id)}
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
            <ConnectorsSearchBar
              activeCategory={controller.activeCategory}
              onCategoryChange={controller.setActiveCategory}
              onQueryChange={controller.setSearchQuery}
              searchQuery={controller.searchQuery}
            />
            <ConnectorsGrid
              activeCategory={controller.activeCategory}
              connectors={controller.connectors}
              loading={controller.loading}
              onConnect={(id) => void handleConnect(id)}
              onOpenConnector={openConnectorPage}
              pendingAction={controller.pendingAction}
              searchQuery={controller.searchQuery}
            />
          </div>
        )}
      </WorkspaceSurfaceScaffold>

      <ConnectorOAuthClientDialog
        busy={busy}
        detail={oauthClientDetail}
        onClose={closeConfigDialog}
        onDelete={(id) => void handleDeleteOauthClient(id)}
        onSave={(id, clientId, clientSecret) => {
          void handleSaveOauthClient(id, clientId, clientSecret);
        }}
      />
      <ConnectorCredentialDialog
        busy={busy}
        detail={credentialDetail}
        onClose={closeConfigDialog}
        onSave={(id, credential) => {
          void handleSaveCredential(id, credential);
        }}
      />
      <ConnectorDeviceAuthDialog
        onClose={controller.closeDeviceAuthSession}
        onConnected={async (id) => {
          await handleDeviceConnected();
          navigate(AppRouteBuilders.connectorDetail(id));
          await openDetail(id);
        }}
        onError={(message) => reportFeedback({
          tone: "error",
          title: "操作失败",
          message,
        })}
        session={controller.deviceAuthSession}
      />
      <FeedbackBannerViewport
        item={controller.feedback ? {
          message: controller.feedback.message,
          onDismiss: clearFeedback,
          title: controller.feedback.title,
          tone: controller.feedback.tone,
        } : null}
      />
    </>
  );
}
