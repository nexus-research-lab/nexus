import type { PropsWithChildren } from "react";

import type {
  ImageContent,
  ToolResultContent,
} from "@/types/conversation/message/content";

import { ImageBlock } from "../artifact/image-block";
import { CodeBlock } from "@/shared/ui/markdown/code/code-block";

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
          content={toolResult.content}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
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
  content,
  onOpenWorkspaceFile,
  workspaceAgentId,
}: {
  content: ToolResultContent["content"];
  onOpenWorkspaceFile?: (path: string) => void;
  workspaceAgentId?: string | null;
}) {
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
