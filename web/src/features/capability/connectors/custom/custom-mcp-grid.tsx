"use client";

import { Cable, Pencil, Plus, Server, Trash2 } from "lucide-react";

import {
  CAPABILITY_DIRECTORY_GRID_CLASS_NAME,
  CAPABILITY_DIRECTORY_ROW_CLASS_NAME,
  CapabilityItemIcon,
  CapabilitySectionHeader,
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
        <Cable className="h-7 w-7 text-(--icon-muted)" />
        <h2 className="mt-3 text-base font-medium text-(--text-strong)">
          {hasServers
            ? t("capability.custom_mcp_no_results_title")
            : t("capability.custom_mcp_empty_title")}
        </h2>
        <p className="mt-1 max-w-[480px] text-compact leading-5 text-(--text-muted)">
          {hasServers
            ? t("capability.custom_mcp_no_results_description")
            : t("capability.custom_mcp_empty_description")}
        </p>
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
      <CapabilitySectionHeader
        count={t("capability.result_count", { count: servers.length })}
        description={t("capability.custom_mcp_enable_hint")}
        title={t("capability.custom_mcp_section_title")}
      />
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
