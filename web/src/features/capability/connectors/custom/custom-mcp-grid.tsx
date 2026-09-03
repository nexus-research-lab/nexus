/**
 * INPUT: owner 级自定义 MCP 目录及增删改命令。
 * OUTPUT: 不重复页签标题和启用教程的 MCP 目录。
 * POS: Connector 页的自定义 MCP 子目录视图。
 */
"use client";

import { Pencil, Plus, Trash2 } from "lucide-react";
import type { MouseEvent } from "react";

import {
  CAPABILITY_DIRECTORY_GRID_CLASS_NAME,
  CAPABILITY_DIRECTORY_ROW_CLASS_NAME,
} from "@/features/capability/shared/capability-page-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
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
  onAdd,
  onDelete,
  onEdit,
  onOpen,
  onToggle,
  servers,
}: CustomMCPGridProps) {
  const { t } = useI18n();

  if (loading) {
    return (
      <div className={cn(
        "flex min-h-40 items-center justify-center",
        getUiTypographyClassName({ role: "supporting", tone: "muted" }),
      )}>
        {t("capability.connectors_loading")}
      </div>
    );
  }
  if (servers.length === 0) {
    return (
      <div className="flex min-h-48 flex-col items-center justify-center border-y border-(--divider-subtle-color) px-6 text-center">
        <h2 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong", weight: "medium" })}>
          {hasServers
            ? t("capability.custom_mcp_no_results_title")
            : t("capability.custom_mcp_empty_title")}
        </h2>
        {!hasServers ? (
          <UiButton
            className="mt-4"
            disabled={busy}
            onClick={onAdd}
            tone="primary"
            type="button"
            variant="solid"
          >
            <Plus className="h-3.5 w-3.5" />
            {t("capability.custom_mcp_add")}
          </UiButton>
        ) : null}
      </div>
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
                  <UiIconButton
                    aria-label={t(recoveryRequired
                      ? "capability.custom_mcp_recover_action"
                      : "common.edit")}
                    disabled={busy}
                    onClick={(event) => handleAction(event, () => onEdit(server))}
                    size="sm"
                    type="button"
                  >
                    <Pencil className="h-3.5 w-3.5" />
                  </UiIconButton>
                  <UiIconButton
                    aria-label={t("common.delete")}
                    disabled={busy}
                    onClick={(event) => handleAction(event, () => onDelete(server))}
                    size="sm"
                    tone="danger"
                    type="button"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </UiIconButton>
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

function handleAction(
  event: MouseEvent<HTMLButtonElement>,
  action: () => void,
): void {
  event.stopPropagation();
  action();
}
