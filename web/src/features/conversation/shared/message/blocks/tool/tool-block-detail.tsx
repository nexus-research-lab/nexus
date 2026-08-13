/**
 * INPUT: Provider 原生工具结果与可选工作区文件打开能力。
 * OUTPUT: 普通结果明细，或有界的 mutation 拒绝/过期原因与稳定 reason code。
 * POS: ToolBlock 展开内容；完整原始结果仍由复制动作保留。
 */
import type { PropsWithChildren } from "react";

import type {
  ImageContent,
  ToolResultContent,
} from "@/types/conversation/message/content";

import { ImageBlock } from "../artifact/image/image-block";
import { CodeBlock } from "@/shared/ui/markdown/code/code-block";
import { useI18n } from "@/shared/i18n/i18n-context";
import { projectToolResultMutation } from "../../tool-result-semantic-model";

const TOOL_DETAIL_SCROLL_CLASS_NAME =
  "min-w-0 max-h-[18rem] overflow-auto overscroll-contain custom-scrollbar";

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
  return (
    <div className="message-cjk-font ml-7 mt-2 min-w-0">
      <ToolBlockDetailScroll>
        <ToolResultContentView
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          toolResult={toolResult}
          workspaceAgentId={workspaceAgentId}
        />
      </ToolBlockDetailScroll>
    </div>
  );
}

export function ToolBlockDetailScroll({ children }: PropsWithChildren) {
  return <div className={TOOL_DETAIL_SCROLL_CLASS_NAME}>{children}</div>;
}

function ToolResultContentView({
  onOpenWorkspaceFile,
  toolResult,
  workspaceAgentId,
}: {
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

function isImageContent(value: unknown): value is ImageContent {
  return Boolean(
    value &&
    typeof value === "object" &&
    (value as { type?: unknown }).type === "image",
  );
}
