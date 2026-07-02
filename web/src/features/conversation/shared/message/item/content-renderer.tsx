"use client";

import { useEffect, useState, type Key, type ReactNode } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn } from "@/lib/utils";
import {
  ContentBlock,
  SystemEventContent,
  TaskProgressContent,
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/conversation/permission";

import { AskUserQuestionBlock } from "../blocks/ask-user-question-block";
import { ImageBlock } from "../blocks/image-block";
import { ThinkingBlock } from "../blocks/thinking-block";
import { ToolBlock } from "../blocks/tool-block";
import { ToolUseErrorBlock } from "../blocks/tool-use-error-block";
import { WorkspaceFileArtifactBlock } from "../blocks/workspace-file-artifacts";
import { MarkdownRenderer } from "../markdown/markdown-renderer";
import { MessageActivityState, MessageActivityStatus } from "../ui/message-primitives";
import {
  MessageRail,
  MessageRailBody,
  MessageRailLabel,
} from "../ui/message-rail";
import {
  get_system_message_icon_class_name,
  get_system_message_label_class_name,
} from "./message-item-support";
import { resolve_activity_state } from "./content-renderer-activity";
import { SystemEventIcon, TimelineBlock } from "./content-renderer-timeline";

const API_RETRY_VISIBLE_ATTEMPT = 4;
const MAX_API_RETRY_ERROR_CHARS = 1000;

interface ContentRendererProps {
  content: string | ContentBlock[];
  is_streaming?: boolean;
  streaming_block_indexes?: Set<number>;
  fallback_activity_state?: MessageActivityState | null;
  pending_permissions_by_tool_use_id?: ReadonlyMap<string, PendingPermission>;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  can_respond_to_permissions?: boolean;
  permission_read_only_reason?: string;
  on_open_workspace_file?: (path: string) => void;
  workspace_agent_id?: string | null;
  hidden_tool_names?: string[];
  class_name?: string;
  show_timeline_dots?: boolean;
}

function isHiddenApiRetryBlock(block: SystemEventContent): boolean {
  return block.subtype === "api_retry" &&
    typeof block.attempt === "number" &&
    block.attempt < API_RETRY_VISIBLE_ATTEMPT;
}

function ApiRetrySystemEventBody({ block }: { block: SystemEventContent }) {
  const retryDelayMs =
    typeof block.retry_delay_ms === "number" && block.retry_delay_ms > 0
      ? block.retry_delay_ms
      : 0;
  const [nowMs, setNowMs] = useResettableState(
    Date.now(),
    `${block.timestamp}\x1f${retryDelayMs}`,
  );

  useEffect(() => {
    if (retryDelayMs <= 0) {
      return;
    }
    const intervalId = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(intervalId);
  }, [block.timestamp, retryDelayMs]);

  const retryDueAt = block.timestamp + retryDelayMs;
  const retryInSeconds = Math.max(
    0,
    Math.round((retryDueAt - nowMs) / 1000),
  );
  const retryUnit = retryInSeconds === 1 ? "second" : "seconds";
  const attemptText =
    typeof block.attempt === "number" && typeof block.max_retries === "number"
      ? `(attempt ${block.attempt}/${block.max_retries})`
      : null;
  const retryText = retryDelayMs > 0
    ? `Retrying in ${retryInSeconds} ${retryUnit}...${attemptText ? ` ${attemptText}` : ""}`
    : `Retrying...${attemptText ? ` ${attemptText}` : ""}`;
  const content = block.content.length > MAX_API_RETRY_ERROR_CHARS
    ? `${block.content.slice(0, MAX_API_RETRY_ERROR_CHARS)}...`
    : block.content;

  return (
    <>
      <div>{content}</div>
      <div className="mt-0.5 text-[13px] leading-5 text-(--text-muted)">
        {retryText}
      </div>
    </>
  );
}

export function ContentRenderer(
  {
    content,
    is_streaming: isStreaming = false,
    streaming_block_indexes: streamingBlockIndexes,
    fallback_activity_state: fallbackActivityState,
    pending_permissions_by_tool_use_id: pendingPermissionsByToolUseId,
    on_permission_response: onPermissionResponse,
    can_respond_to_permissions: canRespondToPermissions = true,
    permission_read_only_reason: permissionReadOnlyReason,
    on_open_workspace_file: onOpenWorkspaceFile,
    workspace_agent_id: workspaceAgentId,
    hidden_tool_names: hiddenToolNames = [],
    class_name: className,
    show_timeline_dots: showTimelineDots = false,
  }: ContentRendererProps) {
  // Handle string content (Markdown)
  if (typeof content === 'string') {
    const markdown = (
      <MarkdownRenderer
        content={content}
        is_streaming={isStreaming}
        on_open_workspace_file={onOpenWorkspaceFile}
        workspace_agent_id={workspaceAgentId}
      />
    );

    if (!className) {
      return markdown;
    }

    return (
      <div className={cn(className, showTimelineDots ? "relative before:absolute before:bottom-0 before:left-[5.5px] before:top-0 before:w-px before:bg-(--divider-subtle-color)" : null)}>
        {showTimelineDots ? (
          <TimelineBlock active={isStreaming}>
            {markdown}
          </TimelineBlock>
        ) : (
          markdown
        )}
      </div>
    );
  }

  // Handle structured content (ContentBlock[])
  // 首先构建 tool_use 到 tool_result 的映射
  const toolUseMap = new Map<string, {
    use: ToolUseContent;
    result?: ToolResultContent;
    index: number;
  }>();
  const renderedIndices = new Set<number>();

  // 第一遍：收集所有 tool_use 和对应的 tool_result
  content.forEach((block, index) => {
    if (block.type === 'tool_use') {
      toolUseMap.set(block.id, { use: block, index });
    }
  });

  // 第二遍：匹配 tool_result 到 tool_use
  content.forEach((block, index) => {
    if (block.type === 'tool_result') {
      const toolUseData = toolUseMap.get(block.tool_use_id);
      if (toolUseData) {
        toolUseData.result = block;
        renderedIndices.add(index); // 标记这个 result 已被处理
      }
    }
  });

  // 第三遍：把 task_progress 按 tool_use_id 折叠到对应工具块（子 Agent 实时进度）。
  // 不再单独渲染 task_progress 行，统一并入它所属的 Agent ToolBlock。
  const taskProgressByToolUseId = new Map<string, TaskProgressContent>();
  content.forEach((block) => {
    if (block.type === 'task_progress' && block.tool_use_id) {
      taskProgressByToolUseId.set(block.tool_use_id, block);
    }
  });

  // 只要当前轮次仍在进行，就持续在块尾渲染一个状态行；
  // 不再要求“没有 streaming block”才显示，否则纯文本回复阶段会出现状态空窗。
  const activityState = isStreaming
    ? resolve_activity_state({
      content,
      streaming_block_indexes: streamingBlockIndexes,
      tool_use_map: toolUseMap,
      rendered_indices: renderedIndices,
      fallback_activity_state: fallbackActivityState,
      pending_permissions_by_tool_use_id: pendingPermissionsByToolUseId,
      hidden_tool_names: hiddenToolNames,
    })
    : null;

  return (
    <div className={cn("nexus-chat-block-stack min-w-0 space-y-2.5", className, showTimelineDots ? "relative before:absolute before:bottom-0 before:left-[5.5px] before:top-0 before:w-px before:bg-(--divider-subtle-color)" : null)}>
      {content.map((block, index) => {
        const blockIsStreaming = streamingBlockIndexes?.has(index) ?? false;

        // 跳过已经被组合渲染的 tool_result
        if (renderedIndices.has(index)) {
          return null;
        }

        const wrapBlock = (
          key: Key,
          node: ReactNode,
        ) => {
          if (!showTimelineDots) {
            return <div key={key}>{node}</div>;
          }

          return (
            <TimelineBlock
              key={key}
              active={blockIsStreaming}
            >
              {node}
            </TimelineBlock>
          );
        };

        if (block.type === 'text') {
          if (!block.text.trim()) {
            return null;
          }
          return wrapBlock(
            index,
            <ContentRenderer
              content={block.text}
              is_streaming={blockIsStreaming}
              fallback_activity_state={blockIsStreaming ? "replying" : null}
              on_open_workspace_file={onOpenWorkspaceFile}
              workspace_agent_id={workspaceAgentId}
            />,
          );
        }

        if (block.type === 'tool_use_error') {
          return wrapBlock(index, <ToolUseErrorBlock content={block.content} />);
        }

        if (block.type === 'thinking') {
          if (!block.thinking.trim()) {
            return null;
          }
          return wrapBlock(
            index,
            <ThinkingBlock
              thinking={block.thinking || ''}
              is_streaming={blockIsStreaming}
              workspace_agent_id={workspaceAgentId}
            />,
          );
        }

        if (block.type === 'image') {
          return wrapBlock(
            index,
            <ImageBlock
              block={block}
              on_open_workspace_file={onOpenWorkspaceFile}
              workspace_agent_id={workspaceAgentId}
            />,
          );
        }

        if (block.type === 'system_event') {
          if (isHiddenApiRetryBlock(block)) {
            return null;
          }
          return wrapBlock(index, (
            <MessageRail class_name="min-w-0">
              <MessageRailLabel class_name={cn("flex-1", get_system_message_label_class_name(block.tone))}>
                <span
                  data-timeline-anchor
                  data-timeline-anchor-mode="box"
                  className="flex h-4 w-4 shrink-0 items-center justify-center"
                >
                  <SystemEventIcon
                    icon={block.icon}
                    class_name={cn("h-3 w-3", get_system_message_icon_class_name(block.tone))}
                  />
                </span>
                <span>{block.label}</span>
              </MessageRailLabel>
              <MessageRailBody class_name="pt-1 text-[14px] leading-6 text-(--text-default)">
                {block.subtype === "api_retry" ? (
                  <ApiRetrySystemEventBody block={block} />
                ) : (
                  block.content
                )}
              </MessageRailBody>
            </MessageRail>
          ));
        }

        // task_progress 不再单独渲染：已按 tool_use_id 折叠进对应 Agent ToolBlock。

        if (block.type === 'workspace_file_artifact') {
          return wrapBlock(index, (
            <WorkspaceFileArtifactBlock
              artifact={block}
              on_open_workspace_file={onOpenWorkspaceFile}
            />
          ));
        }

        if (block.type === 'tool_use') {
          // 特殊处理 AskUserQuestion 工具
          if (block.name === 'AskUserQuestion') {
            const toolData = toolUseMap.get(block.id);
            const hasResult = !!toolData?.result;
            const toolResult = toolData?.result as ToolResultContent | undefined;
            const pendingPermission = pendingPermissionsByToolUseId?.get(block.id);
            const isThisToolPending = Boolean(pendingPermission && !hasResult);
            return wrapBlock(index, (
              <div>
                <AskUserQuestionBlock
                  tool_use={block}
                  tool_result={toolResult}
                  is_submitted={hasResult && !toolResult?.is_error}
                  is_ready={Boolean(isThisToolPending)}
                  interaction_disabled={!canRespondToPermissions}
                  interaction_disabled_reason={permissionReadOnlyReason}
                  on_submit={(_, answers) => {
                    if (!pendingPermission) {
                      return false;
                    }
                    return onPermissionResponse?.({
                      request_id: pendingPermission.request_id,
                      decision: 'allow',
                      user_answers: answers,
                    }) ?? false;
                  }}
                />
              </div>
            ));
          }

          // 如果工具在隐藏列表中，则不渲染
          if (hiddenToolNames.includes(block.name)) {
            return null;
          }

          const toolData = toolUseMap.get(block.id);
          const pendingPermission = pendingPermissionsByToolUseId?.get(block.id);
          const isThisToolPendingPermission = Boolean(pendingPermission && !toolData?.result);

          // 确定状态
          let toolStatus: 'pending' | 'running' | 'success' | 'error' | 'waiting_permission' = 'running';
          if (isThisToolPendingPermission) {
            toolStatus = 'waiting_permission';
          } else if (toolData?.result) {
            toolStatus = toolData.result.is_error ? 'error' : 'success';
          }

          return wrapBlock(index, (
            <div className="min-w-0">
              <ToolBlock
                tool_use={block}
                tool_result={toolData?.result}
                live_progress={taskProgressByToolUseId.get(block.id) ?? null}
                status={toolStatus}
                permission_request={isThisToolPendingPermission ? {
                  request_id: pendingPermission!.request_id,
                  tool_input: pendingPermission!.tool_input,
                  risk_level: pendingPermission!.risk_level,
                  risk_label: pendingPermission!.risk_label,
                  summary: pendingPermission!.summary,
                  suggestions: pendingPermission!.suggestions,
                  expires_at: pendingPermission!.expires_at,
                  on_allow: (updatedPermissions) => onPermissionResponse?.({
                    request_id: pendingPermission!.request_id,
                    decision: 'allow',
                    updated_permissions: updatedPermissions,
                  }),
                  on_deny: (updatedPermissions) => onPermissionResponse?.({
                    request_id: pendingPermission!.request_id,
                    decision: 'deny',
                    updated_permissions: updatedPermissions,
                  }),
                } : undefined}
                interaction_disabled={!canRespondToPermissions}
                interaction_disabled_reason={permissionReadOnlyReason}
                on_open_workspace_file={onOpenWorkspaceFile}
                workspace_agent_id={workspaceAgentId}
              />
            </div>
          ));
        }

        // 独立的 tool_result（没有对应的 tool_use）
        if (block.type === 'tool_result') {
          return null;
        }

        return null;
      })}
      {activityState ? (
        <MessageActivityStatus class_name="pt-1" state={activityState} />
      ) : null}
    </div>
  );
}
