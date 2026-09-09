// INPUT: OAuth Connector 的应用配置、现有身份与保存/删除动作。
// OUTPUT: 回调地址和实例独立标签的应用凭据组成的 plain 配置弹窗，删除与保存动作保持分离。
// POS: Connector OAuth 客户端配置的人机边界，不重复解释内部授权流程。
"use client";

import { Check, Copy, ExternalLink, Trash2 } from "lucide-react";
import {
  type Dispatch,
  type FormEvent,
  type SetStateAction,
  useCallback,
  useId,
} from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { useCopyToClipboard } from "@/shared/lib/react/use-copy-to-clipboard";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { UiButton, UiIconButton, UiLinkButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
} from "@/shared/ui/dialog/dialog";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
  const { t } = useI18n();
  const model = buildConnectorOauthClientDialogModel(detail, t);
  const form = useConnectorOauthClientForm(model, busy, onSave);
  if (!model) return null;

  return (
    <UiDialogBackdrop onClose={onClose}>
      <UiDialogFormShell
        aria-busy={busy}
        onSubmit={form.handleSubmit}
        size="sm"
        viewport="compactMax"
      >
        <UiDialogHeader
          appearance="plain"
          onClose={onClose}
          title={t("capability.oauth_client_title", { title: model.title })}
        />
        <ConnectorOauthClientBody form={form} model={model} />
        <ConnectorOauthClientFooter
          busy={busy}
          model={model}
          onClose={onClose}
          onDelete={onDelete}
        />
      </UiDialogFormShell>
    </UiDialogBackdrop>
  );
}

interface ConnectorOauthClientFormState {
  clientId: string;
  clientSecret: string;
  handleSubmit: (event: FormEvent<HTMLFormElement>) => void;
  setClientId: Dispatch<SetStateAction<string>>;
  setClientSecret: Dispatch<SetStateAction<string>>;
}

function useConnectorOauthClientForm(
  model: ConnectorOauthClientDialogModel | null,
  busy: boolean,
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
    if (busy || !model || !connectorOauthCredentialsComplete(clientId, clientSecret)) {
      return;
    }
    onSave(model.connectorId, clientId, clientSecret);
  }, [busy, clientId, clientSecret, model, onSave]);

  return {
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
    <UiDialogBody className="space-y-4 px-5" scrollable>
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
  const { t } = useI18n();
  return (
    <>
      <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>
        {t("capability.oauth_client_description", { provider: model.providerName })}
      </p>
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
          {t("capability.credential_docs")}
        </UiLinkButton>
      ) : null}
    </>
  );
}

function ConnectorOauthCallbackField({ callbackUrl }: { callbackUrl: string }) {
  const { t } = useI18n();
  const { copied, copy } = useCopyToClipboard();
  return (
    <div className="space-y-1">
      <div className={getUiTypographyClassName({
        role: "metadata",
        tone: "muted",
        weight: "medium",
      })}>Callback URL</div>
      <UiPanel className="flex min-h-9 items-center gap-2" padding="sm" radius="sm" variant="card">
        <code className={cn(
          "min-w-0 flex-1 break-all",
          getUiTypographyClassName({ role: "code", tone: "strong" }),
        )}>
          {callbackUrl}
        </code>
        <UiIconButton
          aria-label={t(copied ? "capability.oauth_callback_copied" : "capability.oauth_callback_copy")}
          className="shrink-0"
          onClick={() => void copy(callbackUrl)}
          size="sm"
          title={t(copied ? "capability.oauth_callback_copied" : "capability.oauth_callback_copy")}
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
  const fieldId = useId();
  return (
    <>
      <UiField htmlFor={`${fieldId}-client-id`} label="Client ID" required>
        <UiInput
          autoCapitalize="off"
          autoCorrect="off"
          controlSize="sm"
          id={`${fieldId}-client-id`}
          onChange={(event) => form.setClientId(event.target.value)}
          pattern=".*\S.*"
          placeholder={model.clientIdPlaceholder}
          required
          spellCheck={false}
          value={form.clientId}
        />
      </UiField>
      <UiField htmlFor={`${fieldId}-client-secret`} label="Client Secret" required>
        <UiInput
          autoCapitalize="off"
          autoComplete="off"
          autoCorrect="off"
          controlSize="sm"
          data-form-type="other"
          data-lpignore="true"
          id={`${fieldId}-client-secret`}
          name="oauth-client-secret"
          onChange={(event) => form.setClientSecret(event.target.value)}
          pattern=".*\S.*"
          placeholder={model.secretPlaceholder}
          required
          spellCheck={false}
          type="password"
          value={form.clientSecret}
        />
      </UiField>
    </>
  );
}

function ConnectorOauthClientFooter({
  busy,
  model,
  onClose,
  onDelete,
}: {
  busy: boolean;
  model: ConnectorOauthClientDialogModel;
  onClose: ConnectorOAuthClientDialogProps["onClose"];
  onDelete: ConnectorOAuthClientDialogProps["onDelete"];
}) {
  const { t } = useI18n();
  return (
    <UiDialogFooter appearance="plain" className="justify-between">
      <div>
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
          {t("capability.oauth_delete_configuration")}
        </UiButton>
        ) : null}
      </div>
      <div className="flex items-center gap-2">
        <UiButton disabled={busy} onClick={onClose} size="sm" type="button">
          {t("common.cancel")}
        </UiButton>
        <UiButton
          disabled={busy}
          size="sm"
          tone="primary"
          type="submit"
          variant="solid"
        >
          {t("capability.oauth_save")}
        </UiButton>
      </div>
    </UiDialogFooter>
  );
}
