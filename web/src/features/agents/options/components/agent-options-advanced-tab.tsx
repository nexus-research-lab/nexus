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
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { UiListRow } from "@/shared/ui/list/list-row";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { WorkspaceIconFrame } from "@/shared/ui/workspace/catalog/workspace-icon-frame";
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
              <UiChoiceButton
                active={isActive}
                choiceSize="lg"
                className="min-h-[72px] min-w-0 flex-col items-stretch justify-start gap-0 text-left"
                key={mode.value}
                onClick={() => onPermissionModeChange(mode.value)}
                tone="neutral"
                type="button"
              >
                <span className={cn("flex w-full min-w-0 items-center gap-2", getUiTypographyClassName({ role: "control", tone: "strong", weight: "semibold" }))}>
                  <span className="min-w-0 flex-1 truncate">{t(mode.labelKey)}</span>
                  {isActive ? <Check className="h-3.5 w-3.5 shrink-0" /> : null}
                </span>
                <span
                  className={cn("mt-1 line-clamp-2", getUiTypographyClassName({ role: "caption", tone: "muted" }))}
                  title={t(mode.descriptionKey)}
                >
                  {t(mode.descriptionKey)}
                </span>
              </UiChoiceButton>
            );
          })}
        </div>
        {isBypassPermissionMode ? (
          <UiInlineNotice
            icon={<TriangleAlert />}
            message={t("agent_options.advanced.bypass_warning")}
            tone="warning"
          />
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
            <Loader2
              className={getUiSpinnerClassName({ size: "md", tone: "muted" })}
            />
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
          <p className={getUiTypographyClassName({ role: "caption", tone: "soft" })}>
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
        <h3 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>{title}</h3>
        <p className={cn("mt-0.5 max-w-[720px]", getUiTypographyClassName({ role: "metadata", tone: "soft" }))}>
          {description}
        </p>
      </div>
      {trailing ? (
        <span className={cn("shrink-0 tabular-nums sm:pt-0.5", getUiTypographyClassName({ role: "caption", tone: "soft" }))}>
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
    <UiListRow
      active={checked}
      activeTone="sidebar"
      description={description}
      leading={<WorkspaceIconFrame size="sm"><Icon className="h-4 w-4" /></WorkspaceIconFrame>}
      right={<GlassSwitch aria-label={name} checked={checked} onChange={onToggle} size="xs" />}
      title={name}
      tooltip={description}
      variant="outlined"
    />
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
  const disabled = !connected && !checked;
  return (
    <UiListRow
      active={checked}
      activeTone="sidebar"
      description={connected ? connector.description : t("agent_options.advanced.connector_disconnected")}
      disabled={disabled}
      leading={<ConnectorIcon icon={connector.icon} title={connector.title} />}
      right={<GlassSwitch aria-label={connector.title} checked={checked} disabled={disabled} onChange={onToggle} size="xs" />}
      title={connector.title}
      variant="outlined"
    />
  );
}
