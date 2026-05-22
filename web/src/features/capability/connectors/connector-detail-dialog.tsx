"use client";

import { Check, ExternalLink, KeyRound, Link2, Shield, Unplug } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiBadge } from "@/shared/ui/badge";
import { UiButton } from "@/shared/ui/button";
import { get_ui_button_class_name } from "@/shared/ui/button-styles";
import { UiPanel } from "@/shared/ui/panel";
import { UiStateBlock } from "@/shared/ui/state-block";
import type { ConnectorDetail } from "@/types/capability/connector";

import { get_connector_colors, get_connector_letter } from "./connector-icons";

interface ConnectorDetailDialogProps {
  detail: ConnectorDetail | null;
  loading: boolean;
  busy: boolean;
  on_close: () => void;
  on_connect: (connector_id: string) => void;
  on_disconnect: (connector_id: string) => void;
  on_configure_oauth_client: (detail: ConnectorDetail) => void;
}

/** 连接器详情弹窗 */
export function ConnectorDetailDialog({
  detail,
  loading,
  busy,
  on_close,
  on_connect,
  on_disconnect,
  on_configure_oauth_client,
}: ConnectorDetailDialogProps) {
  const colors = detail ? get_connector_colors(detail.icon) : { bg: "bg-(--surface-panel-subtle-background)", text: "text-(--text-muted)" };
  const letter = detail ? get_connector_letter(detail.icon, detail.title) : "?";
  const is_connected = detail?.connection_state === "connected";
  const is_coming_soon = detail?.status === "coming_soon";
  const is_configured = detail?.is_configured ?? true;
  const requires_oauth_client_config = detail?.oauth_client_config_required ?? false;
  const oauth_client_configured = detail?.oauth_client_configured ?? false;

  if (!detail && !loading) return null;

  return (
    <UiDialogBackdrop on_close={on_close}>
      <UiDialogShell class_name="max-h-[84vh]" size="md">
        <UiDialogHeader on_close={on_close}>
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div
              className={cn(
                "dialog-card flex shrink-0 items-center justify-center",
                "h-14 w-14 rounded-[20px] border border-white/50 text-base font-bold",
                colors.bg,
                colors.text,
              )}
            >
              {letter}
            </div>
            <div className="min-w-0 flex-1">
              {loading ? (
                <div className="h-5 w-32 animate-pulse rounded bg-(--surface-panel-subtle-background)" />
              ) : (
                <>
                  <h2 className="dialog-title" data-size="hero">
                    {detail?.title}
                  </h2>
                  <p className="dialog-subtitle">{detail?.description}</p>
                </>
              )}
            </div>
          </div>
        </UiDialogHeader>

        <UiDialogBody scrollable>
          {loading ? (
            <UiStateBlock size="sm" title="加载中..." variant="plain" />
          ) : detail ? (
            <div className="space-y-5">
              <div className="flex flex-wrap gap-2">
                {is_connected ? (
                  <UiBadge tone="success">
                    <Check className="h-3.5 w-3.5" />
                    已连接
                  </UiBadge>
                ) : is_coming_soon ? (
                  <UiBadge>
                    即将推出
                  </UiBadge>
                ) : !is_configured ? (
                  <UiBadge tone="warning">
                    {requires_oauth_client_config ? "待配置应用" : "后端未配置"}
                  </UiBadge>
                ) : (
                  <UiBadge>
                    未连接
                  </UiBadge>
                )}
                <UiBadge>
                  {detail.auth_type === "oauth2" ? "OAuth 2.0" : detail.auth_type === "api_key" ? "API Key" : detail.auth_type}
                </UiBadge>
                <UiBadge>
                  {detail.category}
                </UiBadge>
              </div>

              {detail.features.length > 0 && (
                <div>
                  <h3 className="mb-2 text-[13px] font-semibold text-(--text-default)">支持的功能</h3>
                  <div className="grid grid-cols-2 gap-2">
                    {detail.features.map((feature) => (
                      <UiPanel
                        key={feature}
                        class_name="flex items-center gap-2 text-[12px] text-(--text-muted)"
                        padding="sm"
                        radius="sm"
                        variant="inset"
                      >
                        <Check className="h-3 w-3 shrink-0 text-(--success)" />
                        {feature}
                      </UiPanel>
                    ))}
                  </div>
                </div>
              )}

              {!is_coming_soon && (
                <UiPanel padding="sm" variant="inset">
                  <div className="flex items-center gap-2 text-[13px] font-medium text-(--text-default)">
                    <Shield className="h-4 w-4" />
                    安全授权
                  </div>
                  <p className="mt-1 text-[12px] leading-relaxed text-(--text-muted)">
                    连接后，Agent 将通过安全的 MCP 协议访问此应用。你可以随时断开连接并撤销授权。
                  </p>
                </UiPanel>
              )}

              {!is_connected && !is_coming_soon && !is_configured && detail.config_error && !requires_oauth_client_config ? (
                <UiStateBlock description={detail.config_error} size="sm" title="配置不可用" tone="danger" />
              ) : null}

              {detail.docs_url && (
                <a
                  className={get_ui_button_class_name({ size: "sm", variant: "text" }, "w-fit")}
                  href={detail.docs_url}
                  rel="noopener noreferrer"
                  target="_blank"
                >
                  <ExternalLink className="h-3 w-3" />
                  查看文档
                </a>
              )}
            </div>
          ) : null}
        </UiDialogBody>

        {detail && !is_coming_soon && (
          <UiDialogFooter class_name="flex-wrap gap-2">
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
            ) : is_configured ? (
              <UiButton
                disabled={busy}
                onClick={() => on_connect(detail.connector_id)}
                size="sm"
                tone="primary"
                type="button"
                variant="solid"
              >
                <Link2 className="h-3.5 w-3.5" />
                授权连接
              </UiButton>
            ) : requires_oauth_client_config ? null : (
              <UiButton
                disabled
                size="sm"
                type="button"
              >
                <Shield className="h-3.5 w-3.5" />
                后端未配置
              </UiButton>
            )}
          </UiDialogFooter>
        )}
      </UiDialogShell>
    </UiDialogBackdrop>
  );
}
