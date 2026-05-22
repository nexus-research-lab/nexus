"use client";

import { Check, Copy, ExternalLink, KeyRound, Save, Trash2, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { get_connector_oauth_redirect_uri } from "@/config/desktop-runtime";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import { cn } from "@/lib/utils";
import {
  DIALOG_ICON_BUTTON_CLASS_NAME,
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  DIALOG_TEXT_BUTTON_CLASS_NAME,
  get_dialog_action_class_name,
  get_dialog_note_class_name,
  get_dialog_note_style,
} from "@/shared/ui/dialog/dialog-styles";
import type { ConnectorDetail } from "@/types/capability/connector";

const OAUTH_CLIENT_INPUT_CLASS_NAME =
  "dialog-input h-8 w-full rounded-[10px] px-3 text-[13px] text-(--text-strong) outline-none disabled:opacity-(--disabled-opacity)";

interface ConnectorOAuthClientDialogProps {
  detail: ConnectorDetail | null;
  busy: boolean;
  on_close: () => void;
  on_save: (connector_id: string, client_id: string, client_secret: string) => void;
  on_delete: (connector_id: string) => void;
}

/** OAuth Client 配置弹窗。 */
export function ConnectorOAuthClientDialog({
  detail,
  busy,
  on_close,
  on_save,
  on_delete,
}: ConnectorOAuthClientDialogProps) {
  const [client_id, set_client_id] = useState("");
  const [client_secret, set_client_secret] = useState("");
  const { copied: callback_url_copied, copy: copy_callback_url } = useCopyToClipboard();

  useEffect(() => {
    set_client_id(detail?.oauth_client_id ?? "");
    set_client_secret("");
  }, [detail?.connector_id, detail?.oauth_client_id]);

  const handle_backdrop_click = useCallback(
    (event: React.MouseEvent) => {
      if (event.target === event.currentTarget) on_close();
    },
    [on_close],
  );

  const handle_submit = useCallback(
    (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (!detail) return;
      on_save(detail.connector_id, client_id, client_secret);
    },
    [client_id, client_secret, detail, on_save],
  );

  if (!detail) return null;

  const is_configured = detail.oauth_client_configured ?? false;
  const can_save = client_id.trim() !== "" && client_secret.trim() !== "";
  const callback_url = get_connector_oauth_redirect_uri();
  const provider_name = detail.connector_id === "feishu-docx" ? "飞书开放平台应用" : "OAuth 应用";

  return (
    <div
      className="dialog-backdrop"
      onClick={handle_backdrop_click}
    >
      <div className="dialog-shell relative flex max-h-[84vh] w-full max-w-[420px] flex-col overflow-hidden rounded-3xl">
        <div className="dialog-header">
          <div className={DIALOG_HEADER_LEADING_CLASS_NAME}>
            <div className={cn(DIALOG_HEADER_ICON_CLASS_NAME, "h-9 w-9 rounded-[14px]")}>
              <KeyRound className="h-4 w-4" />
            </div>
            <div className="min-w-0">
              <h2 className="dialog-title">配置应用</h2>
              <p className="dialog-subtitle">{detail.title}</p>
            </div>
          </div>
          <button
            className={DIALOG_ICON_BUTTON_CLASS_NAME}
            aria-label="关闭"
            onClick={on_close}
            type="button"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <form className="dialog-body space-y-3" onSubmit={handle_submit}>
          <div
            className={get_dialog_note_class_name("default", "px-3 py-2.5 text-[12px] leading-relaxed")}
            style={get_dialog_note_style("default")}
          >
            在{provider_name}中填写下面的 Callback URL，再复制 App ID 和 App Secret。
          </div>

          {detail.docs_url ? (
            <a
              className={DIALOG_TEXT_BUTTON_CLASS_NAME}
              href={detail.docs_url}
              rel="noopener noreferrer"
              target="_blank"
            >
              <ExternalLink className="h-3 w-3" />
              查看文档
            </a>
          ) : null}

          <div className="space-y-1">
            <div className="text-[12px] font-medium text-(--text-muted)">Callback URL</div>
            <div className="dialog-card flex min-h-9 items-center gap-2 rounded-[10px] px-3 py-1.5">
              <code className="min-w-0 flex-1 break-all text-[11px] leading-5 text-(--text-strong)">
                {callback_url}
              </code>
              <button
                aria-label={callback_url_copied ? "已复制 Callback URL" : "复制 Callback URL"}
                className={cn(DIALOG_ICON_BUTTON_CLASS_NAME, "h-7 w-7 shrink-0 rounded-[9px]")}
                onClick={() => void copy_callback_url(callback_url)}
                title={callback_url_copied ? "已复制" : "复制 Callback URL"}
                type="button"
              >
                {callback_url_copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
              </button>
            </div>
          </div>

          <label className="block space-y-1 text-[12px] font-medium text-(--text-muted)">
            <span>Client ID</span>
            <input
              autoCapitalize="off"
              autoCorrect="off"
              className={OAUTH_CLIENT_INPUT_CLASS_NAME}
              onChange={(event) => set_client_id(event.target.value)}
              placeholder="飞书应用 App ID"
              spellCheck={false}
              type="text"
              value={client_id}
            />
          </label>

          <label className="block space-y-1 text-[12px] font-medium text-(--text-muted)">
            <span>Client Secret</span>
            <input
              autoCapitalize="off"
              autoComplete="off"
              autoCorrect="off"
              className={OAUTH_CLIENT_INPUT_CLASS_NAME}
              data-form-type="other"
              data-lpignore="true"
              name="feishu-docx-client-secret"
              onChange={(event) => set_client_secret(event.target.value)}
              placeholder={is_configured ? "重新填写后保存" : "飞书应用 App Secret"}
              spellCheck={false}
              type="password"
              value={client_secret}
            />
          </label>
        </form>

        <div className="dialog-footer flex-wrap gap-1.5">
          {is_configured ? (
            <button
              className={get_dialog_action_class_name("danger", "compact")}
              disabled={busy}
              onClick={() => on_delete(detail.connector_id)}
              type="button"
            >
              <Trash2 className="h-3.5 w-3.5" />
              删除配置
            </button>
          ) : null}
          <button
            className={get_dialog_action_class_name("primary", "compact")}
            disabled={busy || !can_save}
            onClick={() => on_save(detail.connector_id, client_id, client_secret)}
            type="button"
          >
            <Save className="h-3.5 w-3.5" />
            保存配置
          </button>
        </div>
      </div>
    </div>
  );
}
