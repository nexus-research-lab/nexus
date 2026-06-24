"use client";

import { useEffect, useState, type Key, type ReactNode } from "react";

import { cn } from "@/lib/utils";
import {
  ContentBlock,
  SystemEventContent,
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

function is_hidden_api_retry_block(block: SystemEventContent): boolean {
  return block.subtype === "api_retry" &&
    typeof block.attempt === "number" &&
    block.attempt < API_RETRY_VISIBLE_ATTEMPT;
}

function ApiRetrySystemEventBody({ block }: { block: SystemEventContent }) {
  const retry_delay_ms =
    typeof block.retry_delay_ms === "number" && block.retry_delay_ms > 0
      ? block.retry_delay_ms
      : 0;
  const [now_ms, set_now_ms] = useState(() => Date.now());

  useEffect(() => {
    if (retry_delay_ms <= 0) {
      return;
    }
    set_now_ms(Date.now());
    const interval_id = window.setInterval(() => set_now_ms(Date.now()), 1000);
    return () => window.clearInterval(interval_id);
  }, [block.timestamp, retry_delay_ms]);

  const retry_due_at = block.timestamp + retry_delay_ms;
  const retry_in_seconds = Math.max(
    0,
    Math.round((retry_due_at - now_ms) / 1000),
  );
  const retry_unit = retry_in_seconds === 1 ? "second" : "seconds";
  const attempt_text =
    typeof block.attempt === "number" && typeof block.max_retries === "number"
      ? `(attempt ${block.attempt}/${block.max_retries})`
      : null;
  const retry_text = retry_delay_ms > 0
    ? `Retrying in ${retry_in_seconds} ${retry_unit}...${attempt_text ? ` ${attempt_text}` : ""}`
    : `Retrying...${attempt_text ? ` ${attempt_text}` : ""}`;
  const content = block.content.length > MAX_API_RETRY_ERROR_CHARS
    ? `${block.content.slice(0, MAX_API_RETRY_ERROR_CHARS)}...`
    : block.content;

  return (
    <>
      <div>{content}</div>
      <div className="mt-0.5 text-[13px] leading-5 text-(--text-muted)">
        {retry_text}
      </div>
    </>
  );
}

export function ContentRenderer(
  {
    content,
    is_streaming = false,
    streaming_block_indexes,
    fallback_activity_state,
    pending_permissions_by_tool_use_id,
    on_permission_response,
    can_respond_to_permissions = true,
    permission_read_only_reason,
    on_open_workspace_file,
    workspace_agent_id,
    hidden_tool_names = [],
    class_name,
    show_timeline_dots = false,
  }: ContentRendererProps) {
  // Handle string content (Markdown)
  if (typeof content === 'string') {
    const markdown = (
      <MarkdownRenderer
        content={content}
        is_streaming={is_streaming}
        on_open_workspace_file={on_open_workspace_file}
        workspace_agent_id={workspace_agent_id}
      />
    );

    if (!class_name) {
      return markdown;
    }

    return (
      <div className={cn(class_name, show_timeline_dots ? "relative before:absolute before:bottom-0 before:left-[5.5px] before:top-0 before:w-px before:bg-(--divider-subtle-color)" : null)}>
        {show_timeline_dots ? (
          <TimelineBlock active={is_streaming}>
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

  // 只要当前轮次仍在进行，就持续在块尾渲染一个状态行；
  // 不再要求“没有 streaming block”才显示，否则纯文本回复阶段会出现状态空窗。
  const activityState = is_streaming
    ? resolve_activity_state({
      content,
      streaming_block_indexes,
      tool_use_map: toolUseMap,
      rendered_indices: renderedIndices,
      fallback_activity_state,
      pending_permissions_by_tool_use_id,
      hidden_tool_names,
    })
    : null;

  return (
    <div className={cn("nexus-chat-block-stack min-w-0 space-y-2.5", class_name, show_timeline_dots ? "relative before:absolute before:bottom-0 before:left-[5.5px] before:top-0 before:w-px before:bg-(--divider-subtle-color)" : null)}>
      {content.map((block, index) => {
        const blockIsStreaming = streaming_block_indexes?.has(index) ?? false;

        // 跳过已经被组合渲染的 tool_result
        if (renderedIndices.has(index)) {
          return null;
        }

        const wrap_block = (
          key: Key,
          node: ReactNode,
        ) => {
          if (!show_timeline_dots) {
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
          return wrap_block(
            index,
            <ContentRenderer
              content={block.text}
              is_streaming={blockIsStreaming}
              fallback_activity_state={blockIsStreaming ? "replying" : null}
              on_open_workspace_file={on_open_workspace_file}
              workspace_agent_id={workspace_agent_id}
            />,
          );
        }

        if (block.type === 'tool_use_error') {
          return wrap_block(index, <ToolUseErrorBlock content={block.content} />);
        }

        if (block.type === 'thinking') {
          if (!block.thinking.trim()) {
            return null;
          }
          return wrap_block(
            index,
            <ThinkingBlock
              thinking={block.thinking || ''}
              is_streaming={blockIsStreaming}
              workspace_agent_id={workspace_agent_id}
            />,
          );
        }

        if (block.type === 'image') {
          return wrap_block(
            index,
            <ImageBlock
              block={block}
              on_open_workspace_file={on_open_workspace_file}
              workspace_agent_id={workspace_agent_id}
            />,
          );
        }

        if (block.type === 'system_event') {
          if (is_hidden_api_retry_block(block)) {
            return null;
          }
          return wrap_block(index, (
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

        if (block.type === 'task_progress') {
          return wrap_block(index, (
            <MessageRail class_name="min-w-0">
              <MessageRailLabel active>
                <span
                  data-timeline-anchor
                  data-timeline-anchor-mode="box"
                  className="flex h-4 w-4 shrink-0 items-center justify-center"
                >
                  <SystemEventIcon
                    icon="progress"
                    class_name="h-3 w-3 text-primary"
                  />
                </span>
                <span className="truncate">{block.last_tool_name || '后台任务'}</span>
                <span className="shrink-0 rounded-[6px] bg-primary/10 px-1.5 py-0.5 text-[10px] font-semibold text-primary">
                  执行中
                </span>
              </MessageRailLabel>
              <MessageRailBody class_name="pt-0.5 text-[12px] leading-5 text-(--text-muted)">
                <span className="block truncate">
                  {block.description || '正在处理中…'}
                </span>
              </MessageRailBody>
            </MessageRail>
          ));
        }

        if (block.type === 'workspace_file_artifact') {
          return wrap_block(index, (
            <WorkspaceFileArtifactBlock
              artifact={block}
              on_open_workspace_file={on_open_workspace_file}
            />
          ));
        }

        if (block.type === 'tool_use') {
          // 特殊处理 AskUserQuestion 工具
          if (block.name === 'AskUserQuestion') {
            const toolData = toolUseMap.get(block.id);
            const hasResult = !!toolData?.result;
            const toolResult = toolData?.result as ToolResultContent | undefined;
            const pending_permission = pending_permissions_by_tool_use_id?.get(block.id);
            const isThisToolPending = Boolean(pending_permission && !hasResult);
            return wrap_block(index, (
              <div>
                <AskUserQuestionBlock
                  tool_use={block}
                  tool_result={toolResult}
                  is_submitted={hasResult && !toolResult?.is_error}
                  is_ready={Boolean(isThisToolPending)}
                  interaction_disabled={!can_respond_to_permissions}
                  interaction_disabled_reason={permission_read_only_reason}
                  on_submit={(_, answers) => {
                    if (!pending_permission) {
                      return false;
                    }
                    return on_permission_response?.({
                      request_id: pending_permission.request_id,
                      decision: 'allow',
                      user_answers: answers,
                    }) ?? false;
                  }}
                />
              </div>
            ));
          }

          // 如果工具在隐藏列表中，则不渲染
          if (hidden_tool_names.includes(block.name)) {
            return null;
          }

          const toolData = toolUseMap.get(block.id);
          const pending_permission = pending_permissions_by_tool_use_id?.get(block.id);
          const isThisToolPendingPermission = Boolean(pending_permission && !toolData?.result);

          // 确定状态
          let toolStatus: 'pending' | 'running' | 'success' | 'error' | 'waiting_permission' = 'running';
          if (isThisToolPendingPermission) {
            toolStatus = 'waiting_permission';
          } else if (toolData?.result) {
            toolStatus = toolData.result.is_error ? 'error' : 'success';
          }

          return wrap_block(index, (
            <div className="min-w-0">
              <ToolBlock
                tool_use={block}
                tool_result={toolData?.result}
                status={toolStatus}
                permission_request={isThisToolPendingPermission ? {
                  request_id: pending_permission!.request_id,
                  tool_input: pending_permission!.tool_input,
                  risk_level: pending_permission!.risk_level,
                  risk_label: pending_permission!.risk_label,
                  summary: pending_permission!.summary,
                  suggestions: pending_permission!.suggestions,
                  expires_at: pending_permission!.expires_at,
                  on_allow: (updated_permissions) => on_permission_response?.({
                    request_id: pending_permission!.request_id,
                    decision: 'allow',
                    updated_permissions,
                  }),
                  on_deny: (updated_permissions) => on_permission_response?.({
                    request_id: pending_permission!.request_id,
                    decision: 'deny',
                    updated_permissions,
                  }),
                } : undefined}
                interaction_disabled={!can_respond_to_permissions}
                interaction_disabled_reason={permission_read_only_reason}
                on_open_workspace_file={on_open_workspace_file}
                workspace_agent_id={workspace_agent_id}
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
