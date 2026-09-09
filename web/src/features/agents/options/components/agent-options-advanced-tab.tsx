/**
 * INPUT: Agent 权限模式和 Connector 选择状态。
 * OUTPUT: Connector 配置与默认折叠、运行时感知的独立权限设置。
 * POS: Agent 详情中的授权决策面；不提供工具预授权入口。
 */

"use client";

import { useId } from "react";
import { Link } from "react-router-dom";
import { Check, Loader2, TriangleAlert } from "lucide-react";

import { useDefaultAgentRuntimeKind } from "@/hooks/settings/use-default-agent-runtime-kind";
import {
  getAgentPermissionChoices,
  resolveRuntimePermissionMode,
} from "@/lib/agent-options";
import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { cn } from "@/shared/ui/class-name";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiListRow } from "@/shared/ui/list/list-row";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { ConnectorIcon } from "@/features/capability/connectors/connector-icon";
import type { ConnectorInfo } from "@/types/capability/connector";

interface AgentOptionsAdvancedTabProps {
  permissionMode: string;
  onPermissionModeChange: (mode: string) => void;
  connectorIds: string[];
  connectors: ConnectorInfo[];
  connectorsError: string | null;
  connectorsLoading: boolean;
  onRetryConnectors: () => void;
  onToggleConnector: (connectorId: string) => void;
}

/** Connector 是常用能力；危险权限控制默认折叠，避免误触。 */
export function AgentOptionsAdvancedTab({
  permissionMode,
  onPermissionModeChange,
  connectorIds,
  connectors,
  connectorsError,
  connectorsLoading,
  onRetryConnectors,
  onToggleConnector,
}: AgentOptionsAdvancedTabProps) {
  const { t } = useI18n();
  const runtimeKind = useDefaultAgentRuntimeKind();
  const effectivePermissionMode = resolveRuntimePermissionMode(permissionMode, runtimeKind);
  const permissionGroupId = useId();
  const AGENT_PERMISSION_MODES = getAgentPermissionChoices(runtimeKind);
  const isBypassPermissionMode = effectivePermissionMode === "bypassPermissions";

  return (
    <div className="space-y-6 animate-in fade-in duration-200 [overflow-anchor:none]">
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
            <Loader2 className={getUiSpinnerClassName({ size: "md", tone: "muted" })} />
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

      <UiDisclosure
        contentClassName="space-y-3"
        label={t("agent_options.advanced.permission_settings")}
        variant="panel"
      >
        <SectionHeader
          description={t("agent_options.advanced.permission_control_hint")}
          title={t("agent_options.advanced.permission_control")}
        />
        <div className="grid grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-2">
          {AGENT_PERMISSION_MODES.map((mode) => {
            const isActive = effectivePermissionMode === mode.value;
            const titleId = `${permissionGroupId}-${mode.value}-title`;
            const descriptionId = `${permissionGroupId}-${mode.value}-description`;
            return (
              <UiChoiceButton
                active={isActive}
                aria-describedby={descriptionId}
                aria-labelledby={titleId}
                choiceSize="lg"
                className="min-h-[72px] min-w-0 flex-col items-stretch justify-start gap-0 text-left"
                data-agent-permission-mode={mode.value}
                key={mode.value}
                onClick={() => onPermissionModeChange(mode.value)}
                tone="neutral"
              >
                <span className={cn(
                  "flex w-full min-w-0 items-center gap-2",
                  getUiTypographyClassName({ role: "control", tone: "strong", weight: "medium" }),
                )}>
                  <span className="min-w-0 flex-1 break-words" id={titleId}>
                    {t(mode.labelKey)}
                  </span>
                  {isActive ? <Check aria-hidden className="h-3.5 w-3.5 shrink-0" /> : null}
                </span>
                <span
                  className={cn(
                    "mt-1 break-words",
                    getUiTypographyClassName({ role: "supporting", tone: "muted" }),
                  )}
                  id={descriptionId}
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
      </UiDisclosure>
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
        <h3 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
          {title}
        </h3>
        <p className={cn(
          "mt-0.5 max-w-[720px]",
          getUiTypographyClassName({ role: "metadata", tone: "soft" }),
        )}>
          {description}
        </p>
      </div>
      {trailing ? (
        <span className={cn(
          "shrink-0 tabular-nums sm:pt-0.5",
          getUiTypographyClassName({ role: "caption", tone: "soft" }),
        )}>
          {trailing}
        </span>
      ) : null}
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
  const disabled = !connected && !checked;
  return (
    <UiListRow
      active={checked}
      activeTone="sidebar"
      muted={disabled}
      right={(
        <GlassSwitch
          aria-label={connector.title}
          checked={checked}
          disabled={disabled}
          onChange={onToggle}
          size="xs"
        />
      )}
      variant="outlined"
    >
      <Link
        aria-label={connector.title}
        className="grid min-w-0 flex-1 grid-cols-[30px_minmax(0,1fr)] items-center gap-2.5 radius-control-md focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]"
        to={AppRouteBuilders.connectorDetail(connector.connector_id)}
      >
        <ConnectorIcon
          className="h-[30px] w-[30px]"
          icon={connector.icon}
          title={connector.title}
        />
        <span className="min-w-0">
          <span className={cn(
            "block truncate hover:underline",
            getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
          )}>
            {connector.title}
          </span>
          <span className={cn(
            "mt-0.5 block truncate",
            getUiTypographyClassName({ role: "metadata", tone: "muted" }),
          )}>
            {connected
              ? connector.description
              : t("agent_options.advanced.connector_disconnected")}
          </span>
        </span>
      </Link>
    </UiListRow>
  );
}
