import { useCallback } from "react";

import { cn } from "@/shared/ui/class-name";

import { AssistantMessageContent } from "./assistant-message-content";
import { AssistantMessageHeader } from "./assistant-message-header";
import {
  resolveAssistantMessageLayout,
  resolveAssistantMessageScope,
  type AssistantFooterState,
  type MessageAssistantSectionProps,
} from "./assistant-message-model";
import { AssistantMessageStats } from "./assistant-message-stats";

export function MessageAssistantSection({
  assistant,
  assistantContentMode,
  assistantHeaderAction,
  canRespondToPermissions,
  compact,
  currentAgentAvatar,
  currentAgentName,
  hiddenToolNames,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onPermissionResponse,
  permissionReadOnlyReason,
  workspaceAgentId,
  agentMentionDirectory,
}: MessageAssistantSectionProps) {
  const layout = resolveAssistantMessageLayout(compact);
  const scope = resolveAssistantMessageScope({
    assistantAgentId: assistant.header.agentId,
    hasContactAction: Boolean(onOpenAgentContact),
    workspaceAgentId,
  });
  const openContact = useOpenAgentContact(scope, onOpenAgentContact);

  if (assistant.hidden) {
    return null;
  }

  return (
    <div className={cn("nexus-chat-message-section w-full", layout.section)}>
      <div className={cn("w-full", layout.inner)}>
        <div className="nexus-chat-assistant group relative min-w-0">
          <AssistantMessageHeader
            avatarUrl={currentAgentAvatar}
            canStop={assistant.header.canStop}
            compact={compact}
            headerAction={assistantHeaderAction}
            model={assistant.header.model}
            name={currentAgentName}
            onOpenContact={openContact}
            onStop={assistant.header.stop}
            showMetadata={layout.showMetadata}
            timestamp={assistant.header.timestamp}
          />

          <div
            className={cn(
              "nexus-chat-message-content min-w-0 max-w-full overflow-x-hidden pb-2 text-left",
              layout.content,
            )}
            ref={assistant.layout.contentAreaRef}
            style={assistant.layout.contentAreaStyle}
          >
            <AssistantMessageContent
              activity={assistant.activity}
              direct={assistant.direct}
              environment={{
                canRespondToPermissions,
                hiddenToolNames,
                mode: assistantContentMode,
                onOpenWorkspaceFile,
                onPermissionResponse,
                permissionReadOnlyReason,
                workspaceAgentId: scope.contentWorkspaceAgentId,
                agentMentionDirectory,
                onOpenAgentContact,
              }}
              final={assistant.final}
              permissions={assistant.permissions}
              process={assistant.process}
              showMaxTokensWarning={assistant.showMaxTokensWarning}
            />
          </div>

          <AssistantFooter
            activityShowCursor={assistant.activity.showCursor}
            compact={compact}
            footer={assistant.footer}
            model={assistant.header.model}
          />
        </div>
      </div>
    </div>
  );
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
  activityShowCursor,
  compact,
  footer,
  model,
}: {
  activityShowCursor: boolean;
  compact: boolean;
  footer: AssistantFooterState;
  model?: string;
}) {
  if (!footer.visible) {
    return null;
  }
  return (
    <AssistantMessageStats
      compact={compact}
      copied={footer.copied}
      onCopy={footer.onCopy}
      stats={footer.stats}
      streaming={activityShowCursor}
      model={model}
    />
  );
}
