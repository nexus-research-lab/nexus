"use client";

import { Check, ExternalLink, Link2, Shield, Unplug, X } from "lucide-react";
import { useCallback } from "react";

import { cn } from "@/lib/utils";
import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  DIALOG_TAG_CLASS_NAME,
  getDialogNoteClassName,
} from "@/shared/ui/dialog/dialog-styles";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import { ConnectorDetail } from "@/types/connector";

import { getConnectorColors, getConnectorLetter } from "./connector-icons";

interface ConnectorDetailDialogProps {
  detail: ConnectorDetail | null;
  loading: boolean;
  busy: boolean;
  on_close: () => void;
  on_connect: (connector_id: string) => void;
  on_disconnect: (connector_id: string) => void;
}

/** 连接器详情弹窗 */
export function ConnectorDetailDialog({
  detail,
  loading,
  busy,
  on_close,
  on_connect,
  on_disconnect,
}: ConnectorDetailDialogProps) {
  const handle_backdrop_click = useCallback(
    (e: React.MouseEvent) => {
      if (e.target === e.currentTarget) on_close();
    },
    [on_close],
  );

  const colors = detail ? getConnectorColors(detail.icon) : { bg: "bg-slate-100", text: "text-slate-600" };
  const letter = detail ? getConnectorLetter(detail.icon, detail.title) : "?";
  const is_connected = detail?.connection_state === "connected";
  const is_coming_soon = detail?.status === "coming_soon";
  const is_configured = detail?.is_configured ?? true;

  if (!detail && !loading) return null;

  return (
    <div
      className="dialog-backdrop"
      onClick={handle_backdrop_click}
    >
      <div className="dialog-shell relative flex max-h-[84vh] w-full max-w-lg flex-col overflow-hidden rounded-3xl">
        <div className="dialog-header">
          <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
            <div
              className={cn(
                DIALOG_HEADER_ICON_CLASS_NAME,
                "h-14 w-14 rounded-[20px] border border-white/50 text-base font-bold",
                colors.bg,
                colors.text,
              )}
            >
              {letter}
            </div>
            <div className="min-w-0 flex-1">
            {loading ? (
              <div className="h-5 w-32 animate-pulse rounded bg-slate-200" />
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
          <WorkspacePillButton
            aria-label="关闭"
            density="compact"
            onClick={on_close}
            size="icon"
            variant="icon"
          >
            <X className="h-4 w-4" />
          </WorkspacePillButton>
        </div>

        <div className="dialog-body dialog-body--scroll soft-scrollbar flex-1">
          {loading ? (
            <div className="flex min-h-32 items-center justify-center text-sm text-slate-400">
              加载中…
            </div>
          ) : detail ? (
            <div className="space-y-5">
              <div className="flex flex-wrap gap-2">
                {is_connected ? (
                  <span className={cn(DIALOG_TAG_CLASS_NAME, "text-emerald-700")}>
                    <Check className="h-3.5 w-3.5" />
                    已连接
                  </span>
                ) : is_coming_soon ? (
                  <span className={DIALOG_TAG_CLASS_NAME}>
                    即将推出
                  </span>
                ) : !is_configured ? (
                  <span className={cn(DIALOG_TAG_CLASS_NAME, "text-amber-700")}>
                    待配置
                  </span>
                ) : (
                  <span className={DIALOG_TAG_CLASS_NAME}>
                    未连接
                  </span>
                )}
                <span className={DIALOG_TAG_CLASS_NAME}>
                  {detail.auth_type === "oauth2" ? "OAuth 2.0" : detail.auth_type === "api_key" ? "API Key" : detail.auth_type}
                </span>
                <span className={DIALOG_TAG_CLASS_NAME}>
                  {detail.category}
                </span>
              </div>

              {detail.features.length > 0 && (
                <div>
                  <h3 className="mb-2 text-[13px] font-semibold text-slate-700">支持的功能</h3>
                  <div className="grid grid-cols-2 gap-2">
                    {detail.features.map((f) => (
                      <div
                        key={f}
                        className="surface-card flex items-center gap-2 rounded-xl px-3 py-2 text-[12px] text-slate-600"
                      >
                        <Check className="h-3 w-3 shrink-0 text-emerald-500" />
                        {f}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {!is_coming_soon && (
                <div className={getDialogNoteClassName("default")}>
                  <div className="flex items-center gap-2 text-[13px] font-medium text-slate-700">
                    <Shield className="h-4 w-4" />
                    安全授权
                  </div>
                  <p className="mt-1 text-[12px] leading-relaxed text-slate-600/84">
                    连接后，Agent 将通过安全的 MCP 协议访问此应用。你可以随时断开连接并撤销授权。
                  </p>
                </div>
              )}

              {!is_connected && !is_coming_soon && !is_configured && detail.config_error ? (
                <div className={getDialogNoteClassName("danger")}>
                  {detail.config_error}
                </div>
              ) : null}

              {detail.docs_url && (
                <a
                  className="flex items-center gap-1.5 text-[12px] text-sky-600 hover:underline"
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
        </div>

        {/* 底部操作 */}
        {detail && !is_coming_soon && (
          <div className="dialog-footer flex-wrap gap-2">
            {is_connected ? (
              <WorkspacePillButton
                disabled={busy}
                onClick={() => on_disconnect(detail.connector_id)}
                size="md"
                variant="outlined"
              >
                <Unplug className="h-3.5 w-3.5" />
                断开连接
              </WorkspacePillButton>
            ) : (
              <WorkspacePillButton
                disabled={busy || !is_configured}
                onClick={() => on_connect(detail.connector_id)}
                size="md"
                variant="primary"
              >
                <Link2 className="h-3.5 w-3.5" />
                {is_configured ? "授权连接" : "等待配置"}
              </WorkspacePillButton>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
