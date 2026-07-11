import type { ToolResultContent } from "@/types/conversation/message";

import { ImageBlock } from "../artifact/image-block";
import { CodeBlock } from "@/shared/ui/markdown/code/code-block";
import {
  isImageContent,
  TOOL_DETAIL_SCROLL_CLASS_NAME,
} from "./tool-block-model";

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
      <div className={TOOL_DETAIL_SCROLL_CLASS_NAME}>
        <ToolResultContentView
          content={toolResult.content}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          workspaceAgentId={workspaceAgentId}
        />
      </div>
    </div>
  );
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
