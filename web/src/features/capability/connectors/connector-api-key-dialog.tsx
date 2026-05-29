"use client";

import { ExternalLink, KeyRound, Save } from "lucide-react";
import { type FormEvent, useCallback, useEffect, useState } from "react";

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

interface ConnectorAPIKeyDialogProps {
  detail: ConnectorDetail | null;
  busy: boolean;
  on_close: () => void;
  on_save: (connector_id: string, api_key: string) => void;
}

function get_api_key_description(detail: ConnectorDetail): string {
  if (detail.connector_id === "amap") {
    return "在高德开放平台创建 Web 服务 Key 后粘贴保存，Agent 运行时会直接挂载官方高德 MCP Server。";
  }
  if (detail.connector_id === "didi") {
    return "在滴滴 MCP 服务页面获取 MCP Key 后粘贴保存，Agent 运行时会直接挂载官方 DiDi MCP Server。";
  }
  return "填写此连接器的 API Key 后保存，Agent 运行时会按需挂载对应 MCP Server。";
}

function get_api_key_placeholder(detail: ConnectorDetail): string {
  if (detail.connector_id === "amap") {
    return "高德 Web 服务 Key";
  }
  if (detail.connector_id === "didi") {
    return "滴滴 MCP Key";
  }
  return `${detail.title} API Key`;
}

/** API Key 连接器凭证弹窗。 */
export function ConnectorAPIKeyDialog({
  detail,
  busy,
  on_close,
  on_save,
}: ConnectorAPIKeyDialogProps) {
  const [api_key, set_api_key] = useState("");

  useEffect(() => {
    set_api_key("");
  }, [detail?.connector_id]);

  const handle_submit = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (!detail) return;
      on_save(detail.connector_id, api_key);
    },
    [api_key, detail, on_save],
  );

  if (!detail) return null;

  const can_save = api_key.trim() !== "";

  return (
    <UiDialogBackdrop on_close={on_close}>
      <UiDialogFormShell class_name="max-h-[84vh]" onSubmit={handle_submit} size="sm">
        <UiDialogHeader
          icon={<KeyRound className="h-4 w-4" />}
          icon_class_name="h-9 w-9 rounded-[14px]"
          on_close={on_close}
          subtitle={detail.title}
          title="连接 API Key"
        />

        <UiDialogBody class_name="space-y-3" scrollable>
          <UiPanel class_name="text-[12px] leading-relaxed" padding="sm" variant="inset">
            {get_api_key_description(detail)}
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

          <label className="block space-y-1 text-[12px] font-medium text-(--text-muted)">
            <span>API Key</span>
            <UiInput
              autoCapitalize="off"
              autoComplete="off"
              autoCorrect="off"
              control_size="sm"
              data-form-type="other"
              data-lpignore="true"
              name={`${detail.connector_id}-api-key`}
              onChange={(event) => set_api_key(event.target.value)}
              placeholder={get_api_key_placeholder(detail)}
              spellCheck={false}
              type="password"
              value={api_key}
            />
          </label>
        </UiDialogBody>

        <UiDialogFooter>
          <UiButton
            disabled={busy || !can_save}
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
