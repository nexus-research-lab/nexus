/**
 * INPUT: 一个根轮次的 durable 消息与运行态。
 * OUTPUT: 按展示契约渲染用户补充与唯一 assistant 输出。
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
  animateEntry = true,
  compact = false,
  currentAgentName,
  currentAgentAvatar,
  workspaceAgentId,
  roundId,
  messages,
  isLastRound,
  isLoading,
  activityState,
  runtimePhase,
  unresolvedToolStatus,
  pendingPermissions,
  onEditUserMessage,
  onForkConversation,
  onOpenAgentContact,
  onOpenSubagentTask,
  onOpenWorkspaceFile,
  onPermissionResponse,
  canRespondToPermissions = true,
  permissionReadOnlyReason,
  hiddenToolNames = CONVERSATION_TASK_TOOL_NAMES,
  onStopMessage,
  defaultProcessExpanded,
  assistantHeaderAction,
  assistantEmptyState,
  assistantContentMode = "dm_archived",
  showAssistantHeader = true,
  showUserMessages = true,
  className,
  agentMentionDirectory,
}: MessageItemProps) {
  const state = useMessageItemController({
    roundId,
    messages,
    isLastRound,
    isLoading,
    activityState,
    runtimePhase,
    pendingPermissions,
    hiddenToolNames,
    onForkConversation,
    onStopMessage,
    defaultProcessExpanded,
    assistantContentMode,
  });

  return (
    <div
      className={cn(
        "nexus-chat-message-round w-full min-w-0 space-y-1 py-3",
        animateEntry && "animate-in fade-in slide-in-from-bottom-2 duration-300",
        compact ? "nexus-chat-message-round-compact" : "nexus-chat-message-round-expanded",
        className,
      )}
    >
      {showUserMessages
        ? state.userMessages.map((message) => (
            <MessageUserSection
              compact={compact}
              agentMentionDirectory={agentMentionDirectory}
              key={message.client_message_id?.trim() || message.message_id}
              message={message}
              onEditUserMessage={
                state.userMessages.length === 1
                  ? onEditUserMessage
                  : undefined
              }
              onOpenWorkspaceFile={onOpenWorkspaceFile}
              onOpenAgentContact={onOpenAgentContact}
              workspaceAgentId={workspaceAgentId}
            />
          ))
        : null}

      <MessageAssistantSection
        compact={compact}
        currentAgentName={currentAgentName}
        currentAgentAvatar={currentAgentAvatar}
        canRespondToPermissions={canRespondToPermissions}
        permissionReadOnlyReason={permissionReadOnlyReason}
        onPermissionResponse={onPermissionResponse}
        onOpenAgentContact={onOpenAgentContact}
        onOpenSubagentTask={onOpenSubagentTask}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        workspaceAgentId={workspaceAgentId}
        unresolvedToolStatus={unresolvedToolStatus}
        hiddenToolNames={hiddenToolNames}
        assistantHeaderAction={assistantHeaderAction}
        assistantEmptyState={assistantEmptyState}
        assistantContentMode={assistantContentMode}
        showHeader={showAssistantHeader}
        agentMentionDirectory={agentMentionDirectory}
        assistant={state.assistant}
      />
    </div>
  );
}

// 默认浅比较覆盖完整 Props 协议，避免手写白名单遗漏动作回调并保留旧闭包。
export const MessageItem = memo(MessageItemInner);
