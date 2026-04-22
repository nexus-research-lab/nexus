"use client";

import { KeyRound, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { createPortal } from "react-dom";

import {
  delete_connector_oauth_client_api,
  get_connector_oauth_client_api,
  upsert_connector_oauth_client_api,
} from "@/lib/api/connector-api";
import {
  DIALOG_ICON_BUTTON_CLASS_NAME,
  get_dialog_action_class_name,
  get_dialog_note_class_name,
  get_dialog_note_style,
} from "@/shared/ui/dialog/dialog-styles";

interface ConnectorOAuthClientDialogProps {
  connector_id: string | null;
  on_close: () => void;
  on_saved: (connector_id: string) => Promise<void>;
  on_error: (message: string) => void;
}

/** OAuth 应用配置弹窗，保存用户自己的 Client ID / Secret。 */
export function ConnectorOAuthClientDialog({
  connector_id,
  on_close,
  on_saved,
  on_error,
}: ConnectorOAuthClientDialogProps) {
  const [client_id, set_client_id] = useState("");
  const [client_secret, set_client_secret] = useState("");
  const [configured, set_configured] = useState(false);
  const [loading, set_loading] = useState(false);
  const [submitting, set_submitting] = useState(false);

  useEffect(() => {
    if (!connector_id) {
      return;
    }
    let cancelled = false;
    set_loading(true);
    set_client_id("");
    set_client_secret("");
    set_configured(false);

    get_connector_oauth_client_api(connector_id)
      .then((item) => {
        if (cancelled) return;
        if (item) {
          set_client_id(item.client_id);
          set_configured(true);
        }
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        on_error(err instanceof Error ? err.message : "读取 OAuth 应用配置失败");
      })
      .finally(() => {
        if (!cancelled) set_loading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [connector_id, on_error]);

  const handle_submit = useCallback(async () => {
    if (!connector_id) return;
    set_submitting(true);
    try {
      await upsert_connector_oauth_client_api(connector_id, client_id, client_secret);
      await on_saved(connector_id);
      on_close();
    } catch (err) {
      on_error(err instanceof Error ? err.message : "保存 OAuth 应用配置失败");
    } finally {
      set_submitting(false);
    }
  }, [client_id, client_secret, connector_id, on_close, on_error, on_saved]);

  const handle_delete = useCallback(async () => {
    if (!connector_id) return;
    set_submitting(true);
    try {
      await delete_connector_oauth_client_api(connector_id);
      await on_saved(connector_id);
      on_close();
    } catch (err) {
      on_error(err instanceof Error ? err.message : "清除 OAuth 应用配置失败");
    } finally {
      set_submitting(false);
    }
  }, [connector_id, on_close, on_error, on_saved]);

  if (!connector_id || typeof document === "undefined") {
    return null;
  }

  const disabled = loading || submitting;
  const dialog = (
    <div className="dialog-backdrop z-[9999]" data-modal-root="true" role="dialog" aria-modal="true">
      <section className="dialog-shell radius-shell-lg flex w-full max-w-md flex-col overflow-hidden">
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <h3 className="dialog-title flex items-center gap-2">
              <KeyRound className="h-4 w-4" />
              OAuth 应用配置
            </h3>
          </div>
          <button className={DIALOG_ICON_BUTTON_CLASS_NAME} aria-label="关闭" onClick={on_close} type="button">
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="dialog-body space-y-4">
          <div className={get_dialog_note_class_name("default")} style={get_dialog_note_style("default")}>
            Client Secret 保存后不会再显示。更新配置时请重新填写 secret。
          </div>

          {connector_id === "github" ? (
            <div className="text-xs text-(--text-muted) leading-relaxed">
              还没有 OAuth 应用？去{" "}
              <a
                className="text-sky-500 hover:underline"
                href="https://github.com/settings/developers"
                rel="noopener noreferrer"
                target="_blank"
              >
                GitHub Developer settings
              </a>{" "}
              新建 OAuth App，Callback URL 填：
              <code className="ml-1 rounded bg-(--surface-inset-background) px-1.5 py-0.5 text-[11px]">
                {window.location.origin}/capability/connectors/oauth/callback
              </code>
            </div>
          ) : null}

          <label className="block space-y-1.5">
            <span className="text-xs font-medium text-(--text-muted)">Client ID</span>
            <input
              className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:ring-2 focus-visible:ring-primary/20 focus-visible:outline-none"
              disabled={disabled}
              onChange={(event) => set_client_id(event.target.value)}
              placeholder="OAuth Client ID"
              type="text"
              value={client_id}
            />
          </label>

          <label className="block space-y-1.5">
            <span className="text-xs font-medium text-(--text-muted)">Client Secret</span>
            <input
              className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:ring-2 focus-visible:ring-primary/20 focus-visible:outline-none"
              disabled={disabled}
              onChange={(event) => set_client_secret(event.target.value)}
              placeholder={configured ? "输入新的 Client Secret" : "OAuth Client Secret"}
              type="password"
              value={client_secret}
            />
          </label>
        </div>

        <div className="dialog-footer">
          {configured ? (
            <button className={get_dialog_action_class_name("danger")} disabled={disabled} onClick={handle_delete} type="button">
              清除配置
            </button>
          ) : null}
          <button className={get_dialog_action_class_name("default")} disabled={disabled} onClick={on_close} type="button">
            取消
          </button>
          <button className={get_dialog_action_class_name("primary")} disabled={disabled} onClick={handle_submit} type="button">
            保存
          </button>
        </div>
      </section>
    </div>
  );

  return createPortal(dialog, document.body);
}
