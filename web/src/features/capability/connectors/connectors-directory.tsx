/**
 * INPUT: Connector/自定义 MCP 目录、详情路由与认证命令。
 * OUTPUT: 低密度连接器目录、配置入口或详情页面。
 * POS: “能力 > 连接器”的唯一页面装配入口。
 */
"use client";

import { Plus } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { CapabilityPageLayout } from "@/features/capability/shared/capability-page-layout";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-toolbar-action";
import type { ConnectorsRouteParams } from "@/types/app/route";
import type { ConnectorDetail } from "@/types/capability/connector";

import { ConnectorCredentialDialog } from "./auth/connector-credential-dialog";
import { ConnectorDeviceAuthDialog } from "./auth/device-flow/connector-device-auth-dialog";
import { FeishuAppConnectionDialog } from "./auth/feishu/feishu-app-connection-dialog";
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
import { CustomMCPGrid } from "./custom/custom-mcp-grid";
import { filterCustomMCPServers } from "./custom/custom-mcp-model";
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
  const {
    cancelDeviceAuthSession,
    clearFeedback,
    closeDetail,
    handleConnect,
    handleConnectFeishuManually,
    handleConnectFeishuWithQr,
    handleConnectWithCredential,
    handleDeleteOauthClient: deleteOauthClient,
    handleDeviceConnected,
    handleDisconnect,
    handleSaveOauthClient: saveOauthClient,
    openDetail,
    refreshCatalog,
    reportFeedback,
  } = controller;
  const customMCP = useCustomMCPServers({
    enabled: !connectorId && directoryMode === "custom_mcp",
    onCatalogChanged: refreshCatalog,
    reportFeedback,
  });
  const visibleCustomMCPServers = useMemo(
    () => filterCustomMCPServers(customMCP.servers, customSearchQuery),
    [customMCP.servers, customSearchQuery],
  );

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
            onConnect={requestConnectorConnect}
            onDisconnect={(id) => void handleDisconnect(id)}
            onReplaceOauthClient={() => setFeishuConnectionOpen(true)}
          />
        ) : (
          <CapabilityPageLayout
            actions={directoryMode === "custom_mcp" ? (
              <WorkspaceSurfaceToolbarAction
                disabled={customMCP.busy}
                onClick={customMCP.openCreate}
                tone="primary"
              >
                <Plus className="h-3.5 w-3.5" />
                {t("capability.custom_mcp_add")}
              </WorkspaceSurfaceToolbarAction>
            ) : undefined}
            description={t("capability.connectors_intro_description")}
            title={t("capability.connectors_intro_title")}
          >
            <ConnectorsSearchBar
              activeCategory={controller.activeCategory}
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
            {directoryMode === "catalog" ? (
              <ConnectorsGrid
                activeCategory={controller.activeCategory}
                connectors={controller.connectors}
                loading={controller.loading}
                onConnect={requestConnectorConnect}
                onDisconnect={(id) => void handleDisconnect(id)}
                onOpenConnector={openConnectorPage}
                pendingAction={controller.pendingAction}
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
                servers={visibleCustomMCPServers}
              />
            )}
          </CapabilityPageLayout>
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
          } catch (error) {
            reportFeedback({
              tone: "error",
              title: "连接已完成",
              message: getErrorMessage(
                error,
                "飞书已连接，但页面状态刷新失败，请重新打开连接器页面",
              ),
            });
          }
        }}
        onError={(message) => {
          reportFeedback({
            tone: "error",
            title: "操作失败",
            message,
          });
          void cancelDeviceAuthSession();
        }}
        onNext={controller.continueDeviceAuthSession}
        onOpenWebAuthUrl={controller.openFeishuWebAuthorizationUrl}
        session={controller.deviceAuthSession}
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
              name: customMCP.deleteTarget.name,
            })
          : ""}
        onCancel={() => customMCP.requestDelete(null)}
        onConfirm={() => void customMCP.confirmDelete()}
        title={t("capability.custom_mcp_delete_title")}
        variant="danger"
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
