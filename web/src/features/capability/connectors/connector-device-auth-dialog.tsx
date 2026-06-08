"use client";

import { Check, Copy, ExternalLink, Github, Loader2 } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";

import { write_text_to_clipboard } from "@/hooks/ui/clipboard";
import { poll_connector_device_auth_api } from "@/lib/api/connector-api";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { UiPanel } from "@/shared/ui/panel";
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
    if (await write_text_to_clipboard(session.user_code)) {
      set_copied(true);
      setTimeout(() => set_copied(false), 1400);
      return;
    }
    on_error_ref.current("复制授权码失败");
  }, [session]);

  if (!session || typeof document === "undefined") {
    return null;
  }

  const auth_url = session.verification_uri_complete || session.verification_uri;
  return (
    <UiDialogPortal>
      <UiDialogBackdrop class_name="z-[9999]" on_close={on_close}>
        <UiDialogShell size="sm">
          <UiDialogHeader
            icon={<Github className="h-5 w-5" />}
            on_close={on_close}
            subtitle="在 GitHub 输入授权码完成连接。"
            title="连接 GitHub"
          />

          <UiDialogBody class_name="space-y-4">
            <UiPanel padding="sm" variant="inset">
              <div className="flex items-center gap-2 text-[13px] font-medium text-(--text-default)">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span aria-live="polite">{polling_message}</span>
              </div>
            </UiPanel>

            <UiPanel padding="md">
              <div className="text-[11px] font-semibold uppercase text-(--text-soft)">GitHub code</div>
              <div className="mt-2 flex items-center gap-3">
                <code className="min-w-0 flex-1 select-all break-all rounded-[14px] bg-transparent px-3 py-2.5 text-center text-[24px] font-black text-(--text-strong)">
                  {session.user_code}
                </code>
                <UiIconButton
                  aria-label="复制授权码"
                  onClick={() => void handle_copy()}
                  type="button"
                >
                  {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                </UiIconButton>
              </div>
            </UiPanel>
          </UiDialogBody>

          <UiDialogFooter>
            <UiButton onClick={on_close} type="button">
              取消
            </UiButton>
            <UiButton
              onClick={() => window.open(auth_url, "_blank", "noopener,noreferrer")}
              tone="primary"
              type="button"
              variant="solid"
            >
              <ExternalLink className="h-3.5 w-3.5" />
              打开 GitHub
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
