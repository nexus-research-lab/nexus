"use client";

import { Check, ExternalLink, Link2, Shield, Unplug, X } from "lucide-react";
import { useCallback } from "react";

import { cn } from "@/lib/utils";
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
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 backdrop-blur-[2px]"
      onClick={handle_backdrop_click}
    >
      <div className="modal-dialog-surface relative flex max-h-[84vh] w-full max-w-lg flex-col overflow-hidden rounded-3xl shadow-xl">
        {/* 头部 */}
        <div className="flex items-start gap-4 border-b border-white/12 px-6 py-5">
          <div
            className={cn(
              "flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-white/50 text-base font-bold",
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
                <h2 className="text-[17px] font-black tracking-[-0.01em] text-slate-900">
                  {detail?.title}
                </h2>
                <p className="mt-0.5 text-[13px] text-slate-500">{detail?.description}</p>
              </>
            )}
          </div>
          <button
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-white/50 hover:text-slate-600"
            onClick={on_close}
            type="button"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* 内容 */}
        <div className="soft-scrollbar flex-1 overflow-y-auto px-6 py-5">
          {loading ? (
            <div className="flex min-h-32 items-center justify-center text-sm text-slate-400">
              加载中…
            </div>
          ) : detail ? (
            <div className="space-y-5">
              {/* 状态标签 */}
              <div className="flex flex-wrap gap-2">
                {is_connected ? (
                  <span className="flex items-center gap-1 rounded-full bg-emerald-50 px-2.5 py-1 text-[12px] font-medium text-emerald-600">
                    <Check className="h-3.5 w-3.5" />
                    已连接
                  </span>
                ) : is_coming_soon ? (
                  <span className="rounded-full bg-slate-100 px-2.5 py-1 text-[12px] font-medium text-slate-400">
                    即将推出
                  </span>
                ) : !is_configured ? (
                  <span className="rounded-full bg-amber-50 px-2.5 py-1 text-[12px] font-medium text-amber-600">
                    待配置
                  </span>
                ) : (
                  <span className="rounded-full bg-slate-100 px-2.5 py-1 text-[12px] font-medium text-slate-500">
                    未连接
                  </span>
                )}
                <span className="rounded-full bg-slate-100 px-2.5 py-1 text-[12px] font-medium text-slate-500">
                  {detail.auth_type === "oauth2" ? "OAuth 2.0" : detail.auth_type === "api_key" ? "API Key" : detail.auth_type}
                </span>
                <span className="rounded-full bg-slate-100 px-2.5 py-1 text-[12px] font-medium text-slate-500">
                  {detail.category}
                </span>
              </div>

              {/* 功能列表 */}
              {detail.features.length > 0 && (
                <div>
                  <h3 className="mb-2 text-[13px] font-semibold text-slate-700">支持的功能</h3>
                  <div className="grid grid-cols-2 gap-2">
                    {detail.features.map((f) => (
                      <div
                        key={f}
                        className="flex items-center gap-2 rounded-xl bg-white/40 px-3 py-2 text-[12px] text-slate-600"
                      >
                        <Check className="h-3 w-3 shrink-0 text-emerald-500" />
                        {f}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* 授权说明 */}
              {!is_coming_soon && (
                <div className="rounded-2xl bg-sky-50/60 px-4 py-3">
                  <div className="flex items-center gap-2 text-[13px] font-medium text-sky-700">
                    <Shield className="h-4 w-4" />
                    安全授权
                  </div>
                  <p className="mt-1 text-[12px] leading-relaxed text-sky-600/80">
                    连接后，Agent 将通过安全的 MCP 协议访问此应用。你可以随时断开连接并撤销授权。
                  </p>
                </div>
              )}

              {!is_connected && !is_coming_soon && !is_configured && detail.config_error ? (
                <div className="rounded-2xl bg-amber-50/80 px-4 py-3 text-[12px] leading-relaxed text-amber-700">
                  {detail.config_error}
                </div>
              ) : null}

              {/* 文档链接 */}
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
          <div className="flex items-center justify-end gap-2 border-t border-white/12 px-6 py-4">
            {is_connected ? (
              <button
                className="flex items-center gap-1.5 rounded-full bg-slate-100 px-4 py-2 text-[13px] font-medium text-slate-600 transition-colors hover:bg-red-50 hover:text-red-600"
                disabled={busy}
                onClick={() => on_disconnect(detail.connector_id)}
                type="button"
              >
                <Unplug className="h-3.5 w-3.5" />
                断开连接
              </button>
            ) : (
              <button
                className="flex items-center gap-1.5 rounded-full bg-sky-500 px-4 py-2 text-[13px] font-medium text-white shadow-sm transition-all hover:bg-sky-600"
                disabled={busy || !is_configured}
                onClick={() => on_connect(detail.connector_id)}
                type="button"
              >
                <Link2 className="h-3.5 w-3.5" />
                {is_configured ? "授权连接" : "等待配置"}
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
