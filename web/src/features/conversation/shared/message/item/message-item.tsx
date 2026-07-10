/**
 * =====================================================
 * @File   ：message-item.tsx
 * @Date   ：2026-04-16 16:02
 * @Author ：leemysw
 * 2026-04-16 16:02   Create
 * =====================================================
 */

"use client";

import { memo } from "react";

import { cn } from "@/lib/utils";

import { MessageShell } from "../ui/message-primitives";
import { useMessageItemController } from "./controller/use-message-item-controller";
import type { MessageItemProps } from "./message-item-types";
import { MessageAssistantSection } from "./view/message-assistant-section";
import { MessageUserSection } from "./view/message-user-section";

const DEFAULT_HIDDEN_TOOL_NAMES = ["TodoWrite"];

function MessageItemInner({
  compact = false,
  currentAgentName: currentAgentName,
  currentAgentAvatar: currentAgentAvatar,
  workspaceAgentId: workspaceAgentId,
  currentUserAvatar: currentUserAvatar,
  roundId: roundId,
  messages: messages,
  isLastRound: isLastRound,
  isLoading: isLoading,
  runtimePhase: runtimePhase,
  pendingPermissions: pendingPermissions,
  onEditUserMessage: onEditUserMessage,
  onOpenAgentContact: onOpenAgentContact,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  onPermissionResponse: onPermissionResponse,
  canRespondToPermissions: canRespondToPermissions = true,
  permissionReadOnlyReason: permissionReadOnlyReason,
  hiddenToolNames: hiddenToolNames = DEFAULT_HIDDEN_TOOL_NAMES,
  onStopMessage: onStopMessage,
  defaultProcessExpanded: defaultProcessExpanded,
  assistantHeaderAction: assistantHeaderAction,
  assistantContentMode: assistantContentMode = "dm_archived",
  className: className,
}: MessageItemProps) {
  const state = useMessageItemController({
    roundId,
    messages,
    isLastRound,
    isLoading,
    runtimePhase,
    pendingPermissions,
    hiddenToolNames: hiddenToolNames,
    onStopMessage,
    defaultProcessExpanded,
    assistantContentMode: assistantContentMode,
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
        userMessage={state.userMessage}
        userContent={state.userContent}
        userAttachments={state.userAttachments}
        currentUserAvatar={currentUserAvatar}
        copiedUser={state.copiedUser}
        onCopyUser={state.handleCopyUser}
        onEditUserMessage={onEditUserMessage}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
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
        state={state}
      />
    </MessageShell>
  );
}

// 仅在影响视觉输出的关键属性变化时重新渲染，避免流式阶段产生无效更新。
const MessageItem = memo(MessageItemInner, (prev, next) => {
  if (prev.roundId !== next.roundId) return false;
  if (prev.isLastRound !== next.isLastRound) return false;
  if (prev.isLoading !== next.isLoading) return false;
  if (prev.runtimePhase !== next.runtimePhase) return false;
  if (prev.compact !== next.compact) return false;
  if (prev.currentAgentName !== next.currentAgentName) return false;
  if (prev.currentAgentAvatar !== next.currentAgentAvatar) return false;
  if (prev.workspaceAgentId !== next.workspaceAgentId) return false;
  if (prev.currentUserAvatar !== next.currentUserAvatar) return false;
  if (prev.pendingPermissions !== next.pendingPermissions) return false;
  if (prev.canRespondToPermissions !== next.canRespondToPermissions) return false;
  if (prev.permissionReadOnlyReason !== next.permissionReadOnlyReason) return false;
  if (prev.onOpenAgentContact !== next.onOpenAgentContact) return false;
  if (prev.assistantHeaderAction !== next.assistantHeaderAction) return false;
  if (prev.assistantContentMode !== next.assistantContentMode) return false;
  if (prev.className !== next.className) return false;
  if (prev.messages !== next.messages) return false;
  return true;
});

export default MessageItem;
