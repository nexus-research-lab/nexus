import { memo } from "react";

import { cn } from "@/shared/ui/class-name";

import { MessageShell } from "../ui/message-primitives";
import { useMessageItemController } from "./controller/use-message-item-controller";
import type { MessageItemProps } from "./message-item-types";
import { MessageAssistantSection } from "./view/assistant/message-assistant-section";
import { MessageUserSection } from "./view/user/message-user-section";

const DEFAULT_HIDDEN_TOOL_NAMES = ["TodoWrite"];

function MessageItemInner({
  compact = false,
  currentAgentName,
  currentAgentAvatar,
  workspaceAgentId,
  currentUserAvatar,
  roundId,
  messages,
  isLastRound,
  isLoading,
  runtimePhase,
  pendingPermissions,
  onEditUserMessage,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onPermissionResponse,
  canRespondToPermissions = true,
  permissionReadOnlyReason,
  hiddenToolNames = DEFAULT_HIDDEN_TOOL_NAMES,
  onStopMessage,
  defaultProcessExpanded,
  assistantHeaderAction,
  assistantContentMode = "dm_archived",
  className,
}: MessageItemProps) {
  const state = useMessageItemController({
    roundId,
    messages,
    isLastRound,
    isLoading,
    runtimePhase,
    pendingPermissions,
    hiddenToolNames,
    onStopMessage,
    defaultProcessExpanded,
    assistantContentMode,
  });

  return (
    <MessageShell
      className={cn(
        "nexus-chat-message-round animate-in fade-in slide-in-from-bottom-2 space-y-2 py-3 duration-300",
        compact ? "nexus-chat-message-round-compact" : "nexus-chat-message-round-expanded",
        className,
      )}
      separated={!compact}
    >
      <MessageUserSection
        compact={compact}
        currentUserAvatar={currentUserAvatar}
        onEditUserMessage={onEditUserMessage}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        user={state.user}
        workspaceAgentId={workspaceAgentId}
      />

      <MessageAssistantSection
        compact={compact}
        currentAgentName={currentAgentName}
        currentAgentAvatar={currentAgentAvatar}
        canRespondToPermissions={canRespondToPermissions}
        permissionReadOnlyReason={permissionReadOnlyReason}
        onPermissionResponse={onPermissionResponse}
        onOpenAgentContact={onOpenAgentContact}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        workspaceAgentId={workspaceAgentId}
        hiddenToolNames={hiddenToolNames}
        assistantHeaderAction={assistantHeaderAction}
        assistantContentMode={assistantContentMode}
        assistant={state.assistant}
      />
    </MessageShell>
  );
}

// 默认浅比较覆盖完整 Props 协议，避免手写白名单遗漏动作回调并保留旧闭包。
export const MessageItem = memo(MessageItemInner);
