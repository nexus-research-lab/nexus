"use client";

import type { ReactNode } from "react";
import {
  ArrowLeft,
  Check,
  ChevronRight,
  ExternalLink,
  KeyRound,
  Link2,
  Shield,
  Unplug,
} from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiButton } from "@/shared/ui/button/button";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";
import { WORKSPACE_DETAIL_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiPanel } from "@/shared/ui/panel";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import type { ConnectorDetail } from "@/types/capability/connector";

import { ConnectorIcon } from "../connector-icon";
import { getConnectorCategoryLabel } from "../catalog/connectors-categories";
import { ConnectorFeatureDialog } from "./connector-feature-dialog";
import {
  getConnectorAuthLabel,
  getConnectorDetailState,
  getConnectorFeatureDetails,
  type ConnectorPrimaryAction,
  type ConnectorStatusTone,
} from "./connector-detail-model";

interface ConnectorDetailViewProps {
  busy: boolean;
  detail: ConnectorDetail | null;
  loading: boolean;
  onBack: () => void;
  onConfigureCredential: (detail: ConnectorDetail) => void;
  onConfigureOauthClient: (detail: ConnectorDetail) => void;
  onConnect: (connectorId: string) => void;
  onDisconnect: (connectorId: string) => void;
}

/** 连接器详情页承载完整应用信息，配置表单只作为附属弹窗出现。 */
export function ConnectorDetailView({
  busy,
  detail,
  loading,
  onBack,
  onConfigureCredential,
  onConfigureOauthClient,
  onConnect,
  onDisconnect,
}: ConnectorDetailViewProps) {
  const { t } = useI18n();
  const [selectedFeature, setSelectedFeature] = useResettableState<string | null>(
    null,
    detail?.connector_id ?? null,
  );

  if (loading) {
    return (
      <div className={WORKSPACE_DETAIL_PAGE_CLASS_NAME}>
        <ConnectorBreadcrumb detail={detail} onBack={onBack} />
        <UiStateBlock
          className="min-h-[420px]"
          size="md"
          title="加载连接器详情中..."
          variant="plain"
        />
      </div>
    );
  }
  if (!detail) {
    return (
      <div className={WORKSPACE_DETAIL_PAGE_CLASS_NAME}>
        <ConnectorBreadcrumb detail={null} onBack={onBack} />
        <UiStateBlock
          actions={(
            <UiButton onClick={onBack} size="sm" type="button">
              返回连接器
            </UiButton>
          )}
          className="min-h-[420px]"
          size="md"
          title="连接器不存在"
          variant="plain"
        />
      </div>
    );
  }

  const state = getConnectorDetailState(detail);
  const featureDetails = getConnectorFeatureDetails(detail);
  const selectedFeatureDetail = featureDetails.find((feature) => (
    feature.name === selectedFeature
  )) ?? null;
  const primaryActions: Record<ConnectorPrimaryAction, ReactNode> = {
    connect: (
      <UiButton
        disabled={busy}
        onClick={() => onConnect(detail.connector_id)}
        size="sm"
        tone="primary"
        type="button"
        variant="solid"
      >
        <Link2 className="h-3.5 w-3.5" />
        添加到 Nexus
      </UiButton>
    ),
    "configure-credential": (
      <UiButton
        disabled={busy}
        onClick={() => onConfigureCredential(detail)}
        size="sm"
        tone="primary"
        type="button"
        variant="solid"
      >
        <KeyRound className="h-3.5 w-3.5" />
        配置凭证
      </UiButton>
    ),
    disconnect: (
      <UiButton
        disabled={busy}
        onClick={() => onDisconnect(detail.connector_id)}
        size="sm"
        type="button"
      >
        <Unplug className="h-3.5 w-3.5" />
        断开连接
      </UiButton>
    ),
    "coming-soon": (
      <UiButton disabled size="sm" type="button">
        即将推出
      </UiButton>
    ),
    unavailable: (
      <UiButton disabled size="sm" type="button">
        <Shield className="h-3.5 w-3.5" />
        后端未配置
      </UiButton>
    ),
    none: null,
  };
  const statusBadges: Record<ConnectorStatusTone, ReactNode> = {
    connected: (
      <UiBadge tone="success">
        <Check className="h-3.5 w-3.5" />
        已连接
      </UiBadge>
    ),
    "coming-soon": <UiBadge>即将推出</UiBadge>,
    unconfigured: (
      <UiBadge tone="warning">
        {state.requiresOauthClientConfig ? "待配置应用" : "后端未配置"}
      </UiBadge>
    ),
    disconnected: <UiBadge>未连接</UiBadge>,
  };

  return (
    <div className={WORKSPACE_DETAIL_PAGE_CLASS_NAME}>
      <ConnectorBreadcrumb detail={detail} onBack={onBack} />
      <div className="pt-9">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex min-w-0 items-start gap-4">
            <ConnectorIcon icon={detail.icon} size="lg" title={detail.title} />
            <div className="min-w-0">
              <h1 className="text-[24px] font-semibold tracking-[-0.035em] text-(--text-strong)">
                {detail.title}{" "}
                <span className="ml-2 font-normal text-(--text-muted)">App</span>
              </h1>
              <p className="mt-2 text-[15px] leading-6 text-(--text-muted)">
                {detail.description}
              </p>
            </div>
          </div>
          <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
            {state.requiresOauthClientConfig && state.status !== "connected" ? (
              <UiButton
                disabled={busy}
                onClick={() => onConfigureOauthClient(detail)}
                size="sm"
                tone={state.oauthClientConfigured ? "default" : "primary"}
                type="button"
                variant={state.oauthClientConfigured ? "surface" : "solid"}
              >
                <KeyRound className="h-3.5 w-3.5" />
                配置应用
              </UiButton>
            ) : null}
            {primaryActions[state.primaryAction]}
          </div>
        </div>

        <div className="mt-8 space-y-6">
          <p className="text-[15px] leading-7 text-(--text-default)">
            连接后，Agent 会通过安全的 MCP 协议访问此应用。你可以在需要时断开连接，OAuth 类型连接器也可以在原应用侧撤销授权。
          </p>
          <div className="flex flex-wrap gap-2">
            {statusBadges[state.status]}
            <UiBadge>{getConnectorAuthLabel(detail.auth_type)}</UiBadge>
            <UiBadge>{getConnectorCategoryLabel(detail.category, t)}</UiBadge>
            {detail.scopes.length > 0 ? (
              <UiBadge>{detail.scopes.length} 项权限范围</UiBadge>
            ) : null}
          </div>
          {state.status === "unconfigured"
            && detail.config_error
            && !state.requiresOauthClientConfig ? (
              <UiStateBlock
                description={detail.config_error}
                size="sm"
                title="配置不可用"
                tone="danger"
              />
            ) : null}
          {featureDetails.length > 0 ? (
            <section>
              <h2 className="mb-3 text-[16px] font-semibold tracking-[-0.025em] text-(--text-strong)">
                包含内容
              </h2>
              <UiPanel
                className="divide-y divide-(--divider-subtle-color)"
                padding="none"
                radius="md"
                variant="inset"
              >
                {featureDetails.map((feature) => (
                  <UiListRow
                    className="rounded-none"
                    description={feature.description}
                    key={feature.name}
                    leading={(
                      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-(--divider-subtle-color) bg-(--surface-panel-background)">
                        <Check className="h-4 w-4 text-(--icon-muted)" />
                      </span>
                    )}
                    onClick={() => setSelectedFeature(feature.name)}
                    right={<ChevronRight className="h-4 w-4 shrink-0 text-(--icon-muted)" />}
                    title={feature.name}
                  />
                ))}
              </UiPanel>
            </section>
          ) : null}
          {detail.docs_url ? (
            <a
              className={getUiButtonClassName(
                { size: "sm", variant: "text" },
                "w-fit",
              )}
              href={detail.docs_url}
              rel="noopener noreferrer"
              target="_blank"
            >
              <ExternalLink className="h-3.5 w-3.5" />
              查看文档
            </a>
          ) : null}
        </div>
      </div>
      <ConnectorFeatureDialog
        connectorTitle={detail.title}
        feature={selectedFeatureDetail}
        onClose={() => setSelectedFeature(null)}
      />
    </div>
  );
}

function ConnectorBreadcrumb({
  detail,
  onBack,
}: {
  detail: ConnectorDetail | null;
  onBack: () => void;
}) {
  return (
    <div className="flex items-center gap-2 text-[14px] text-(--text-muted)">
      <button
        className="inline-flex items-center gap-1 rounded-full px-2 py-1 font-medium transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]"
        onClick={onBack}
        type="button"
      >
        <ArrowLeft className="h-3.5 w-3.5" />
        连接器
      </button>
      {detail ? (
        <>
          <ChevronRight className="h-3.5 w-3.5 text-(--icon-muted)" />
          <span className="truncate font-medium text-(--text-strong)">
            {detail.title}
          </span>
        </>
      ) : null}
    </div>
  );
}
