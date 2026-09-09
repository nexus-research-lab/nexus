/**
 * INPUT: owner 级自定义 MCP 目录及增删改命令。
 * OUTPUT: 公共资源状态、outlined 条目与独立行内动作组成的 MCP 目录。
 * POS: Connector 页的自定义 MCP 子目录视图。
 */
"use client";

import { Pencil, Plus, Trash2 } from "lucide-react";

import {
  CAPABILITY_DIRECTORY_GRID_CLASS_NAME,
  CAPABILITY_DIRECTORY_ROW_CLASS_NAME,
} from "@/features/capability/shared/capability-page-layout";
import type { ResourceFailure } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { UiListRow } from "@/shared/ui/list/list-row";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { CustomMCPServer } from "@/types/capability/connector";

import { ConnectorIcon } from "../connector-icon";
import {
  getCustomMCPConnectionTarget,
  getCustomMCPDisplayName,
  isCustomMCPRecoveryRequired,
} from "./custom-mcp-model";

interface CustomMCPGridProps {
  busy: boolean;
  hasServers: boolean;
  loading: boolean;
  failure?: ResourceFailure | null;
  onRetry?: () => void;
  onAdd: () => void;
  onDelete: (server: CustomMCPServer) => void;
  onEdit: (server: CustomMCPServer) => void;
  onOpen: (server: CustomMCPServer) => void;
  onToggle: (server: CustomMCPServer, enabled: boolean) => void;
  servers: CustomMCPServer[];
}

export function CustomMCPGrid({
  busy,
  hasServers,
  loading,
  failure,
  onRetry,
  onAdd,
  onDelete,
  onEdit,
  onOpen,
  onToggle,
  servers,
}: CustomMCPGridProps) {
  const { t } = useI18n();

  if (failure && (failure.access || !hasServers)) {
    return <UiResourceState state="error" title={t("capability.custom_mcp_operation_failed")} impact={t("state.read_failure_impact")} primaryAction={onRetry ? { label: t("state.retry"), onClick: onRetry, disabled: loading } : undefined} />;
  }
  if (loading && !hasServers) {
    return (
      <UiResourceState size="sm" state="loading" title={t("capability.connectors_loading")} variant="plain" />
    );
  }
  if (servers.length === 0) {
    return (
      <UiResourceState
        primaryAction={hasServers ? undefined : {
          disabled: busy,
          icon: <Plus className="h-3.5 w-3.5" />,
          label: t("capability.custom_mcp_add"),
          onClick: onAdd,
        }}
        size="sm"
        state="empty"
        title={t(hasServers ? "capability.custom_mcp_no_results_title" : "capability.custom_mcp_empty_title")}
        variant="plain"
      />
    );
  }

  return (
    <section>
      <div className={CAPABILITY_DIRECTORY_GRID_CLASS_NAME}>
        {servers.map((server) => {
          const recoveryRequired = isCustomMCPRecoveryRequired(server);
          const displayName = getCustomMCPDisplayName(
            server,
            t("capability.custom_mcp_recovery_name"),
          );
          return (
            <UiListRow
              variant="outlined"
              className={CAPABILITY_DIRECTORY_ROW_CLASS_NAME}
              description={recoveryRequired ? (
                t("capability.custom_mcp_recovery_summary")
              ) : (
                <span
                  className={getUiTypographyClassName({ role: "code" })}
                  title={getCustomMCPConnectionTarget(server)}
                >
                  {getCustomMCPConnectionTarget(server)}
                </span>
              )}
              key={server.connector_id}
              leading={(
                <ConnectorIcon
                  icon="custom-mcp"
                  title={recoveryRequired ? server.connector_id : server.name}
                />
              )}
              meta={(
                <>
                  {recoveryRequired ? (
                    <UiBadge size="xs" tone="warning">
                      {t("capability.custom_mcp_recovery_badge")}
                    </UiBadge>
                  ) : (
                    <UiBadge size="xs">{server.type.toUpperCase()}</UiBadge>
                  )}
                  {!recoveryRequired && !server.enabled ? (
                    <UiBadge size="xs" tone="idle">
                      {t("capability.custom_mcp_disabled")}
                    </UiBadge>
                  ) : null}
                </>
              )}
              onClick={() => onOpen(server)}
              right={(
                <div
                  className="flex shrink-0 items-center gap-1"
                  onClick={(event) => event.stopPropagation()}
                  role="presentation"
                >
                  <GlassSwitch
                    aria-label={t("capability.custom_mcp_available_in_chat")}
                    checked={!recoveryRequired && server.enabled}
                    disabled={busy || recoveryRequired}
                    onChange={(enabled) => onToggle(server, enabled)}
                    size="xs"
                  />
                  <UiListActionButton
                    aria-label={t(recoveryRequired
                      ? "capability.custom_mcp_recover_action"
                      : "common.edit")}
                    disabled={busy}
                    onClick={() => onEdit(server)}
                    size="sm"
                    stopPropagation
                    visibility="visible"
                  >
                    <Pencil className="h-3.5 w-3.5" />
                  </UiListActionButton>
                  <UiListActionButton
                    aria-label={t("common.delete")}
                    disabled={busy}
                    onClick={() => onDelete(server)}
                    size="sm"
                    tone="danger"
                    stopPropagation
                    visibility="visible"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </UiListActionButton>
                </div>
              )}
              title={displayName}
            />
          );
        })}
      </div>
    </section>
  );
}
