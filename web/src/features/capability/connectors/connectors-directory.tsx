/**
 * INPUT: Connector/自定义 MCP 目录、详情路由与认证命令。
 * OUTPUT: 低密度连接器目录、配置入口或详情页面。
 * POS: “能力 > 连接器”的唯一页面装配入口。
 */
"use client";

import { Plus } from "lucide-react";
import {
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { CapabilityPageLayout } from "@/features/capability/shared/capability-page-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { completeFeedbackBanner } from "@/shared/ui/feedback/feedback-banner-contract";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { ConnectorsRouteParams } from "@/types/app/route";
import type { ConnectorDetail } from "@/types/capability/connector";

import { ConnectorCredentialDialog } from "./auth/connector-credential-dialog";
import { ConnectorDeviceAuthDialog } from "./auth/device-flow/connector-device-auth-dialog";
import { FeishuAppConnectionDialog } from "./auth/feishu/feishu-app-connection-dialog";
import { RichMailPairingDialog } from "./auth/richmail/richmail-pairing-dialog";
import { ConnectorOAuthClientDialog } from "./auth/connector-oauth-client-dialog";
import { ShopDomainPromptDialog } from "./auth/shop-domain/shop-domain-prompt-dialog";
import { ConnectorsGrid } from "./catalog/connectors-grid";
import {
  type ConnectorDirectoryMode,
  ConnectorsSearchBar,
} from "./catalog/connectors-search-bar";
import { useConnectorController } from "./controller/use-connector-controller";
import { useConnectorOauthEvents } from "./controller/use-connector-oauth-events";
import { CustomMCPDialog } from "./custom/custom-mcp-dialog";
import { CustomMCPDetailView } from "./custom/detail/custom-mcp-detail-view";
import { useCustomMCPTools } from "./custom/detail/use-custom-mcp-tools";
import { CustomMCPGrid } from "./custom/custom-mcp-grid";
import {
  filterCustomMCPServers,
  getCustomMCPDisplayName,
  isCustomMCPConnectorId,
} from "./custom/custom-mcp-model";
import { useCustomMCPServers } from "./custom/use-custom-mcp-servers";
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
  const [directoryMode, setDirectoryMode] =
    useState<ConnectorDirectoryMode>("catalog");
  const [customSearchQuery, setCustomSearchQuery] = useState("");
  const [configDialog, setConfigDialog] =
    useState<ConnectorConfigDialog>(null);
  const [feishuConnectionOpen, setFeishuConnectionOpen] = useState(false);
  const customMCPRoute = Boolean(
    connectorId && isCustomMCPConnectorId(connectorId),
  );
  const {
    cancelDeviceAuthSession,
    clearFeedback,
    closeDetail,
    closeLocalPairingSession,
    handleConnect,
    handleConnectFeishuManually,
    handleConnectFeishuWithQr,
    handleConnectWithCredential,
    handleDeleteOauthClient: deleteOauthClient,
    handleDeviceAuthFailure,
    handleDeviceConnected,
    handleDisconnect,
    handleLocalPairingFailure,
    handleSaveOauthClient: saveOauthClient,
    openDetail,
    refreshCatalog,
    reportFeedback,
    localPairingSession,
  } = controller;
  const customMCP = useCustomMCPServers({
    enabled: customMCPRoute || (!connectorId && directoryMode === "custom_mcp"),
    onCatalogChanged: refreshCatalog,
    reportFeedback,
  });
  const visibleCustomMCPServers = useMemo(
    () => filterCustomMCPServers(customMCP.servers, customSearchQuery),
    [customMCP.servers, customSearchQuery],
  );
  const selectedCustomMCPServer = useMemo(() => (
    customMCPRoute
      ? customMCP.servers.find((server) => server.connector_id === connectorId)
        ?? null
      : null
  ), [connectorId, customMCP.servers, customMCPRoute]);
  const customMCPTools = useCustomMCPTools(selectedCustomMCPServer);
  const activeFeedback = controller.reconciliationFeedbacks[0]
    ?? controller.feedback;

  useEffect(() => {
    if (!connectorId) {
      closeDetail();
      return;
    }
    if (customMCPRoute) {
      closeDetail();
      return;
    }
    void openDetail(connectorId);
  }, [closeDetail, connectorId, customMCPRoute, openDetail]);

  useConnectorOauthEvents({
    completeReconciliation: controller.completeReconciliation,
    reconcileCatalog: controller.reconcileCatalog,
    refreshConnector: controller.refreshConnector,
    reportFeedback,
    requireReconciliation: controller.requireReconciliation,
  });

  const openConnectorPage = useCallback((id: string) => {
    navigate(AppRouteBuilders.connectorDetail(id));
  }, [navigate]);
  const backToConnectors = useCallback(() => {
    navigate(AppRouteBuilders.connectors());
  }, [navigate]);
  const closeConfigDialog = useCallback(() => setConfigDialog(null), []);
  const requestConnectorConnect = useCallback((id: string) => {
    if (id === "feishu-docx") {
      setFeishuConnectionOpen(true);
      return;
    }
    void handleConnect(id);
  }, [handleConnect]);

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

  const credentialDetail = configDialog?.kind === "credential"
    ? configDialog.detail
    : null;
  const oauthClientDetail = configDialog?.kind === "oauth-client"
    ? configDialog.detail
    : null;
  const activeConnectorId = configDialog?.detail.connector_id
    ?? connectorId
    ?? (feishuConnectionOpen ? "feishu-docx" : null);
  const busy = controller.pendingAction !== null
    || controller.reconciliationActions.some((action) => (
      action.connectorId === activeConnectorId
    ))
    || customMCP.busy;
  const confirmCustomMCPDelete = useCallback(async () => {
    const target = customMCP.deleteTarget;
    if (
      await customMCP.confirmDelete()
      && target?.connector_id === connectorId
    ) {
      backToConnectors();
    }
  }, [backToConnectors, connectorId, customMCP]);

  let surfaceContent: ReactNode;
  if (customMCPRoute) {
    surfaceContent = (
      <CustomMCPDetailView
        busy={busy}
        catalog={customMCPTools.catalog}
        failure={customMCPTools.failure}
        loading={customMCPTools.loading}
        onBack={backToConnectors}
        onDelete={customMCP.requestDelete}
        onEdit={customMCP.openEdit}
        onRetry={customMCPTools.refresh}
        onToggle={(enabled) => {
          if (selectedCustomMCPServer) {
            void customMCP.setEnabled(selectedCustomMCPServer, enabled);
          }
        }}
        server={selectedCustomMCPServer}
        serverLoading={customMCP.loading}
      />
    );
  } else if (connectorId) {
    surfaceContent = (
      <ConnectorDetailView
        busy={busy}
        detail={controller.selectedDetail}
        failure={controller.detailFailure}
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
        onConnect={requestConnectorConnect}
        onDisconnect={(id) => void handleDisconnect(id)}
        onReplaceOauthClient={() => setFeishuConnectionOpen(true)}
        onRetry={() => {
          void openDetail(connectorId);
        }}
      />
    );
  } else {
    const directoryGrid = directoryMode === "catalog" ? (
      <ConnectorsGrid
        activeCategory={controller.activeCategory}
        connectors={controller.connectors}
        failure={controller.catalogFailure}
        loading={controller.loading}
        onConnect={requestConnectorConnect}
        onDisconnect={(id) => void handleDisconnect(id)}
        onOpenConnector={openConnectorPage}
        onRefresh={() => {
          void refreshCatalog();
        }}
        pendingAction={controller.pendingAction}
        reconciliationActions={controller.reconciliationActions}
        searchQuery={controller.searchQuery}
      />
    ) : (
      <CustomMCPGrid
        busy={customMCP.busy}
        hasServers={customMCP.servers.length > 0}
        loading={customMCP.loading}
        onAdd={customMCP.openCreate}
        onDelete={customMCP.requestDelete}
        onEdit={customMCP.openEdit}
        onOpen={(server) => openConnectorPage(server.connector_id)}
        onToggle={(server, enabled) => {
          void customMCP.setEnabled(server, enabled);
        }}
        servers={visibleCustomMCPServers}
      />
    );
    surfaceContent = (
      <CapabilityPageLayout
        actions={directoryMode === "custom_mcp" ? (
          <UiButton
            disabled={customMCP.busy}
            onClick={customMCP.openCreate}
            size="2xs"
            tone="primary"
            variant="text"
          >
            <Plus className="h-3.5 w-3.5" />
            {t("capability.custom_mcp_add")}
          </UiButton>
        ) : undefined}
        description={t("capability.connectors_intro_description")}
        title={t("capability.connectors_intro_title")}
      >
        <ConnectorsSearchBar
          activeCategory={controller.activeCategory}
          categoryKeys={controller.categoryKeys}
          mode={directoryMode}
          onCategoryChange={controller.setActiveCategory}
          onModeChange={setDirectoryMode}
          onQueryChange={directoryMode === "catalog"
            ? controller.setSearchQuery
            : setCustomSearchQuery}
          searchQuery={directoryMode === "catalog"
            ? controller.searchQuery
            : customSearchQuery}
        />
        {directoryGrid}
      </CapabilityPageLayout>
    );
  }

  return (
    <>
      <WorkspaceSurfaceScaffold
        bodyScrollable
        stableGutter
      >
        {surfaceContent}
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
      <FeishuAppConnectionDialog
        busy={busy}
        isOpen={feishuConnectionOpen}
        onClose={() => setFeishuConnectionOpen(false)}
        onConnectManually={(clientId, clientSecret) => {
          void (async () => {
            if (await handleConnectFeishuManually(clientId, clientSecret)) {
              setFeishuConnectionOpen(false);
            }
          })();
        }}
        onScan={() => {
          void (async () => {
            if (await handleConnectFeishuWithQr()) {
              setFeishuConnectionOpen(false);
            }
          })();
        }}
      />
      <ConnectorDeviceAuthDialog
        onCancel={() => void cancelDeviceAuthSession()}
        onClose={controller.closeDeviceAuthSession}
        onConnected={async (id) => {
          try {
            await handleDeviceConnected();
            navigate(AppRouteBuilders.connectorDetail(id));
            await openDetail(id);
          } catch {
            reportFeedback({
              impact: t("capability.connector_connected_refresh_failed_impact"),
              nextStep: t("capability.connector_connected_refresh_failed_next_step"),
              tone: "error",
              title: t("capability.connector_connected_refresh_failed_title"),
            });
          }
        }}
        onError={(message, kind) => {
          if (kind) {
            handleDeviceAuthFailure(message, kind);
            return;
          }
          reportFeedback({
            impact: t("capability.connector_auth_local_error_impact"),
            nextStep: t("capability.connector_auth_local_error_next_step"),
            tone: "error",
            title: t("capability.connector_auth_local_error_title"),
            message,
          });
        }}
        onNext={controller.continueDeviceAuthSession}
        onOpenWebAuthUrl={controller.openFeishuWebAuthorizationUrl}
        session={controller.deviceAuthSession}
      />
      <RichMailPairingDialog
        onCancel={closeLocalPairingSession}
        onClose={closeLocalPairingSession}
        onConnected={async (id) => {
          try {
            await handleDeviceConnected();
            navigate(AppRouteBuilders.connectorDetail(id));
            await openDetail(id);
          } catch {
            reportFeedback({
              impact: t("capability.connector_connected_refresh_failed_impact"),
              nextStep: t("capability.connector_connected_refresh_failed_next_step"),
              tone: "error",
              title: t("capability.connector_connected_refresh_failed_title"),
            });
          }
        }}
        onError={handleLocalPairingFailure}
        session={localPairingSession}
      />
      <ShopDomainPromptDialog
        onCancel={controller.cancelShopDomainPrompt}
        onConfirm={controller.confirmShopDomainPrompt}
        state={controller.shopDomainPrompt}
      />
      {customMCP.dialogState ? (
        <CustomMCPDialog
          busy={customMCP.busy}
          key={customMCP.dialogState.mode === "edit"
            ? customMCP.dialogState.server.connector_id
            : "create"}
          onClose={customMCP.closeDialog}
          onSave={customMCP.save}
          {...(customMCP.dialogState.mode === "edit"
            ? { server: customMCP.dialogState.server }
            : {})}
        />
      ) : null}
      <ConfirmDialog
        confirmText={t("capability.custom_mcp_delete_confirm")}
        isOpen={customMCP.deleteTarget !== null}
        message={customMCP.deleteTarget
          ? t("capability.custom_mcp_delete_message", {
              name: getCustomMCPDisplayName(
                customMCP.deleteTarget,
                t("capability.custom_mcp_recovery_name"),
              ),
            })
          : ""}
        onCancel={() => customMCP.requestDelete(null)}
        onConfirm={() => void confirmCustomMCPDelete()}
        title={t("capability.custom_mcp_delete_title")}
        variant="danger"
      />
      <FeedbackBannerViewport
        item={activeFeedback
          ? completeFeedbackBanner(
            activeFeedback.tone === "success"
              ? {
                  message: activeFeedback.message ?? activeFeedback.title,
                  onDismiss: activeFeedback.persistent ? undefined : clearFeedback,
                  title: activeFeedback.title,
                  tone: "success",
                }
              : {
                  action: activeFeedback.action,
                  impact: activeFeedback.impact,
                  nextStep: activeFeedback.nextStep,
                  onDismiss: activeFeedback.persistent ? undefined : clearFeedback,
                  title: activeFeedback.title,
                  tone: activeFeedback.tone,
                },
            {
              impact: t("feedback.unconfirmed_impact"),
            },
          )
          : null}
      />
    </>
  );
}
