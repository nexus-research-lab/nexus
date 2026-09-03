// INPUT: 需要直接凭证的 Connector、提交状态与保存/关闭动作。
// OUTPUT: 只呈现一段必要说明、凭证字段和连接动作的 plain 表单弹窗。
// POS: Connector 直接凭证的人机边界，不解释 runtime 或 MCP 内部装配细节。
"use client";

import { ExternalLink } from "lucide-react";
import { type FormEvent, useCallback } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
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

const CONNECTOR_CREDENTIAL_COPY: Record<string, Partial<CredentialCopy>> = {
  amap: {
    description: "粘贴高德开放平台的 Web 服务 Key。",
    placeholder: "高德 Web 服务 Key",
  },
  didi: {
    description: "粘贴滴滴 MCP 服务页面提供的 MCP Key。",
    placeholder: "滴滴 MCP Key",
  },
  "dingtalk-ai-table": {
    description: "粘贴钉钉 AI 表格提供的 Streamable HTTP URL。",
    label: "MCP Server URL",
    placeholder: "钉钉 AI 表格 Streamable HTTP URL",
  },
  "tencent-docs": {
    description: "粘贴腾讯文档 MCP 授权页提供的个人 Token。",
    placeholder: "腾讯文档个人 Token",
  },
  yuque: {
    description: "粘贴语雀个人设置中的 Personal Token。",
    placeholder: "语雀 Personal Token",
  },
};

function getCredentialCopy(detail: ConnectorDetail): CredentialCopy {
  const label = getDirectCredentialLabel(detail.auth_type);
  return {
    description: `填写 ${label} 以连接 ${detail.title}。`,
    label,
    placeholder: `${detail.title} ${label}`,
    ...CONNECTOR_CREDENTIAL_COPY[detail.connector_id],
  };
}

/** 直接凭证连接器弹窗。 */
export function ConnectorCredentialDialog({
  detail,
  busy,
  onClose,
  onSave,
}: ConnectorCredentialDialogProps) {
  const [credential, setCredential] = useResettableState("", detail?.connector_id ?? null);

  const handleSubmit = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (!detail || !credential.trim()) return;
      onSave(detail.connector_id, credential.trim());
    },
    [credential, detail, onSave],
  );

  if (!detail) return null;

  const copy = getCredentialCopy(detail);
  return (
    <UiDialogBackdrop onClose={onClose}>
      <UiDialogFormShell
        onSubmit={handleSubmit}
        size="sm"
        viewport="compactMax"
      >
        <UiDialogHeader
          appearance="plain"
          onClose={onClose}
          title={`连接 ${detail.title}`}
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
              查看文档
            </UiLinkButton>
          ) : null}

          <UiField
            htmlFor={`${detail.connector_id}-credential`}
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
              id={`${detail.connector_id}-credential`}
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
            取消
          </UiButton>
          <UiButton
            disabled={busy}
            size="sm"
            tone="primary"
            type="submit"
            variant="solid"
          >
            连接
          </UiButton>
        </UiDialogFooter>
      </UiDialogFormShell>
    </UiDialogBackdrop>
  );
}
