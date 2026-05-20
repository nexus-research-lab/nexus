"use client";

import { Check, Copy, ExternalLink, Github, Loader2, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import { poll_connector_device_auth_api } from "@/lib/api/connector-api";
import {
  DIALOG_ICON_BUTTON_CLASS_NAME,
  get_dialog_action_class_name,
  get_dialog_note_class_name,
  get_dialog_note_style,
} from "@/shared/ui/dialog/dialog-styles";
import type { ConnectorDeviceAuthStart } from "@/types/capability/connector";

interface ConnectorDeviceAuthDialogProps {
  session: ConnectorDeviceAuthStart | null;
  on_close: () => void;
  on_connected: (connector_id: string) => Promise<void>;
  on_error: (message: string) => void;
}

/** 桌面 GitHub Device Flow 授权弹窗。 */
export function ConnectorDeviceAuthDialog({
  session,
  on_close,
  on_connected,
  on_error,
}: ConnectorDeviceAuthDialogProps) {
  const [copied, set_copied] = useState(false);
  const [polling_message, set_polling_message] = useState("等待 GitHub 授权确认");
  const on_connected_ref = useRef(on_connected);
  const on_close_ref = useRef(on_close);
  const on_error_ref = useRef(on_error);

  useEffect(() => {
    on_connected_ref.current = on_connected;
  }, [on_connected]);

  useEffect(() => {
    on_close_ref.current = on_close;
  }, [on_close]);

  useEffect(() => {
    on_error_ref.current = on_error;
  }, [on_error]);

  useEffect(() => {
    if (!session) {
      return;
    }
    let cancelled = false;
    let timeout_id: ReturnType<typeof setTimeout> | null = null;
    let delay_ms = Math.max(session.interval || 5, 1) * 1000;

    const schedule_next_poll = () => {
      timeout_id = setTimeout(() => {
        void poll();
      }, delay_ms);
    };

    const poll = async () => {
      try {
        const result = await poll_connector_device_auth_api(session.connector_id, session.device_code);
        if (cancelled) {
          return;
        }
        if (result.status === "connected") {
          set_polling_message("GitHub 已授权");
          await on_connected_ref.current(session.connector_id);
          if (!cancelled) {
            on_close_ref.current();
          }
          return;
        }
        if (result.status === "slow_down") {
          delay_ms += 5000;
        }
        if (result.status === "expired" || result.status === "denied") {
          on_error_ref.current(result.message || "GitHub 授权未完成");
          on_close_ref.current();
          return;
        }
        set_polling_message(result.message || "等待 GitHub 授权确认");
        schedule_next_poll();
      } catch (err) {
        if (!cancelled) {
          on_error_ref.current(err instanceof Error ? err.message : "GitHub 授权轮询失败");
          on_close_ref.current();
        }
      }
    };

    set_polling_message("等待 GitHub 授权确认");
    schedule_next_poll();
    return () => {
      cancelled = true;
      if (timeout_id) {
        clearTimeout(timeout_id);
      }
    };
  }, [session]);

  const handle_copy = useCallback(async () => {
    if (!session) {
      return;
    }
    try {
      await navigator.clipboard.writeText(session.user_code);
      set_copied(true);
      setTimeout(() => set_copied(false), 1400);
    } catch {
      on_error_ref.current("复制授权码失败");
    }
  }, [session]);

  if (!session || typeof document === "undefined") {
    return null;
  }

  const auth_url = session.verification_uri_complete || session.verification_uri;
  const dialog = (
    <div className="dialog-backdrop z-[9999]" data-modal-root="true" role="dialog" aria-modal="true">
      <section className="dialog-shell radius-shell-lg flex w-full max-w-md flex-col overflow-hidden">
        <div className="dialog-header">
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div className="dialog-card flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl text-(--text-strong)">
              <Github className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <h3 className="dialog-title">连接 GitHub</h3>
              <p className="dialog-subtitle">在 GitHub 输入授权码完成连接。</p>
            </div>
          </div>
          <button className={DIALOG_ICON_BUTTON_CLASS_NAME} aria-label="关闭" onClick={on_close} type="button">
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="dialog-body space-y-4">
          <div className={get_dialog_note_class_name("default")} style={get_dialog_note_style("default")}>
            <div className="flex items-center gap-2 text-[13px] font-medium text-(--text-default)">
              <Loader2 className="h-4 w-4 animate-spin" />
              <span aria-live="polite">{polling_message}</span>
            </div>
          </div>

          <div className="surface-card rounded-[18px] px-4 py-4">
            <div className="text-[11px] font-semibold uppercase text-(--text-soft)">GitHub code</div>
            <div className="mt-2 flex items-center gap-3">
              <code className="min-w-0 flex-1 select-all break-all rounded-[14px] bg-(--surface-inset-background) px-3 py-2.5 text-center text-[24px] font-black text-(--text-strong)">
                {session.user_code}
              </code>
              <button
                aria-label="复制授权码"
                className={DIALOG_ICON_BUTTON_CLASS_NAME}
                onClick={() => void handle_copy()}
                type="button"
              >
                {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
              </button>
            </div>
          </div>
        </div>

        <div className="dialog-footer">
          <button className={get_dialog_action_class_name("default")} onClick={on_close} type="button">
            取消
          </button>
          <button
            className={get_dialog_action_class_name("primary")}
            onClick={() => window.open(auth_url, "_blank", "noopener,noreferrer")}
            type="button"
          >
            <ExternalLink className="h-3.5 w-3.5" />
            打开 GitHub
          </button>
        </div>
      </section>
    </div>
  );

  return createPortal(dialog, document.body);
}
