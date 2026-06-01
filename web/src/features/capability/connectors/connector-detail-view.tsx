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
import { get_ui_button_class_name } from "@/shared/ui/button-styles";
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

import { is_direct_credential_auth } from "./connector-auth";
import { ConnectorIcon } from "./connector-icon";
import { get_connector_category_label } from "./connectors-categories";

interface ConnectorDetailViewProps {
  detail: ConnectorDetail | null;
  loading: boolean;
  busy: boolean;
  on_back: () => void;
  on_connect: (connector_id: string) => void;
  on_disconnect: (connector_id: string) => void;
  on_configure_credential: (detail: ConnectorDetail) => void;
  on_configure_oauth_client: (detail: ConnectorDetail) => void;
}

function get_connector_auth_label(auth_type: ConnectorDetail["auth_type"]): string {
  if (auth_type === "oauth2") return "OAuth 2.0";
  if (auth_type === "api_key") return "API Key";
  if (auth_type === "token") return "Token";
  return "无需授权";
}

function get_connector_feature_details(detail: ConnectorDetail): ConnectorFeatureDetail[] {
  if (!detail.feature_details || detail.feature_details.length === 0) {
    return [];
  }

  const detail_by_name = new Map(detail.feature_details.map((feature) => [feature.name, feature]));
  if (detail.features.length === 0) {
    return detail.feature_details;
  }
  return detail.features.map((name) => detail_by_name.get(name)).filter((feature): feature is ConnectorFeatureDetail => Boolean(feature));
}

/** 连接器详情页 —— 一级应用点击后进入完整页面，不使用弹窗承载主体内容。 */
export function ConnectorDetailView({
  detail,
  loading,
  busy,
  on_back,
  on_connect,
  on_disconnect,
  on_configure_credential,
  on_configure_oauth_client,
}: ConnectorDetailViewProps) {
  const { t } = useI18n();
  const [selected_feature, set_selected_feature] = useState<string | null>(null);
  const is_connected = detail?.connection_state === "connected";
  const is_coming_soon = detail?.status === "coming_soon";
  const is_configured = detail?.is_configured ?? true;
  const requires_oauth_client_config = detail?.oauth_client_config_required ?? false;
  const oauth_client_configured = detail?.oauth_client_configured ?? false;
  const can_connect = detail && !is_connected && !is_coming_soon && is_configured;
  const requires_direct_credential = is_direct_credential_auth(detail?.auth_type);
  const feature_details = detail ? get_connector_feature_details(detail) : [];
  const selected_feature_detail = feature_details.find((feature) => feature.name === selected_feature);

  return (
    <div className={WORKSPACE_DETAIL_PAGE_CLASS_NAME}>
      <div className="flex items-center gap-2 text-[14px] text-(--text-muted)">
        <button
          className="inline-flex items-center gap-1 rounded-full px-2 py-1 font-medium transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]"
          onClick={on_back}
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
        <UiStateBlock class_name="min-h-[420px]" size="md" title="加载连接器详情中..." variant="plain" />
      ) : !detail ? (
        <UiStateBlock
          actions={(
            <UiButton onClick={on_back} size="sm" type="button">
              返回连接器
            </UiButton>
          )}
          class_name="min-h-[420px]"
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
              {requires_oauth_client_config && !is_connected ? (
                <UiButton
                  disabled={busy}
                  onClick={() => on_configure_oauth_client(detail)}
                  size="sm"
                  tone={oauth_client_configured ? "default" : "primary"}
                  type="button"
                  variant={oauth_client_configured ? "surface" : "solid"}
                >
                  <KeyRound className="h-3.5 w-3.5" />
                  配置应用
                </UiButton>
              ) : null}
              {is_connected ? (
                <UiButton
                  disabled={busy}
                  onClick={() => on_disconnect(detail.connector_id)}
                  size="sm"
                  type="button"
                >
                  <Unplug className="h-3.5 w-3.5" />
                  断开连接
                </UiButton>
              ) : can_connect ? (
                <UiButton
                  disabled={busy}
                  onClick={() => {
                    if (requires_direct_credential) {
                      on_configure_credential(detail);
                      return;
                    }
                    on_connect(detail.connector_id);
                  }}
                  size="sm"
                  tone="primary"
                  type="button"
                  variant="solid"
                >
                  {requires_direct_credential ? (
                    <KeyRound className="h-3.5 w-3.5" />
                  ) : (
                    <Link2 className="h-3.5 w-3.5" />
                  )}
                  {requires_direct_credential ? "配置凭证" : "添加到 Nexus"}
                </UiButton>
              ) : is_coming_soon ? (
                <UiButton disabled size="sm" type="button">
                  即将推出
                </UiButton>
              ) : requires_oauth_client_config ? null : (
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
              {is_connected ? (
                <UiBadge tone="success">
                  <Check className="h-3.5 w-3.5" />
                  已连接
                </UiBadge>
              ) : is_coming_soon ? (
                <UiBadge>即将推出</UiBadge>
              ) : !is_configured ? (
                <UiBadge tone="warning">
                  {requires_oauth_client_config ? "待配置应用" : "后端未配置"}
                </UiBadge>
              ) : (
                <UiBadge>未连接</UiBadge>
              )}
              <UiBadge>{get_connector_auth_label(detail.auth_type)}</UiBadge>
              <UiBadge>{get_connector_category_label(detail.category, t)}</UiBadge>
              {detail.scopes.length > 0 ? <UiBadge>{detail.scopes.length} 项权限范围</UiBadge> : null}
            </div>

            {!is_connected && !is_coming_soon && !is_configured && detail.config_error && !requires_oauth_client_config ? (
              <UiStateBlock description={detail.config_error} size="sm" title="配置不可用" tone="danger" />
            ) : null}

            {feature_details.length > 0 ? (
              <section>
                <h2 className="mb-3 text-[16px] font-semibold tracking-[-0.025em] text-(--text-strong)">
                  包含内容
                </h2>
                <UiPanel class_name="divide-y divide-(--divider-subtle-color)" padding="none" radius="md" variant="inset">
                  {feature_details.map((feature) => (
                    <UiListRow
                      key={feature.name}
                      class_name="rounded-none"
                      description={feature.description}
                      leading={(
                        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-(--divider-subtle-color) bg-(--surface-panel-background)">
                          <Check className="h-4 w-4 text-(--icon-muted)" />
                        </span>
                      )}
                      on_click={() => set_selected_feature(feature.name)}
                      right={<ChevronRight className="h-4 w-4 shrink-0 text-(--icon-muted)" />}
                      title={feature.name}
                    />
                  ))}
                </UiPanel>
              </section>
            ) : null}

            {detail.docs_url ? (
              <a
                className={get_ui_button_class_name({ size: "sm", variant: "text" }, "w-fit")}
                href={detail.docs_url}
                rel="noopener noreferrer"
                target="_blank"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                查看文档
              </a>
            ) : null}
          </div>

          {selected_feature_detail ? (
            <UiDialogPortal>
              <UiDialogBackdrop
                class_name="z-[9999]"
                on_close={() => set_selected_feature(null)}
              >
                <UiDialogShell class_name="max-h-[min(84vh,640px)]" size="lg">
                  <UiDialogHeader
                    icon={<Check className="h-4 w-4" />}
                    on_close={() => set_selected_feature(null)}
                    subtitle={`${detail.title} 能力`}
                    title={selected_feature_detail.name}
                  />
                  <UiDialogBody class_name="space-y-4" scrollable>
                    <p className="text-[14px] leading-7 text-(--text-default)">
                      {selected_feature_detail.description}
                    </p>

                    {selected_feature_detail.items && selected_feature_detail.items.length > 0 ? (
                      <UiPanel padding="sm" radius="sm" variant="inset">
                        <div className="mb-2 text-[12px] font-semibold text-(--text-strong)">能力范围</div>
                        <div className="space-y-2">
                          {selected_feature_detail.items.map((item) => (
                            <div key={item} className="flex gap-2 text-[13px] leading-6 text-(--text-default)">
                              <Check className="mt-1 h-3.5 w-3.5 shrink-0 text-(--primary)" />
                              <span>{item}</span>
                            </div>
                          ))}
                        </div>
                      </UiPanel>
                    ) : null}

                    {selected_feature_detail.scopes && selected_feature_detail.scopes.length > 0 ? (
                      <div>
                        <div className="mb-2 text-[12px] font-medium text-(--text-muted)">相关 OAuth scopes</div>
                        <div className="flex flex-wrap gap-1.5">
                          {selected_feature_detail.scopes.map((scope) => (
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
