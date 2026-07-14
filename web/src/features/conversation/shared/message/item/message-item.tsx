/**
 * INPUT: 一个根轮次的 durable 消息与运行态。
 * OUTPUT: 所有用户补充按时间排列，其后仅渲染一次 assistant 输出。
 * POS: DM / Room 共用轮次视图，用户消息数量不在调用方分支处理。
 */
import { memo } from "react";

import { cn } from "@/shared/ui/class-name";
import { CONVERSATION_TASK_TOOL_NAMES } from "@/features/conversation/shared/todos/task-tool-names";

import { useMessageItemController } from "./controller/use-message-item-controller";
import type { MessageItemProps } from "./message-item-types";
import { MessageAssistantSection } from "./view/assistant/message-assistant-section";
import { MessageUserSection } from "./view/user/message-user-section";

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
  hiddenToolNames = CONVERSATION_TASK_TOOL_NAMES,
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
    <div
      className={cn(
        "nexus-chat-message-round w-full min-w-0 animate-in fade-in slide-in-from-bottom-2 space-y-2 py-3 duration-300",
        compact ? "nexus-chat-message-round-compact" : "nexus-chat-message-round-expanded",
        !compact && "border-b border-(--divider-subtle-color)",
        className,
      )}
    >
      {state.userMessages.map((message) => (
        <MessageUserSection
          compact={compact}
          currentUserAvatar={currentUserAvatar}
          key={message.message_id}
          message={message}
          onEditUserMessage={
            state.userMessages.length === 1
              ? onEditUserMessage
              : undefined
          }
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          workspaceAgentId={workspaceAgentId}
        />
      ))}

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
    </div>
  );
}

// 默认浅比较覆盖完整 Props 协议，避免手写白名单遗漏动作回调并保留旧闭包。
export const MessageItem = memo(MessageItemInner);
