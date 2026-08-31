/**
 * INPUT: Provider 原生工具结果与可选工作区文件打开能力。
 * OUTPUT: 普通结果明细、按需完整大结果，或有界的 mutation 拒绝/过期原因与稳定 reason code。
 * POS: ToolBlock 展开内容；历史大结果只在展开后读取，复制动作独立读取完整内容。
 */
import { useEffect, useState } from "react";

import type {
  ImageContent,
  ToolResultContent,
} from "@/types/conversation/message/content";

import { ImageBlock } from "../artifact/image/image-block";
import { CodeBlock } from "@/shared/ui/markdown/code/code-block";
import { useI18n } from "@/shared/i18n/i18n-context";
import { projectToolResultMutation } from "../../tool-result-semantic-model";
import { getSessionMessageDetailApi } from "@/lib/api/conversation/session-api";
import { MessageDetailScroll } from "../../ui/message-rail";

interface ToolBlockResultProps {
  onOpenWorkspaceFile?: (path: string) => void;
  toolResult: ToolResultContent;
  workspaceAgentId?: string | null;
}

export function ToolBlockResult({
  onOpenWorkspaceFile,
  toolResult,
  workspaceAgentId,
}: ToolBlockResultProps) {
  const { t } = useI18n();
  const detail = useToolResultDetail(toolResult);
  return (
    <div className="message-cjk-font min-w-0">
      <MessageDetailScroll>
        {detail.loading && detail.toolResult.content != null ? (
          <p className="mb-1 text-xs text-(--text-muted)">
            {t("message.tool_detail_loading")}
          </p>
        ) : null}
        {detail.error && detail.toolResult.content != null ? (
          <p className="mb-1 text-xs text-(--destructive)">
            {t("message.tool_detail_error")}
          </p>
        ) : null}
        <ToolResultContentView
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          detailError={detail.error}
          detailLoading={detail.loading}
          toolResult={detail.toolResult}
          workspaceAgentId={workspaceAgentId}
        />
      </MessageDetailScroll>
    </div>
  );
}

function ToolResultContentView({
  detailError,
  detailLoading,
  onOpenWorkspaceFile,
  toolResult,
  workspaceAgentId,
}: {
  detailError: boolean;
  detailLoading: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  toolResult: ToolResultContent;
  workspaceAgentId?: string | null;
}) {
  const { t } = useI18n();
  const mutation = projectToolResultMutation(toolResult);
  if (mutation?.outcome === "rejected") {
    return (
      <div
        className="rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_6%,transparent)] px-3 py-2"
        data-tool-result-semantic-outcome="rejected"
      >
        <p className="text-xs leading-5 text-(--destructive)">
          {mutation.message || t("message.tool_rejection_without_detail")}
        </p>
        {mutation.reasonCode ? (
          <p className="mt-1 font-mono text-[10px] text-(--text-soft)">
            {mutation.reasonCode}
          </p>
        ) : null}
      </div>
    );
  }
  if (mutation?.outcome === "superseded") {
    return (
      <div
        className="rounded-[10px] border border-(--divider-subtle-color) bg-(--surface-muted-background) px-3 py-2"
        data-tool-result-semantic-outcome="superseded"
      >
        <p className="text-xs leading-5 text-(--text-muted)">
          {mutation.message || t("message.tool_superseded_without_detail")}
        </p>
        {mutation.reasonCode ? (
          <p className="mt-1 font-mono text-[10px] text-(--text-soft)">
            {mutation.reasonCode}
          </p>
        ) : null}
      </div>
    );
  }

  if (detailLoading && toolResult.content == null) {
    return <p className="text-xs text-(--text-muted)">{t("message.tool_detail_loading")}</p>;
  }
  if (detailError && toolResult.content == null) {
    return <p className="text-xs text-(--destructive)">{t("message.tool_detail_error")}</p>;
  }

  const content = toolResult.content;
  if (typeof content === "string") {
    return (
      <pre className="message-cjk-font px-0 py-0 text-xs whitespace-pre-wrap break-all text-(--text-strong)">
        {content}
      </pre>
    );
  }

  if (Array.isArray(content) && content.some(isImageContent)) {
    return (
      <div className="space-y-2">
        {content.map((item, index) => (
          isImageContent(item) ? (
            <ImageBlock
              key={`image-${index}`}
              block={item}
              onOpenWorkspaceFile={onOpenWorkspaceFile}
              workspaceAgentId={workspaceAgentId}
            />
          ) : (
            <CodeBlock
              key={`data-${index}`}
              language="json"
              value={JSON.stringify(item, null, 2)}
            />
          )
        ))}
      </div>
    );
  }

  return <CodeBlock language="json" value={JSON.stringify(content, null, 2)} />;
}

function useToolResultDetail(toolResult: ToolResultContent): {
  error: boolean;
  loading: boolean;
  toolResult: ToolResultContent;
} {
  const detailRef = toolResult.detail_ref?.trim() ?? "";
  const sessionKey = toolResult.detail_session_key?.trim() ?? "";
  const [state, setState] = useState<{
    content: unknown;
    error: boolean;
    key: string;
    loading: boolean;
  }>({ content: undefined, error: false, key: "", loading: false });
  const key = `${sessionKey}:${detailRef}`;

  useEffect(() => {
    if (!sessionKey || !detailRef) {
      return;
    }
    const controller = new AbortController();
    setState({ content: undefined, error: false, key, loading: true });
    void getSessionMessageDetailApi(sessionKey, detailRef, controller.signal)
      .then((detail) => {
        setState({ content: detail.content, error: false, key, loading: false });
      })
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          console.error("[conversation] Failed to load tool detail", error);
          setState({ content: undefined, error: true, key, loading: false });
        }
      });
    return () => controller.abort();
  }, [detailRef, key, sessionKey]);

  const current = state.key === key ? state : {
    content: undefined,
    error: false,
    loading: Boolean(sessionKey && detailRef),
  };
  return {
    error: current.error,
    loading: current.loading,
    toolResult: current.content === undefined
      ? toolResult
      : { ...toolResult, content: current.content },
  };
}

function isImageContent(value: unknown): value is ImageContent {
  return Boolean(
    value &&
    typeof value === "object" &&
    (value as { type?: unknown }).type === "image",
  );
}
