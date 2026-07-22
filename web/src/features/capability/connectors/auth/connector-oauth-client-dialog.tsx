"use client";

import { Check, Copy, ExternalLink, KeyRound, Save, Trash2 } from "lucide-react";
import {
  type Dispatch,
  type FormEvent,
  type SetStateAction,
  useCallback,
} from "react";

import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { UiButton, UiIconButton, UiLinkButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
} from "@/shared/ui/dialog/dialog";
import { UiInput } from "@/shared/ui/form/form-control";
import { UiPanel } from "@/shared/ui/panel";
import type { ConnectorDetail } from "@/types/capability/connector";

import {
  buildConnectorOauthClientDialogModel,
  connectorOauthCredentialsComplete,
  type ConnectorOauthClientDialogModel,
} from "./connector-oauth-client-model";

interface ConnectorOAuthClientDialogProps {
  busy: boolean;
  detail: ConnectorDetail | null;
  onClose: () => void;
  onDelete: (connectorId: string) => void;
  onSave: (connectorId: string, clientId: string, clientSecret: string) => void;
}

export function ConnectorOAuthClientDialog({
  busy,
  detail,
  onClose,
  onDelete,
  onSave,
}: ConnectorOAuthClientDialogProps) {
  const model = buildConnectorOauthClientDialogModel(detail);
  const form = useConnectorOauthClientForm(model, onSave);
  if (!model) return null;

  return (
    <UiDialogBackdrop onClose={onClose}>
      <UiDialogFormShell
        className="max-h-[84vh]"
        onSubmit={form.handleSubmit}
        size="sm"
      >
        <UiDialogHeader
          icon={<KeyRound className="h-4 w-4" />}
          iconClassName="h-9 w-9 rounded-[14px]"
          onClose={onClose}
          subtitle={model.title}
          title="配置应用"
        />
        <ConnectorOauthClientBody form={form} model={model} />
        <ConnectorOauthClientFooter
          busy={busy}
          canSave={form.canSave}
          model={model}
          onDelete={onDelete}
        />
      </UiDialogFormShell>
    </UiDialogBackdrop>
  );
}

interface ConnectorOauthClientFormState {
  canSave: boolean;
  clientId: string;
  clientSecret: string;
  handleSubmit: (event: FormEvent<HTMLFormElement>) => void;
  setClientId: Dispatch<SetStateAction<string>>;
  setClientSecret: Dispatch<SetStateAction<string>>;
}

function useConnectorOauthClientForm(
  model: ConnectorOauthClientDialogModel | null,
  onSave: ConnectorOAuthClientDialogProps["onSave"],
): ConnectorOauthClientFormState {
  const resetKey = model?.resetKey ?? "closed";
  const [clientId, setClientId] = useResettableState(
    model?.initialClientId ?? "",
    resetKey,
  );
  const [clientSecret, setClientSecret] = useResettableState("", resetKey);
  const handleSubmit = useCallback((event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!model) return;
    onSave(model.connectorId, clientId, clientSecret);
  }, [clientId, clientSecret, model, onSave]);

  return {
    canSave: connectorOauthCredentialsComplete(clientId, clientSecret),
    clientId,
    clientSecret,
    handleSubmit,
    setClientId,
    setClientSecret,
  };
}

function ConnectorOauthClientBody({
  form,
  model,
}: {
  form: ConnectorOauthClientFormState;
  model: ConnectorOauthClientDialogModel;
}) {
  return (
    <UiDialogBody className="space-y-3" scrollable>
      <ConnectorOauthClientIntroduction model={model} />
      <ConnectorOauthCallbackField callbackUrl={model.callbackUrl} />
      <ConnectorOauthClientFields form={form} model={model} />
    </UiDialogBody>
  );
}

function ConnectorOauthClientIntroduction({
  model,
}: {
  model: ConnectorOauthClientDialogModel;
}) {
  return (
    <>
      <UiPanel className="text-[12px] leading-relaxed" padding="sm" variant="inset">
        在{model.providerName}中填写下面的 Callback URL，再复制 App ID 和 App Secret。
      </UiPanel>
      {model.docsUrl ? (
        <UiLinkButton
          className="w-fit"
          href={model.docsUrl}
          rel="noopener noreferrer"
          size="sm"
          target="_blank"
          variant="text"
        >
          <ExternalLink className="h-3 w-3" />
          查看文档
        </UiLinkButton>
      ) : null}
    </>
  );
}

function ConnectorOauthCallbackField({ callbackUrl }: { callbackUrl: string }) {
  const { copied, copy } = useCopyToClipboard();
  return (
    <div className="space-y-1">
      <div className="text-[12px] font-medium text-(--text-muted)">Callback URL</div>
      <UiPanel className="flex min-h-9 items-center gap-2" padding="sm" radius="sm" variant="inset">
        <code className="min-w-0 flex-1 break-all text-[11px] leading-5 text-(--text-strong)">
          {callbackUrl}
        </code>
        <UiIconButton
          aria-label={copied ? "已复制 Callback URL" : "复制 Callback URL"}
          className="shrink-0"
          onClick={() => void copy(callbackUrl)}
          size="sm"
          title={copied ? "已复制" : "复制 Callback URL"}
          type="button"
        >
          {copied
            ? <Check className="h-3.5 w-3.5" />
            : <Copy className="h-3.5 w-3.5" />}
        </UiIconButton>
      </UiPanel>
    </div>
  );
}

function ConnectorOauthClientFields({
  form,
  model,
}: {
  form: ConnectorOauthClientFormState;
  model: ConnectorOauthClientDialogModel;
}) {
  return (
    <>
      <label className="block space-y-1 text-[12px] font-medium text-(--text-muted)" htmlFor="oauth-client-id">
        <span>Client ID</span>
        <UiInput
          autoCapitalize="off"
          autoCorrect="off"
          controlSize="sm"
          id="oauth-client-id"
          onChange={(event) => form.setClientId(event.target.value)}
          placeholder="飞书应用 App ID"
          spellCheck={false}
          value={form.clientId}
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
          onChange={(event) => form.setClientSecret(event.target.value)}
          placeholder={model.secretPlaceholder}
          spellCheck={false}
          type="password"
          value={form.clientSecret}
        />
      </label>
    </>
  );
}

function ConnectorOauthClientFooter({
  busy,
  canSave,
  model,
  onDelete,
}: {
  busy: boolean;
  canSave: boolean;
  model: ConnectorOauthClientDialogModel;
  onDelete: ConnectorOAuthClientDialogProps["onDelete"];
}) {
  return (
    <UiDialogFooter className="flex-wrap gap-1.5">
      {model.configured ? (
        <UiButton
          disabled={busy}
          onClick={() => onDelete(model.connectorId)}
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
  );
}
