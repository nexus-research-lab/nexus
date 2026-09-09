// INPUT: 需要直接凭证的 Connector、提交状态与保存/关闭动作。
// OUTPUT: 本地化的必要说明、实例独立标签的凭证字段与连接动作；语言切换保留当前目标的输入。
// POS: Connector 直接凭证的人机边界，不解释 runtime 或 MCP 内部装配细节。
"use client";

import { ExternalLink } from "lucide-react";
import { type FormEvent, useCallback, useId } from "react";

import { useI18n, type I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { UiButton, UiLinkButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
} from "@/shared/ui/dialog/dialog";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ConnectorDetail } from "@/types/capability/connector";

import { getDirectCredentialLabel } from "./connector-auth";

interface ConnectorCredentialDialogProps {
  detail: ConnectorDetail | null;
  busy: boolean;
  onClose: () => void;
  onSave: (connectorId: string, credential: string) => void;
}

type CredentialCopy = {
  description: string;
  label: string;
  placeholder: string;
};

const CONNECTOR_CREDENTIAL_COPY: Record<string, { description: TranslationKey; placeholder: TranslationKey; label?: string }> = {
  amap: {
    description: "capability.credential_amap_description",
    placeholder: "capability.credential_amap_placeholder",
  },
  didi: {
    description: "capability.credential_didi_description",
    placeholder: "capability.credential_didi_placeholder",
  },
  "dingtalk-ai-table": {
    description: "capability.credential_dingtalk_description",
    label: "MCP Server URL",
    placeholder: "capability.credential_dingtalk_placeholder",
  },
  "tencent-docs": {
    description: "capability.credential_tencent_description",
    placeholder: "capability.credential_tencent_placeholder",
  },
  yuque: {
    description: "capability.credential_yuque_description",
    placeholder: "capability.credential_yuque_placeholder",
  },
};

function getCredentialCopy(detail: ConnectorDetail, t: I18nContextValue["t"]): CredentialCopy {
  const label = getDirectCredentialLabel(detail.auth_type);
  const copy = CONNECTOR_CREDENTIAL_COPY[detail.connector_id];
  return {
    description: copy ? t(copy.description) : t("capability.credential_description", { label, title: detail.title }),
    label: copy?.label ?? label,
    placeholder: copy ? t(copy.placeholder) : `${detail.title} ${label}`,
  };
}

/** 直接凭证连接器弹窗。 */
export function ConnectorCredentialDialog({
  detail,
  busy,
  onClose,
  onSave,
}: ConnectorCredentialDialogProps) {
  const { t } = useI18n();
  const credentialId = useId();
  const [credential, setCredential] = useResettableState("", detail?.connector_id ?? null);

  const handleSubmit = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (busy || !detail || !credential.trim()) return;
      onSave(detail.connector_id, credential.trim());
    },
    [busy, credential, detail, onSave],
  );

  if (!detail) return null;

  const copy = getCredentialCopy(detail, t);
  return (
    <UiDialogBackdrop onClose={onClose}>
      <UiDialogFormShell
        aria-busy={busy}
        onSubmit={handleSubmit}
        size="sm"
        viewport="compactMax"
      >
        <UiDialogHeader
          appearance="plain"
          onClose={onClose}
          title={t("capability.credential_title", { title: detail.title })}
        />

        <UiDialogBody className="space-y-4 px-5" scrollable>
          <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>
            {copy.description}
          </p>

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
              {t("capability.credential_docs")}
            </UiLinkButton>
          ) : null}

          <UiField
            htmlFor={credentialId}
            label={copy.label}
            required
          >
            <UiInput
              autoCapitalize="off"
              autoComplete="off"
              autoCorrect="off"
              controlSize="sm"
              data-form-type="other"
              data-lpignore="true"
              id={credentialId}
              name={`${detail.connector_id}-credential`}
              onChange={(event) => setCredential(event.target.value)}
              pattern=".*\S.*"
              placeholder={copy.placeholder}
              required
              spellCheck={false}
              type="password"
              value={credential}
            />
          </UiField>
        </UiDialogBody>

        <UiDialogFooter appearance="plain">
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
            {t("capability.credential_connect")}
          </UiButton>
        </UiDialogFooter>
      </UiDialogFormShell>
    </UiDialogBackdrop>
  );
}
