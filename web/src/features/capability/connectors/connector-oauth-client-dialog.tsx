"use client";

import { Check, Copy, ExternalLink, KeyRound, Save, Trash2 } from "lucide-react";
import { type FormEvent, useCallback } from "react";

import { get_connector_oauth_redirect_uri } from "@/config/desktop-runtime";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
} from "@/shared/ui/dialog/dialog";
import { UiButton, UiIconButton, UiLinkButton } from "@/shared/ui/button";
import { UiInput } from "@/shared/ui/form-control";
import { UiPanel } from "@/shared/ui/panel";
import type { ConnectorDetail } from "@/types/capability/connector";

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
  const detail_reset_key = `${detail?.connector_id ?? ""}\x1f${detail?.oauth_client_id ?? ""}`;
  const [client_id, set_client_id] = useResettableState(detail?.oauth_client_id ?? "", detail_reset_key);
  const [client_secret, set_client_secret] = useResettableState("", detail_reset_key);
  const { copied: callback_url_copied, copy: copy_callback_url } = useCopyToClipboard();

  const handle_submit = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
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
    <UiDialogBackdrop on_close={on_close}>
      <UiDialogFormShell class_name="max-h-[84vh]" onSubmit={handle_submit} size="sm">
        <UiDialogHeader
          icon={<KeyRound className="h-4 w-4" />}
          icon_class_name="h-9 w-9 rounded-[14px]"
          on_close={on_close}
          subtitle={detail.title}
          title="配置应用"
        />

        <UiDialogBody class_name="space-y-3" scrollable>
          <UiPanel class_name="text-[12px] leading-relaxed" padding="sm" variant="inset">
            在{provider_name}中填写下面的 Callback URL，再复制 App ID 和 App Secret。
          </UiPanel>

          {detail.docs_url ? (
            <UiLinkButton
              class_name="w-fit"
              href={detail.docs_url}
              rel="noopener noreferrer"
              size="sm"
              target="_blank"
              variant="text"
            >
              <ExternalLink className="h-3 w-3" />
              查看文档
            </UiLinkButton>
          ) : null}

          <div className="space-y-1">
            <div className="text-[12px] font-medium text-(--text-muted)">Callback URL</div>
            <UiPanel class_name="flex min-h-9 items-center gap-2" padding="sm" radius="sm" variant="inset">
              <code className="min-w-0 flex-1 break-all text-[11px] leading-5 text-(--text-strong)">
                {callback_url}
              </code>
              <UiIconButton
                aria-label={callback_url_copied ? "已复制 Callback URL" : "复制 Callback URL"}
                class_name="shrink-0"
                onClick={() => void copy_callback_url(callback_url)}
                size="sm"
                title={callback_url_copied ? "已复制" : "复制 Callback URL"}
                type="button"
              >
                {callback_url_copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
              </UiIconButton>
            </UiPanel>
          </div>

          <label className="block space-y-1 text-[12px] font-medium text-(--text-muted)" htmlFor="oauth-client-id">
            <span>Client ID</span>
            <UiInput
              autoCapitalize="off"
              autoCorrect="off"
              control_size="sm"
              id="oauth-client-id"
              onChange={(event) => set_client_id(event.target.value)}
              placeholder="飞书应用 App ID"
              spellCheck={false}
              value={client_id}
            />
          </label>

          <label className="block space-y-1 text-[12px] font-medium text-(--text-muted)" htmlFor="oauth-client-secret">
            <span>Client Secret</span>
            <UiInput
              autoCapitalize="off"
              autoComplete="off"
              autoCorrect="off"
              control_size="sm"
              data-form-type="other"
              data-lpignore="true"
              id="oauth-client-secret"
              name="feishu-docx-client-secret"
              onChange={(event) => set_client_secret(event.target.value)}
              placeholder={is_configured ? "重新填写后保存" : "飞书应用 App Secret"}
              spellCheck={false}
              type="password"
              value={client_secret}
            />
          </label>
        </UiDialogBody>

        <UiDialogFooter class_name="flex-wrap gap-1.5">
          {is_configured ? (
            <UiButton
              disabled={busy}
              onClick={() => on_delete(detail.connector_id)}
              size="sm"
              tone="danger"
              type="button"
              variant="surface"
            >
              <Trash2 className="h-3.5 w-3.5" />
              删除配置
            </UiButton>
          ) : null}
          <UiButton
            disabled={busy || !can_save}
            size="sm"
            tone="primary"
            type="submit"
            variant="solid"
          >
            <Save className="h-3.5 w-3.5" />
            保存配置
          </UiButton>
        </UiDialogFooter>
      </UiDialogFormShell>
    </UiDialogBackdrop>
  );
}
