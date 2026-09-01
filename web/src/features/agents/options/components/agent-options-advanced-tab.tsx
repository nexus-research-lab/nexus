/**
 * INPUT: Agent 权限模式、预授权工具和连接器选择状态。
 * OUTPUT: 同时展示模式差异、工具用途、连接器状态与对应开关的高级配置页。
 * POS: Agent 详情中的授权决策面；说明用于比较选择而非装饰。
 */

"use client";

import {
  Bot,
  Check,
  FilePlus2,
  Globe2,
  Loader2,
  Pencil,
  Search,
  Terminal,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { SIDEBAR_SELECTION_CLASS_NAME } from "@/shared/ui/sidebar/sidebar-selection";
import { ConnectorIcon } from "@/features/capability/connectors/connector-icon";
import type { ConnectorInfo } from "@/types/capability/connector";
import {
  AGENT_PERMISSION_MODES,
  AVAILABLE_AGENT_TOOLS,
  countVisibleAgentPreauthorizedTools,
} from "@/lib/agent-options";

interface AgentOptionsAdvancedTabProps {
  permissionMode: string;
  onPermissionModeChange: (mode: string) => void;
  allowedTools: string[];
  onToggleTool: (toolName: string, type: "allowed" | "disallowed") => void;
  connectorIds: string[];
  connectors: ConnectorInfo[];
  connectorsError: string | null;
  connectorsLoading: boolean;
  onRetryConnectors: () => void;
  onToggleConnector: (connectorId: string) => void;
}

const TOOL_ICONS: Record<
  (typeof AVAILABLE_AGENT_TOOLS)[number]["name"],
  LucideIcon
> = {
  Agent: Bot,
  Bash: Terminal,
  Edit: Pencil,
  WebFetch: Globe2,
  WebSearch: Search,
  Write: FilePlus2,
};

/** Advanced Tab 组件 — 权限控制与工具授权 */
export function AgentOptionsAdvancedTab({
  permissionMode: permissionMode,
  onPermissionModeChange: onPermissionModeChange,
  allowedTools: allowedTools,
  onToggleTool: onToggleTool,
  connectorIds,
  connectors,
  connectorsError,
  connectorsLoading,
  onRetryConnectors,
  onToggleConnector,
}: AgentOptionsAdvancedTabProps) {
  const { t } = useI18n();
  const isBypassPermissionMode = permissionMode === "bypassPermissions";
  const preauthorizedToolCount = countVisibleAgentPreauthorizedTools(allowedTools);

  return (
    <div className="space-y-6 animate-in fade-in duration-200 [overflow-anchor:none]">
      <section className="space-y-3">
        <SectionHeader
          description={t("agent_options.advanced.permission_control_hint")}
          title={t("agent_options.advanced.permission_control")}
        />
        <div className="grid grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-2">
          {AGENT_PERMISSION_MODES.map((mode) => {
            const isActive = permissionMode === mode.value;
            return (
              <button
                aria-pressed={isActive}
                className={cn(
                  "flex min-h-[72px] min-w-0 flex-col rounded-[10px] border px-3 py-2.5 text-left transition-[background,border-color] duration-(--motion-duration-fast)",
                  isActive
                    ? SIDEBAR_SELECTION_CLASS_NAME
                    : "border-(--divider-subtle-color) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)",
                )}
                key={mode.value}
                onClick={() => onPermissionModeChange(mode.value)}
                type="button"
              >
                <span className="flex w-full min-w-0 items-center gap-2 text-sm font-semibold text-(--text-strong)">
                  <span className="min-w-0 flex-1 truncate">{t(mode.labelKey)}</span>
                  {isActive ? <Check className="h-3.5 w-3.5 shrink-0" /> : null}
                </span>
                <span
                  className="mt-1 line-clamp-2 text-xs leading-[1.45] text-(--text-muted)"
                  title={t(mode.descriptionKey)}
                >
                  {t(mode.descriptionKey)}
                </span>
              </button>
            );
          })}
        </div>
        {isBypassPermissionMode ? (
          <div className="flex items-start gap-2 rounded-[8px] bg-[color:color-mix(in_srgb,var(--warning)_7%,transparent)] px-3 py-2 text-xs leading-5 text-(--warning)">
            <TriangleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <span>{t("agent_options.advanced.bypass_warning")}</span>
          </div>
        ) : null}
      </section>

      <section className="space-y-3">
        <SectionHeader
          description={t("agent_options.advanced.security_hint")}
          title={t("agent_options.advanced.tool_access")}
          trailing={t("agent_options.advanced.enabled_tools", {
            count: preauthorizedToolCount,
          })}
        />
        <div className="grid grid-cols-1 gap-2 md:grid-cols-2 xl:grid-cols-3 [overflow-anchor:none]">
          {AVAILABLE_AGENT_TOOLS.map((tool) => (
            <ToolAuthorizationRow
              checked={allowedTools.includes(tool.name)}
              description={t(tool.descriptionKey)}
              key={tool.name}
              name={tool.name}
              onToggle={() => onToggleTool(tool.name, "allowed")}
            />
          ))}
        </div>
      </section>

      <section className="space-y-3">
        <SectionHeader
          description={t("agent_options.advanced.connector_access_hint")}
          title={t("agent_options.advanced.connector_access")}
          trailing={t("agent_options.advanced.enabled_connectors", {
            count: connectorIds.length,
          })}
        />
        {connectorsLoading && connectors.length === 0 ? (
          <div className="flex h-16 items-center justify-center text-(--icon-muted)">
            <Loader2 className="h-4 w-4 animate-spin" />
          </div>
        ) : null}
        {connectorsError ? (
          <UiResourceState
            className="min-h-0 py-3"
            impact={t(connectors.length > 0
              ? "agent_options.advanced.connector_load_failed_stale_impact"
              : "agent_options.advanced.connector_load_failed_empty_impact")}
            primaryAction={{
              busy: connectorsLoading,
              label: t("state.retry"),
              onClick: onRetryConnectors,
            }}
            size="sm"
            state="error"
            title={connectorsError}
            urgency="polite"
            variant="card"
          />
        ) : null}
        {!connectorsLoading && !connectorsError && connectors.length === 0 ? (
          <p className="text-xs text-(--text-soft)">
            {t("agent_options.advanced.connector_empty")}
          </p>
        ) : null}
        {connectors.length > 0 ? (
          <div className="grid grid-cols-1 gap-2 md:grid-cols-2 xl:grid-cols-3 [overflow-anchor:none]">
            {connectors.map((connector) => (
              <ConnectorAuthorizationRow
                checked={connectorIds.includes(connector.connector_id)}
                connector={connector}
                key={connector.connector_id}
                onToggle={() => onToggleConnector(connector.connector_id)}
              />
            ))}
          </div>
        ) : null}
      </section>
    </div>
  );
}

function SectionHeader({
  description,
  title,
  trailing,
}: {
  description: string;
  title: string;
  trailing?: string;
}) {
  return (
    <div className="flex flex-col gap-1.5 sm:flex-row sm:items-start sm:justify-between sm:gap-4">
      <div className="min-w-0">
        <h3 className="text-sm font-semibold text-(--text-strong)">{title}</h3>
        <p className="mt-0.5 max-w-[720px] text-xs leading-5 text-(--text-soft)">
          {description}
        </p>
      </div>
      {trailing ? (
        <span className="shrink-0 text-xs tabular-nums text-(--text-soft) sm:pt-0.5">
          {trailing}
        </span>
      ) : null}
    </div>
  );
}

function ToolAuthorizationRow({
  checked,
  description,
  name,
  onToggle,
}: {
  checked: boolean;
  description: string;
  name: (typeof AVAILABLE_AGENT_TOOLS)[number]["name"];
  onToggle: () => void;
}) {
  const Icon = TOOL_ICONS[name];
  return (
    <div
      className={cn(
        "grid min-h-[64px] grid-cols-[30px_minmax(0,1fr)_auto] items-center gap-2.5 rounded-[10px] border px-3 py-2.5 transition-[background,border-color] duration-(--motion-duration-fast)",
        checked
          ? SIDEBAR_SELECTION_CLASS_NAME
          : "border-(--divider-subtle-color) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)",
      )}
    >
      <span className="flex h-[30px] w-[30px] items-center justify-center rounded-[8px] bg-(--surface-panel-subtle-background) text-(--icon-muted)">
        <Icon className="h-4 w-4" />
      </span>
      <span className="min-w-0">
        <span className="block truncate text-compact font-semibold text-(--text-strong)">
          {name}
        </span>
        <span
          className="mt-0.5 block truncate text-xs leading-4 text-(--text-muted)"
          title={description}
        >
          {description}
        </span>
      </span>
      <GlassSwitch
        aria-label={name}
        checked={checked}
        onChange={onToggle}
        size="xs"
      />
    </div>
  );
}

function ConnectorAuthorizationRow({
  checked,
  connector,
  onToggle,
}: {
  checked: boolean;
  connector: ConnectorInfo;
  onToggle: () => void;
}) {
  const { t } = useI18n();
  const connected = connector.connection_state === "connected";
  return (
    <div
      className={cn(
        "grid min-h-[64px] grid-cols-[30px_minmax(0,1fr)_auto] items-center gap-2.5 rounded-[10px] border px-3 py-2.5 transition-[background,border-color] duration-(--motion-duration-fast)",
        checked
          ? SIDEBAR_SELECTION_CLASS_NAME
          : "border-(--divider-subtle-color) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)",
        !connected && !checked && "opacity-(--disabled-opacity)",
      )}
    >
      <ConnectorIcon
        className="h-[30px] w-[30px] rounded-[8px]"
        icon={connector.icon}
        title={connector.title}
      />
      <span className="min-w-0">
        <span className="block truncate text-compact font-semibold text-(--text-strong)">
          {connector.title}
        </span>
        <span className="mt-0.5 block truncate text-xs leading-4 text-(--text-muted)">
          {connected
            ? connector.description
            : t("agent_options.advanced.connector_disconnected")}
        </span>
      </span>
      <GlassSwitch
        aria-label={connector.title}
        checked={checked}
        disabled={!connected && !checked}
        onChange={onToggle}
        size="xs"
      />
    </div>
  );
}
