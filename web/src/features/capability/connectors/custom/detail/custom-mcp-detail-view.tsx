// INPUT: 脱敏自定义 MCP 配置、owner 级启停命令与 tools/list 快照。
// OUTPUT: Claude 风格的连接信息、服务器信息和工具目录详情页。
// POS: 自定义 MCP 子页面纯视图。
"use client";

import {
  ArrowLeft,
  ChevronRight,
  Pencil,
  Trash2,
  TriangleAlert,
} from "lucide-react";
import type { ReactNode } from "react";

import type { ResourceFailure } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { WorkspaceContentDetailHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { UiPanel } from "@/shared/ui/panel";
import type {
  CustomMCPServer,
  CustomMCPToolCatalog,
} from "@/types/capability/connector";

import { ConnectorIcon } from "../../connector-icon";
import { MCPToolsSection } from "../../mcp/mcp-tools-section";
import {
  getCustomMCPConnectionTarget,
  getCustomMCPDisplayName,
  isCustomMCPRecoveryRequired,
} from "../custom-mcp-model";

interface CustomMCPDetailViewProps {
  busy: boolean;
  catalog: CustomMCPToolCatalog | null;
  failure: ResourceFailure | null;
  loading: boolean;
  onBack: () => void;
  onDelete: (server: CustomMCPServer) => void;
  onEdit: (server: CustomMCPServer) => void;
  onRetry: () => void;
  onToggle: (enabled: boolean) => void;
  server: CustomMCPServer | null;
  serverLoading: boolean;
}

export function CustomMCPDetailView({
  busy,
  catalog,
  failure,
  loading,
  onBack,
  onDelete,
  onEdit,
  onRetry,
  onToggle,
  server,
  serverLoading,
}: CustomMCPDetailViewProps) {
  const { t } = useI18n();
  if (serverLoading && !server) {
    return (
      <CustomMCPDetailFrame onBack={onBack}>
        <UiResourceState
          className="min-h-[420px]"
          state="loading"
          title={t("capability.custom_mcp_detail_loading")}
        />
      </CustomMCPDetailFrame>
    );
  }
  if (!server) {
    return (
      <CustomMCPDetailFrame onBack={onBack}>
        <UiResourceState
          className="min-h-[420px]"
          primaryAction={{ label: t("common.back"), onClick: onBack }}
          state="empty"
          title={t("capability.custom_mcp_detail_missing")}
        />
      </CustomMCPDetailFrame>
    );
  }
  const recoveryRequired = isCustomMCPRecoveryRequired(server);
  const displayName = getCustomMCPDisplayName(
    server,
    t("capability.custom_mcp_recovery_name"),
  );

  return (
    <CustomMCPDetailFrame name={displayName} onBack={onBack}>
      <div className="pt-5">
        <header className="flex flex-wrap items-start justify-between gap-4 border-b border-(--divider-subtle-color) pb-5">
          <div className="flex min-w-0 items-start gap-4">
            <ConnectorIcon
              icon="custom-mcp"
              size="lg"
              title={recoveryRequired ? server.connector_id : server.name}
            />
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h1 className="text-lg font-semibold tracking-[-0.025em] text-(--text-strong)">
                  {displayName}
                </h1>
                <UiBadge tone={recoveryRequired
                  ? "warning"
                  : server.enabled ? "success" : "default"}
                >
                  {t(recoveryRequired
                    ? "capability.custom_mcp_recovery_badge"
                    : server.enabled
                      ? "capability.custom_mcp_enabled"
                      : "capability.custom_mcp_disabled")}
                </UiBadge>
              </div>
              {recoveryRequired ? (
                <p className="mt-1 max-w-[680px] text-xs text-(--text-muted)">
                  {t("capability.custom_mcp_recovery_summary")}
                </p>
              ) : (
                <p
                  className="mt-1 max-w-[680px] truncate font-mono text-xs text-(--text-muted)"
                  title={getCustomMCPConnectionTarget(server)}
                >
                  {getCustomMCPConnectionTarget(server)}
                </p>
              )}
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <span className="mr-1 text-xs font-medium text-(--text-muted)">
              {t("capability.custom_mcp_available_in_chat")}
            </span>
            <GlassSwitch
              aria-label={t("capability.custom_mcp_available_in_chat")}
              checked={!recoveryRequired && server.enabled}
              disabled={busy || recoveryRequired}
              onChange={onToggle}
              size="sm"
            />
            <UiIconButton
              aria-label={t(recoveryRequired
                ? "capability.custom_mcp_recover_action"
                : "common.edit")}
              disabled={busy}
              onClick={() => onEdit(server)}
              size="md"
              variant="ghost"
            >
              <Pencil className="h-4 w-4" />
            </UiIconButton>
            <UiIconButton
              aria-label={t("common.delete")}
              disabled={busy}
              onClick={() => onDelete(server)}
              size="md"
              tone="danger"
              variant="ghost"
            >
              <Trash2 className="h-4 w-4" />
            </UiIconButton>
          </div>
        </header>

        {recoveryRequired ? (
          <CustomMCPRecoverySection
            busy={busy}
            onRecover={() => onEdit(server)}
          />
        ) : (
          <>
            <CustomMCPConnectionSection catalog={catalog} server={server} />
            <MCPToolsSection
              available={server.enabled}
              catalog={catalog}
              failure={failure}
              loading={loading}
              onRetry={onRetry}
            />
          </>
        )}
      </div>
    </CustomMCPDetailFrame>
  );
}

function CustomMCPRecoverySection({
  busy,
  onRecover,
}: {
  busy: boolean;
  onRecover: () => void;
}) {
  const { t } = useI18n();
  return (
    <section className="py-5">
      <UiPanel className="flex items-start gap-3" padding="md" radius="md">
        <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-[9px] bg-[color:color-mix(in_srgb,var(--warning)_8%,transparent)] text-(--warning)">
          <TriangleAlert className="h-4 w-4" />
        </span>
        <div className="min-w-0 flex-1">
          <h2 className="text-base font-medium text-(--text-strong)">
            {t("capability.custom_mcp_recovery_title")}
          </h2>
          <p className="mt-1 text-sm leading-6 text-(--text-muted)">
            {t("capability.custom_mcp_recovery_description")}
          </p>
          <p className="mt-1 text-sm leading-6 text-(--text-muted)">
            {t("capability.custom_mcp_recovery_next_step")}
          </p>
          <UiButton
            className="mt-4"
            disabled={busy}
            onClick={onRecover}
            type="button"
            variant="solid"
          >
            {t("capability.custom_mcp_recover_action")}
          </UiButton>
        </div>
      </UiPanel>
    </section>
  );
}

function CustomMCPDetailFrame({
  children,
  name,
  onBack,
}: {
  children: ReactNode;
  name?: string;
  onBack: () => void;
}) {
  const { t } = useI18n();
  return (
    <div className={WORKSPACE_CONTENT_PAGE_CLASS_NAME}>
      <WorkspaceContentDetailHeader>
        <div className="flex min-w-0 items-center gap-2 text-base text-(--text-muted)">
          <button
            className="inline-flex items-center gap-1 rounded-[8px] px-1.5 py-1 font-medium transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
            onClick={onBack}
            type="button"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            {t("capability.connectors_tab_custom_mcp")}
          </button>
          {name ? (
            <>
              <ChevronRight className="h-3.5 w-3.5 text-(--icon-muted)" />
              <span className="truncate font-medium text-(--text-strong)">
                {name}
              </span>
            </>
          ) : null}
        </div>
      </WorkspaceContentDetailHeader>
      {children}
    </div>
  );
}

function CustomMCPConnectionSection({
  catalog,
  server,
}: {
  catalog: CustomMCPToolCatalog | null;
  server: CustomMCPServer;
}) {
  const { t } = useI18n();
  const authLabel = server.type === "stdio"
    ? t("capability.custom_mcp_auth_local")
    : getCustomMCPAuthLabel(server.auth_type ?? "none", t);
  const serverIdentity = catalog?.server_title
    || catalog?.server_name
    || t("capability.custom_mcp_server_unknown");
  return (
    <section className="border-b border-(--divider-subtle-color) py-5">
      <h2 className="text-base font-medium text-(--text-strong)">
        {t("capability.custom_mcp_connection_info")}
      </h2>
      <dl className="mt-3 grid gap-x-8 gap-y-3 sm:grid-cols-2">
        <CustomMCPFact
          label={t("capability.custom_mcp_transport")}
          value={server.type.toUpperCase()}
        />
        <CustomMCPFact
          label={t("capability.custom_mcp_authentication")}
          value={authLabel}
        />
        <CustomMCPFact
          label={t("capability.custom_mcp_server_identity")}
          value={catalog?.server_version
            ? `${serverIdentity} · ${catalog.server_version}`
            : serverIdentity}
        />
        <CustomMCPFact
          label={t("capability.custom_mcp_protocol_version")}
          value={catalog?.protocol_version
            || t("capability.custom_mcp_protocol_pending")}
        />
      </dl>
      {catalog?.instructions ? (
        <div className="mt-4 border-l-2 border-(--divider-strong-color) pl-3">
          <h3 className="text-xs font-semibold text-(--text-strong)">
            {t("capability.custom_mcp_server_instructions")}
          </h3>
          <p className="mt-1 whitespace-pre-wrap text-sm leading-6 text-(--text-muted)">
            {catalog.instructions}
          </p>
        </div>
      ) : null}
    </section>
  );
}

function getCustomMCPAuthLabel(
  authType: NonNullable<CustomMCPServer["auth_type"]>,
  t: ReturnType<typeof useI18n>["t"],
): string {
  switch (authType) {
    case "bearer":
      return t("capability.custom_mcp_auth_bearer");
    case "headers":
      return t("capability.custom_mcp_auth_headers");
    default:
      return t("capability.custom_mcp_auth_none");
  }
}

function CustomMCPFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[112px_minmax(0,1fr)] gap-3 text-sm">
      <dt className="text-(--text-soft)">{label}</dt>
      <dd className="min-w-0 break-words text-(--text-default)">{value}</dd>
    </div>
  );
}
