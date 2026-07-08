"use client";

import { useState } from "react";
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

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/badge";
import { UiButton } from "@/shared/ui/button";
import { getUiButtonClassName } from "@/shared/ui/button-styles";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { WORKSPACE_DETAIL_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { UiListRow } from "@/shared/ui/list-row";
import { UiPanel } from "@/shared/ui/panel";
import { UiStateBlock } from "@/shared/ui/state-block";
import type { ConnectorDetail, ConnectorFeatureDetail } from "@/types/capability/connector";

import { isDirectCredentialAuth } from "./connector-auth";
import { ConnectorIcon } from "./connector-icon";
import { getConnectorCategoryLabel } from "./connectors-categories";

interface ConnectorDetailViewProps {
  detail: ConnectorDetail | null;
  loading: boolean;
  busy: boolean;
  onBack: () => void;
  onConnect: (connectorId: string) => void;
  onDisconnect: (connectorId: string) => void;
  onConfigureCredential: (detail: ConnectorDetail) => void;
  onConfigureOauthClient: (detail: ConnectorDetail) => void;
}

function getConnectorAuthLabel(authType: ConnectorDetail["auth_type"]): string {
  if (authType === "oauth2") return "OAuth 2.0";
  if (authType === "api_key") return "API Key";
  if (authType === "token") return "Token";
  return "无需授权";
}

function getConnectorFeatureDetails(detail: ConnectorDetail): ConnectorFeatureDetail[] {
  if (!detail.feature_details || detail.feature_details.length === 0) {
    return [];
  }

  const detailByName = new Map(detail.feature_details.map((feature) => [feature.name, feature]));
  if (detail.features.length === 0) {
    return detail.feature_details;
  }
  return detail.features.map((name) => detailByName.get(name)).filter((feature): feature is ConnectorFeatureDetail => Boolean(feature));
}

/** 连接器详情页 —— 一级应用点击后进入完整页面，不使用弹窗承载主体内容。 */
export function ConnectorDetailView({
  detail,
  loading,
  busy,
  onBack: onBack,
  onConnect: onConnect,
  onDisconnect: onDisconnect,
  onConfigureCredential: onConfigureCredential,
  onConfigureOauthClient: onConfigureOauthClient,
}: ConnectorDetailViewProps) {
  const { t } = useI18n();
  const [selectedFeature, setSelectedFeature] = useState<string | null>(null);
  const isConnected = detail?.connection_state === "connected";
  const isComingSoon = detail?.status === "coming_soon";
  const isConfigured = detail?.is_configured ?? true;
  const requiresOauthClientConfig = detail?.oauth_client_config_required ?? false;
  const oauthClientConfigured = detail?.oauth_client_configured ?? false;
  const canConnect = detail && !isConnected && !isComingSoon && isConfigured;
  const requiresDirectCredential = isDirectCredentialAuth(detail?.auth_type);
  const featureDetails = detail ? getConnectorFeatureDetails(detail) : [];
  const selectedFeatureDetail = featureDetails.find((feature) => feature.name === selectedFeature);

  return (
    <div className={WORKSPACE_DETAIL_PAGE_CLASS_NAME}>
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
            <span className="truncate font-medium text-(--text-strong)">{detail.title}</span>
          </>
        ) : null}
      </div>

      {loading ? (
        <UiStateBlock className="min-h-[420px]" size="md" title="加载连接器详情中..." variant="plain" />
      ) : !detail ? (
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
      ) : (
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
              {requiresOauthClientConfig && !isConnected ? (
                <UiButton
                  disabled={busy}
                  onClick={() => onConfigureOauthClient(detail)}
                  size="sm"
                  tone={oauthClientConfigured ? "default" : "primary"}
                  type="button"
                  variant={oauthClientConfigured ? "surface" : "solid"}
                >
                  <KeyRound className="h-3.5 w-3.5" />
                  配置应用
                </UiButton>
              ) : null}
              {isConnected ? (
                <UiButton
                  disabled={busy}
                  onClick={() => onDisconnect(detail.connector_id)}
                  size="sm"
                  type="button"
                >
                  <Unplug className="h-3.5 w-3.5" />
                  断开连接
                </UiButton>
              ) : canConnect ? (
                <UiButton
                  disabled={busy}
                  onClick={() => {
                    if (requiresDirectCredential) {
                      onConfigureCredential(detail);
                      return;
                    }
                    onConnect(detail.connector_id);
                  }}
                  size="sm"
                  tone="primary"
                  type="button"
                  variant="solid"
                >
                  {requiresDirectCredential ? (
                    <KeyRound className="h-3.5 w-3.5" />
                  ) : (
                    <Link2 className="h-3.5 w-3.5" />
                  )}
                  {requiresDirectCredential ? "配置凭证" : "添加到 Nexus"}
                </UiButton>
              ) : isComingSoon ? (
                <UiButton disabled size="sm" type="button">
                  即将推出
                </UiButton>
              ) : requiresOauthClientConfig ? null : (
                <UiButton disabled size="sm" type="button">
                  <Shield className="h-3.5 w-3.5" />
                  后端未配置
                </UiButton>
              )}
            </div>
          </div>

          <div className="mt-8 space-y-6">
            <p className="text-[15px] leading-7 text-(--text-default)">
              连接后，Agent 会通过安全的 MCP 协议访问此应用。你可以在需要时断开连接，OAuth 类型连接器也可以在原应用侧撤销授权。
            </p>

            <div className="flex flex-wrap gap-2">
              {isConnected ? (
                <UiBadge tone="success">
                  <Check className="h-3.5 w-3.5" />
                  已连接
                </UiBadge>
              ) : isComingSoon ? (
                <UiBadge>即将推出</UiBadge>
              ) : !isConfigured ? (
                <UiBadge tone="warning">
                  {requiresOauthClientConfig ? "待配置应用" : "后端未配置"}
                </UiBadge>
              ) : (
                <UiBadge>未连接</UiBadge>
              )}
              <UiBadge>{getConnectorAuthLabel(detail.auth_type)}</UiBadge>
              <UiBadge>{getConnectorCategoryLabel(detail.category, t)}</UiBadge>
              {detail.scopes.length > 0 ? <UiBadge>{detail.scopes.length} 项权限范围</UiBadge> : null}
            </div>

            {!isConnected && !isComingSoon && !isConfigured && detail.config_error && !requiresOauthClientConfig ? (
              <UiStateBlock description={detail.config_error} size="sm" title="配置不可用" tone="danger" />
            ) : null}

            {featureDetails.length > 0 ? (
              <section>
                <h2 className="mb-3 text-[16px] font-semibold tracking-[-0.025em] text-(--text-strong)">
                  包含内容
                </h2>
                <UiPanel className="divide-y divide-(--divider-subtle-color)" padding="none" radius="md" variant="inset">
                  {featureDetails.map((feature) => (
                    <UiListRow
                      key={feature.name}
                      className="rounded-none"
                      description={feature.description}
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
                className={getUiButtonClassName({ size: "sm", variant: "text" }, "w-fit")}
                href={detail.docs_url}
                rel="noopener noreferrer"
                target="_blank"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                查看文档
              </a>
            ) : null}
          </div>

          {selectedFeatureDetail ? (
            <UiDialogPortal>
              <UiDialogBackdrop
                className="z-[9999]"
                onClose={() => setSelectedFeature(null)}
              >
                <UiDialogShell className="max-h-[min(84vh,640px)]" size="lg">
                  <UiDialogHeader
                    icon={<Check className="h-4 w-4" />}
                    onClose={() => setSelectedFeature(null)}
                    subtitle={`${detail.title} 能力`}
                    title={selectedFeatureDetail.name}
                  />
                  <UiDialogBody className="space-y-4" scrollable>
                    <p className="text-[14px] leading-7 text-(--text-default)">
                      {selectedFeatureDetail.description}
                    </p>

                    {selectedFeatureDetail.items && selectedFeatureDetail.items.length > 0 ? (
                      <UiPanel padding="sm" radius="sm" variant="inset">
                        <div className="mb-2 text-[12px] font-semibold text-(--text-strong)">能力范围</div>
                        <div className="space-y-2">
                          {selectedFeatureDetail.items.map((item) => (
                            <div key={item} className="flex gap-2 text-[13px] leading-6 text-(--text-default)">
                              <Check className="mt-1 h-3.5 w-3.5 shrink-0 text-(--primary)" />
                              <span>{item}</span>
                            </div>
                          ))}
                        </div>
                      </UiPanel>
                    ) : null}

                    {selectedFeatureDetail.scopes && selectedFeatureDetail.scopes.length > 0 ? (
                      <div>
                        <div className="mb-2 text-[12px] font-medium text-(--text-muted)">相关 OAuth scopes</div>
                        <div className="flex flex-wrap gap-1.5">
                          {selectedFeatureDetail.scopes.map((scope) => (
                            <UiBadge key={scope} size="xs">
                              {scope}
                            </UiBadge>
                          ))}
                        </div>
                      </div>
                    ) : null}
                  </UiDialogBody>
                </UiDialogShell>
              </UiDialogBackdrop>
            </UiDialogPortal>
          ) : null}
        </div>
      )}
    </div>
  );
}
