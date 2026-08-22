/**
 * INPUT: owner 级自定义 MCP 目录及增删改命令。
 * OUTPUT: 不重复页签标题和启用教程的 MCP 目录。
 * POS: Connector 页的自定义 MCP 子目录视图。
 */
"use client";

import { Pencil, Plus, Server, Trash2 } from "lucide-react";

import {
  CAPABILITY_DIRECTORY_GRID_CLASS_NAME,
  CAPABILITY_DIRECTORY_ROW_CLASS_NAME,
  CapabilityItemIcon,
} from "@/features/capability/shared/capability-page-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiPanel } from "@/shared/ui/panel";
import type { CustomMCPServer } from "@/types/capability/connector";

interface CustomMCPGridProps {
  busy: boolean;
  hasServers: boolean;
  loading: boolean;
  onAdd: () => void;
  onDelete: (server: CustomMCPServer) => void;
  onEdit: (server: CustomMCPServer) => void;
  servers: CustomMCPServer[];
}

export function CustomMCPGrid({
  busy,
  hasServers,
  loading,
  onAdd,
  onDelete,
  onEdit,
  servers,
}: CustomMCPGridProps) {
  const { t } = useI18n();

  if (loading) {
    return (
      <div className="flex min-h-40 items-center justify-center text-sm text-(--text-muted)">
        {t("capability.connectors_loading")}
      </div>
    );
  }
  if (servers.length === 0) {
    return (
      <div className="flex min-h-48 flex-col items-center justify-center border-y border-(--divider-subtle-color) px-6 text-center">
        <h2 className="text-base font-medium text-(--text-strong)">
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
        {servers.map((server) => (
          <UiPanel
            className={CAPABILITY_DIRECTORY_ROW_CLASS_NAME}
            key={server.connector_id}
            padding="none"
            radius="sm"
          >
            <div className="flex h-full min-w-0 items-start gap-3">
              <CapabilityItemIcon>
                <Server className="h-4 w-4" />
              </CapabilityItemIcon>
              <div className="min-w-0 flex-1">
                <div className="flex min-w-0 items-center gap-2">
                  <h3 className="min-w-0 flex-1 truncate text-sm font-semibold text-(--text-strong)">
                    {server.name}
                  </h3>
                  <UiBadge>{server.type.toUpperCase()}</UiBadge>
                </div>
                <p
                  className="mt-1 truncate font-mono text-xs text-(--text-muted)"
                  title={server.type === "stdio" ? server.command : server.url}
                >
                  {server.type === "stdio" ? server.command : server.url}
                </p>
                <div className="mt-2 flex items-center gap-1">
                  <UiIconButton
                    disabled={busy}
                    onClick={() => onEdit(server)}
                    size="sm"
                    title={t("common.edit")}
                    type="button"
                  >
                    <Pencil className="h-3.5 w-3.5" />
                  </UiIconButton>
                  <UiIconButton
                    disabled={busy}
                    onClick={() => onDelete(server)}
                    size="sm"
                    title={t("common.delete")}
                    tone="danger"
                    type="button"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </UiIconButton>
                </div>
              </div>
            </div>
          </UiPanel>
        ))}
      </div>
    </section>
  );
}
