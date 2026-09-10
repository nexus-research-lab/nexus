/**
 * INPUT: controller 投影的 Assistant 状态、Agent 目录与视图动作。
 * OUTPUT: 消息头、过程/最终内容、活动与页脚的唯一装配。
 * POS: MessageItem controller 与 Assistant 子视图之间的展示边界。
 */
import { useCallback } from "react";

import { cn } from "@/shared/ui/class-name";

import { resolveAssistantMessageLayout } from "../message-reading-layout";
import { AssistantMessageContent } from "./assistant-message-content";
import { AssistantMessageHeader } from "./assistant-message-header";
import {
  resolveAssistantMessageScope,
  type AssistantFooterState,
  type MessageAssistantSectionProps,
} from "./assistant-message-model";
import { AssistantMessageStats } from "./assistant-message-stats";

export function MessageAssistantSection({
  assistant,
  assistantContentMode,
  assistantEmptyState,
  assistantHeaderAction,
  canRespondToPermissions,
  compact,
  currentAgentAvatar,
  currentAgentName,
  hiddenToolNames,
  onOpenAgentContact,
  onOpenSubagentTask,
  onOpenWorkspaceFile,
  onPermissionResponse,
  permissionReadOnlyReason,
  showHeader,
  workspaceAgentId,
  unresolvedToolStatus,
  agentMentionDirectory,
}: MessageAssistantSectionProps) {
  const layout = resolveAssistantMessageLayout(compact);
  const showEmptyState = assistantEmptyState != null
    && !hasAssistantBodyContent(assistant);
  const scope = resolveAssistantMessageScope({
    assistantAgentId: assistant.header.agentId,
    hasContactAction: Boolean(onOpenAgentContact),
    workspaceAgentId,
  });
  const openContact = useOpenAgentContact(scope, onOpenAgentContact);

  if (assistant.hidden && !showEmptyState) {
    return null;
  }

  return (
    <div className={cn("nexus-chat-message-section w-full", layout.section)}>
      <div className={cn("w-full", layout.inner)}>
        <div className="nexus-chat-assistant group relative min-w-0">
          {showHeader ? (
            <AssistantMessageHeader
              agentMentionDirectory={agentMentionDirectory}
              avatarUrl={currentAgentAvatar}
              automationTaskName={assistant.header.automationTaskName}
              canStop={assistant.header.canStop}
              compact={compact}
              echo={assistant.header.echo}
              headerAction={assistantHeaderAction}
              handoffReplySourceAgentId={
                assistant.header.handoffReply?.source_agent_id ?? null
              }
              model={assistant.model}
              name={currentAgentName}
              onOpenContact={openContact}
              onStop={assistant.header.stop}
              showMetadata={layout.showMetadata}
              timestamp={assistant.header.timestamp}
            />
          ) : null}

          <div
            className={cn(
              "nexus-chat-message-content min-w-0 max-w-full overflow-x-hidden pb-2 text-left",
              layout.content,
            )}
          >
            {showEmptyState ? (
              assistantEmptyState
            ) : (
              <AssistantMessageContent
                activity={assistant.activity}
                direct={assistant.direct}
                environment={{
                  canRespondToPermissions,
                  hiddenToolNames,
                  mode: assistantContentMode,
                  onOpenSubagentTask,
                  onOpenWorkspaceFile,
                  onPermissionResponse,
                  permissionReadOnlyReason,
                  unresolvedToolStatus,
                  workspaceAgentId: scope.contentWorkspaceAgentId,
                  agentMentionDirectory,
                  onOpenAgentContact,
                }}
                final={assistant.final}
                permissions={assistant.permissions}
                process={assistant.process}
                showMaxTokensWarning={assistant.showMaxTokensWarning}
              />
            )}
          </div>

          <AssistantFooter
            footer={assistant.footer}
            model={assistant.model}
          />
        </div>
      </div>
    </div>
  );
}

function hasAssistantBodyContent(
  assistant: MessageAssistantSectionProps["assistant"],
): boolean {
  return [
    assistant.activity.emptyStreamStatus != null,
    assistant.activity.standalone,
    assistant.direct.visible,
    assistant.final.visible,
    assistant.process.visible,
    assistant.permissions.unmatched.length > 0,
    assistant.showMaxTokensWarning,
  ].some(Boolean);
}

function useOpenAgentContact(
  scope: ReturnType<typeof resolveAssistantMessageScope>,
  onOpenAgentContact?: (agentId: string) => void,
) {
  const handleOpenAgentContact = useCallback(() => {
    if (scope.contactAgentId) {
      onOpenAgentContact?.(scope.contactAgentId);
    }
  }, [onOpenAgentContact, scope.contactAgentId]);
  return scope.canOpenContact ? handleOpenAgentContact : undefined;
}

function AssistantFooter({
  footer,
  model,
}: {
  footer: AssistantFooterState;
  model?: string;
}) {
  if (!footer.visible) {
    return null;
  }
  return (
    <AssistantMessageStats
      copied={footer.copied}
      goalCompletionReceipt={footer.goalCompletionReceipt}
      memories={footer.memories}
      onCopy={footer.onCopy}
      onFork={footer.onFork}
      stats={footer.stats}
      model={model}
    />
  );
}
