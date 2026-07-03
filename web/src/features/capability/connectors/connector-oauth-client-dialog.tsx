"use client";

import { Check, Copy, ExternalLink, KeyRound, Save, Trash2 } from "lucide-react";
import { type FormEvent, useCallback } from "react";

import { getConnectorOauthRedirectUri } from "@/config/desktop-runtime";
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
  onClose: () => void;
  onSave: (connectorId: string, clientId: string, clientSecret: string) => void;
  onDelete: (connectorId: string) => void;
}

/** OAuth Client 配置弹窗。 */
export function ConnectorOAuthClientDialog({
  detail,
  busy,
  onClose: onClose,
  onSave: onSave,
  onDelete: onDelete,
}: ConnectorOAuthClientDialogProps) {
  const detailResetKey = `${detail?.connector_id ?? ""}\x1f${detail?.oauth_client_id ?? ""}`;
  const [clientId, setClientId] = useResettableState(detail?.oauth_client_id ?? "", detailResetKey);
  const [clientSecret, setClientSecret] = useResettableState("", detailResetKey);
  const { copied: callbackUrlCopied, copy: copyCallbackUrl } = useCopyToClipboard();

  const handleSubmit = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (!detail) return;
      onSave(detail.connector_id, clientId, clientSecret);
    },
    [clientId, clientSecret, detail, onSave],
  );

  if (!detail) return null;

  const isConfigured = detail.oauth_client_configured ?? false;
  const canSave = clientId.trim() !== "" && clientSecret.trim() !== "";
  const callbackUrl = getConnectorOauthRedirectUri();
  const providerName = detail.connector_id === "feishu-docx" ? "飞书开放平台应用" : "OAuth 应用";

  return (
    <UiDialogBackdrop onClose={onClose}>
      <UiDialogFormShell className="max-h-[84vh]" onSubmit={handleSubmit} size="sm">
        <UiDialogHeader
          icon={<KeyRound className="h-4 w-4" />}
          iconClassName="h-9 w-9 rounded-[14px]"
          onClose={onClose}
          subtitle={detail.title}
          title="配置应用"
        />

        <UiDialogBody className="space-y-3" scrollable>
          <UiPanel className="text-[12px] leading-relaxed" padding="sm" variant="inset">
            在{providerName}中填写下面的 Callback URL，再复制 App ID 和 App Secret。
          </UiPanel>

          {detail.docs_url ? (
            <UiLinkButton
              className="w-fit"
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
            <UiPanel className="flex min-h-9 items-center gap-2" padding="sm" radius="sm" variant="inset">
              <code className="min-w-0 flex-1 break-all text-[11px] leading-5 text-(--text-strong)">
                {callbackUrl}
              </code>
              <UiIconButton
                aria-label={callbackUrlCopied ? "已复制 Callback URL" : "复制 Callback URL"}
                className="shrink-0"
                onClick={() => void copyCallbackUrl(callbackUrl)}
                size="sm"
                title={callbackUrlCopied ? "已复制" : "复制 Callback URL"}
                type="button"
              >
                {callbackUrlCopied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
              </UiIconButton>
            </UiPanel>
          </div>

          <label className="block space-y-1 text-[12px] font-medium text-(--text-muted)" htmlFor="oauth-client-id">
            <span>Client ID</span>
            <UiInput
              autoCapitalize="off"
              autoCorrect="off"
              controlSize="sm"
              id="oauth-client-id"
              onChange={(event) => setClientId(event.target.value)}
              placeholder="飞书应用 App ID"
              spellCheck={false}
              value={clientId}
            />
          </label>

          <label className="block space-y-1 text-[12px] font-medium text-(--text-muted)" htmlFor="oauth-client-secret">
            <span>Client Secret</span>
            <UiInput
              autoCapitalize="off"
              autoComplete="off"
              autoCorrect="off"
              controlSize="sm"
              data-form-type="other"
              data-lpignore="true"
              id="oauth-client-secret"
              name="feishu-docx-client-secret"
              onChange={(event) => setClientSecret(event.target.value)}
              placeholder={isConfigured ? "重新填写后保存" : "飞书应用 App Secret"}
              spellCheck={false}
              type="password"
              value={clientSecret}
            />
          </label>
        </UiDialogBody>

        <UiDialogFooter className="flex-wrap gap-1.5">
          {isConfigured ? (
            <UiButton
              disabled={busy}
              onClick={() => onDelete(detail.connector_id)}
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
            disabled={busy || !canSave}
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
