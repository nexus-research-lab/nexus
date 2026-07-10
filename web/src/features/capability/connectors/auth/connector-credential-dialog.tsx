"use client";

import { ExternalLink, KeyRound, Save } from "lucide-react";
import { type FormEvent, useCallback } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { UiButton, UiLinkButton } from "@/shared/ui/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
} from "@/shared/ui/dialog/dialog";
import { UiInput } from "@/shared/ui/form-control";
import { UiPanel } from "@/shared/ui/panel";
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
  title: string;
};

const CONNECTOR_CREDENTIAL_COPY: Record<string, Partial<CredentialCopy>> = {
  amap: {
    description: "在高德开放平台创建 Web 服务 Key 后粘贴保存，Agent 运行时会直接挂载官方高德 MCP Server。",
    placeholder: "高德 Web 服务 Key",
  },
  didi: {
    description: "在滴滴 MCP 服务页面获取 MCP Key 后粘贴保存，Agent 运行时会直接挂载官方 DiDi MCP Server。",
    placeholder: "滴滴 MCP Key",
  },
  "dingtalk-ai-table": {
    description: "在钉钉 AI 表格 MCP 广场获取 Streamable HTTP URL 后粘贴保存，Agent 运行时会直接挂载这个远程 MCP Server。",
    label: "MCP Server URL",
    placeholder: "钉钉 AI 表格 Streamable HTTP URL",
    title: "连接 MCP Server URL",
  },
  "tencent-docs": {
    description: "在腾讯文档 MCP 授权页获取个人 Token 后粘贴保存，Agent 运行时会通过 Authorization header 挂载官方腾讯文档 MCP。",
    placeholder: "腾讯文档个人 Token",
  },
  yuque: {
    description: "在语雀个人设置中获取 Personal Token 后粘贴保存，Agent 运行时会启动官方 yuque-mcp 并注入该 Token。",
    placeholder: "语雀 Personal Token",
  },
};

function getCredentialCopy(detail: ConnectorDetail): CredentialCopy {
  const label = getDirectCredentialLabel(detail.auth_type);
  return {
    description: `填写此连接器的 ${label} 后保存，Agent 运行时会按需挂载对应 MCP Server。`,
    label,
    placeholder: `${detail.title} ${label}`,
    title: `连接 ${label}`,
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
      if (!detail) return;
      onSave(detail.connector_id, credential);
    },
    [credential, detail, onSave],
  );

  if (!detail) return null;

  const copy = getCredentialCopy(detail);
  const canSave = credential.trim() !== "";

  return (
    <UiDialogBackdrop onClose={onClose}>
      <UiDialogFormShell className="max-h-[84vh]" onSubmit={handleSubmit} size="sm">
        <UiDialogHeader
          icon={<KeyRound className="h-4 w-4" />}
          iconClassName="h-9 w-9 rounded-[14px]"
          onClose={onClose}
          subtitle={detail.title}
          title={copy.title}
        />

        <UiDialogBody className="space-y-3" scrollable>
          <UiPanel className="text-[12px] leading-relaxed" padding="sm" variant="inset">
            {copy.description}
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

          <label className="block space-y-1 text-[12px] font-medium text-(--text-muted)">
            <span>{copy.label}</span>
            <UiInput
              autoCapitalize="off"
              autoComplete="off"
              autoCorrect="off"
              controlSize="sm"
              data-form-type="other"
              data-lpignore="true"
              name={`${detail.connector_id}-credential`}
              onChange={(event) => setCredential(event.target.value)}
              placeholder={copy.placeholder}
              spellCheck={false}
              type="password"
              value={credential}
            />
          </label>
        </UiDialogBody>

        <UiDialogFooter>
          <UiButton
            disabled={busy || !canSave}
            size="sm"
            tone="primary"
            type="submit"
            variant="solid"
          >
            <Save className="h-3.5 w-3.5" />
            保存并连接
          </UiButton>
        </UiDialogFooter>
      </UiDialogFormShell>
    </UiDialogBackdrop>
  );
}
