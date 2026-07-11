import { useCallback } from "react";

import { cn } from "@/shared/ui/class-name";

import { MessageStats } from "../../../ui/message-stats";
import { AssistantMessageContent } from "./assistant-message-content";
import {
  AssistantMessageAvatar,
  AssistantMessageHeader,
} from "./assistant-message-header";
import {
  resolveAssistantMessageLayout,
  resolveAssistantMessageScope,
  resolveAssistantDisplayName,
  type AssistantFooterState,
  type MessageAssistantSectionProps,
} from "./assistant-message-model";

export function MessageAssistantSection({
  assistant,
  assistantContentMode,
  assistantHeaderAction,
  canRespondToPermissions,
  compact,
  currentAgentAvatar,
  currentAgentName,
  hiddenToolNames = ["TodoWrite"],
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onPermissionResponse,
  permissionReadOnlyReason,
  workspaceAgentId,
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

  const displayName = resolveAssistantDisplayName(currentAgentName);

  return (
    <div className={cn("nexus-chat-message-section w-full", layout.section)}>
      <div className={cn("w-full", layout.inner)}>
        <div
          className={cn(
            "nexus-chat-assistant-grid group grid min-w-0",
            layout.grid,
          )}
        >
          <AssistantSideAvatar
            avatarUrl={currentAgentAvatar}
            displayName={displayName}
            onOpenContact={openContact}
            visible={layout.showSideAvatar}
          />

          <div className="relative min-w-0">
            <AssistantMessageHeader
              avatarUrl={currentAgentAvatar}
              canStop={assistant.header.canStop}
              compact={compact}
              headerAction={assistantHeaderAction}
              model={assistant.header.model}
              name={currentAgentName}
              onOpenContact={openContact}
              onStop={assistant.header.stop}
              timestamp={assistant.header.timestamp}
            />

            <div
              className={cn(
                "nexus-chat-message-content min-w-0 max-w-full overflow-x-hidden pb-2 pt-1 text-left",
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
            />
          </div>
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

function AssistantSideAvatar({
  avatarUrl,
  displayName,
  onOpenContact,
  visible,
}: {
  avatarUrl?: string | null;
  displayName: string;
  onOpenContact?: () => void;
  visible: boolean;
}) {
  if (!visible) {
    return null;
  }
  return (
    <AssistantMessageAvatar
      avatarUrl={avatarUrl}
      displayName={displayName}
      onOpenContact={onOpenContact}
    />
  );
}

function AssistantFooter({
  activityShowCursor,
  compact,
  footer,
}: {
  activityShowCursor: boolean;
  compact: boolean;
  footer: AssistantFooterState;
}) {
  if (!footer.visible) {
    return null;
  }
  return (
    <MessageStats
      compact={compact}
      copiedAssistant={footer.copied}
      onCopyAssistant={footer.onCopy}
      showCursor={activityShowCursor}
      stats={footer.stats || undefined}
    />
  );
}
