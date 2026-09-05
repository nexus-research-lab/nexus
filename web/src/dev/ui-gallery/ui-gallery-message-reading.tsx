// INPUT: Local completed messages and exact-scope file artifacts in both reading densities.
// OUTPUT: Real User/Assistant reading surfaces and independent file-open observations.
// POS: Development fixture; commands terminate in local state and never mutate a conversation.

import { useState } from "react";
import { FileArtifactBlock } from "@/features/conversation/shared/message/blocks/artifact/file/file-artifact-block";
import { MessageAssistantSection } from "@/features/conversation/shared/message/item/view/assistant/message-assistant-section";
import type { MessageAssistantSectionProps } from "@/features/conversation/shared/message/item/view/assistant/assistant-message-model";
import { MessageUserSection } from "@/features/conversation/shared/message/item/view/user/message-user-section";
import type { UserMessage } from "@/types/conversation/message/entity";

const CONTENT = "Reading layout specimen.";
const USER: UserMessage = {
  message_id: "reading-user", session_key: "gallery-reading", agent_id: "author", round_id: "reading-round", role: "user",
  timestamp: 1788566400000, content: CONTENT,
};
const ASSISTANT: MessageAssistantSectionProps["assistant"] = {
  activity: { emptyStreamStatus: null, label: null, showCursor: false, standalone: false, state: null, toolUseSummary: null },
  direct: { visible: false, projection: { content: [], streamingIndexes: new Set() } },
  final: { content: CONTENT, mentions: [], isStreaming: false, streamingIndexes: new Set(), visible: true },
  footer: { copied: false, goalCompletionReceipt: null, memories: [], stats: null, visible: false },
  header: { agentId: "author", automationTaskName: null, echo: false, canStop: false, handoffReply: null, stop: () => undefined, timestamp: USER.timestamp },
  hidden: false,
  permissions: { all: [], matchedByToolUseId: new Map(), owner: "composer", unmatched: [] },
  process: { anchorRef: { current: null }, expanded: false, projection: { content: [], streamingIndexes: new Set() },
    summary: { kind: "details", latestDetail: null, metrics: [] }, toggle: () => undefined, visible: false },
  showMaxTokensWarning: false,
};

export function MessageReadingGallery() {
  const [openedFiles, setOpenedFiles] = useState<string[]>([]);
  return (
    <section className="min-w-0 space-y-4" data-gallery-message-reading>
      {[true, false].map((compact) => (
        <div className="min-w-0 space-y-3" data-reading-density={compact ? "compact" : "expanded"} key={String(compact)}>
          <MessageUserSection compact={compact} message={USER} />
          <MessageAssistantSection assistant={ASSISTANT} assistantContentMode="dm_archived" canRespondToPermissions={false}
            compact={compact} hiddenToolNames={[]} showHeader={false} workspaceAgentId="other-agent" />
          <FileArtifactBlock compact={compact} path="reports/source.md" workspaceAgentId="author"
            onOpenWorkspaceFile={(path, agentId) => setOpenedFiles((current) => [...current, `${agentId}:${path}`])} />
        </div>
      ))}
      <output data-gallery-reading-files>{JSON.stringify(openedFiles)}</output>
    </section>
  );
}
