"use client";

import { AskUserQuestionBlock } from "../blocks/ask-user-question-block";
import { CodeBlock } from "../blocks/code-block";
import { ThinkingBlock } from "../blocks/thinking-block";
import { ToolBlock } from "../blocks/tool-block";
import { MarkdownRenderer } from "../markdown/markdown-renderer";
import { MessageActivityState, MessageActivityStatus } from "../ui/message-primitives";
import {
  MessageCallout,
  MessageCalloutTitle,
  MessageRail,
  MessageResultLabel,
} from "../ui/message-rail";
import { ContentBlock, TaskProgressContent, ToolResultContent, ToolUseContent } from "@/types/conversation/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/conversation/permission";

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
  hidden_tool_names?: string[];
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
    hidden_tool_names = [],
  }: ContentRendererProps) {
  // Handle string content (Markdown)
  if (typeof content === 'string') {
    return (
      <MarkdownRenderer
        content={content}
        is_streaming={is_streaming}
        on_open_workspace_file={on_open_workspace_file}
      />
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
    <div className="min-w-0 space-y-2.5">
      {content.map((block, index) => {
        const blockIsStreaming = streaming_block_indexes?.has(index) ?? false;

        // 跳过已经被组合渲染的 tool_result
        if (renderedIndices.has(index)) {
          return null;
        }

        if (block.type === 'text') {
          return (
            <div key={index}>
              <ContentRenderer
                content={block.text}
                is_streaming={blockIsStreaming}
                fallback_activity_state={blockIsStreaming ? "replying" : null}
                on_open_workspace_file={on_open_workspace_file}
              />
            </div>
          );
        }

        if (block.type === 'thinking') {
          return (
            <div key={index}>
              <ThinkingBlock thinking={block.thinking || ''} is_streaming={blockIsStreaming} />
            </div>
          );
        }

        if (block.type === 'task_progress') {
          return (
            <MessageRail key={index}>
              <MessageCallout>
                <MessageCalloutTitle>
                  {block.last_tool_name || '后台任务'} 正在执行
                </MessageCalloutTitle>
                <div className="mt-1 whitespace-pre-wrap break-words text-(--text-muted)">
                  {block.description || '正在处理中…'}
                </div>
              </MessageCallout>
            </MessageRail>
          );
        }

        if (block.type === 'tool_use') {
          // 特殊处理 AskUserQuestion 工具
          if (block.name === 'AskUserQuestion') {
            const toolData = toolUseMap.get(block.id);
            const hasResult = !!toolData?.result;
            const toolResult = toolData?.result as ToolResultContent | undefined;
            const pending_permission = pending_permissions_by_tool_use_id?.get(block.id);
            const isThisToolPending = Boolean(pending_permission && !hasResult);
            return (
              <div key={index}>
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
            );
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

          return (
            <div key={index} className="min-w-0">
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
              />
            </div>
          );
        }

        // 独立的 tool_result（没有对应的 tool_use）
        if (block.type === 'tool_result') {
          return (
            <MessageRail key={index}>
              <div className="ml-4">
                <MessageResultLabel tone={block.is_error ? "error" : "success"}>
                  {block.is_error ? (
                    <span>Error</span>
                  ) : (
                    <span>Result</span>
                  )}
                </MessageResultLabel>
                <div className="max-h-[300px] overflow-y-auto custom-scrollbar">
                  {typeof block.content === 'string' ? (
                    <pre className="rounded-2xl border border-(--divider-subtle-color) bg-(--surface-inset-background) p-3 text-xs text-(--text-default) whitespace-pre-wrap break-words">
                      {block.content}
                    </pre>
                  ) : (
                    <CodeBlock language="json" value={JSON.stringify(block.content, null, 2)} />
                  )}
                </div>
              </div>
            </MessageRail>
          );
        }

        return null;
      })}
      {activityState ? (
        <MessageActivityStatus class_name="pt-1" state={activityState} />
      ) : null}
    </div>
  );
}

function resolve_activity_state({
  content,
  streaming_block_indexes,
  tool_use_map,
  rendered_indices,
  fallback_activity_state,
  pending_permissions_by_tool_use_id,
  hidden_tool_names,
}: {
  content: ContentBlock[];
  streaming_block_indexes?: ReadonlySet<number>;
  tool_use_map: ReadonlyMap<string, {
    use: ToolUseContent;
    result?: ToolResultContent;
    index: number;
  }>;
  rendered_indices: ReadonlySet<number>;
  fallback_activity_state?: MessageActivityState | null;
  pending_permissions_by_tool_use_id?: ReadonlyMap<string, PendingPermission>;
  hidden_tool_names: string[];
}): MessageActivityState {
  const latest_pending_tool = find_latest_pending_tool_use(
    content,
    tool_use_map,
    hidden_tool_names,
  );
  if (latest_pending_tool) {
    const pending_permission = pending_permissions_by_tool_use_id?.get(latest_pending_tool.id);
    if (pending_permission) {
      if (latest_pending_tool.name === 'AskUserQuestion') {
        return 'waiting_input';
      }
      return 'waiting_permission';
    }

    if (latest_pending_tool.name === 'AskUserQuestion') {
      return fallback_activity_state ?? 'thinking';
    }

    return map_tool_name_to_activity_state(latest_pending_tool.name);
  }

  const latest_visible_block = find_latest_visible_block(
    content,
    rendered_indices,
    hidden_tool_names,
  );
  if (!latest_visible_block) {
    return fallback_activity_state ?? 'thinking';
  }

  if (latest_visible_block.type === 'task_progress') {
    return map_progress_to_activity_state(latest_visible_block);
  }

  if (latest_visible_block.type === 'tool_use') {
    if (latest_visible_block.name === 'AskUserQuestion') {
      return pending_permissions_by_tool_use_id?.has(latest_visible_block.id)
        ? 'waiting_input'
        : (fallback_activity_state ?? 'thinking');
    }
    return map_tool_name_to_activity_state(latest_visible_block.name);
  }

  if (latest_visible_block.type === 'thinking') {
    return 'thinking';
  }

  if (latest_visible_block.type === 'text') {
    return has_streaming_text_block(content, streaming_block_indexes) ? 'replying' : (fallback_activity_state ?? 'replying');
  }

  return fallback_activity_state ?? 'thinking';
}

function find_latest_pending_tool_use(
  content: ContentBlock[],
  tool_use_map: ReadonlyMap<string, {
    use: ToolUseContent;
    result?: ToolResultContent;
    index: number;
  }>,
  hidden_tool_names: string[],
): ToolUseContent | null {
  for (let index = content.length - 1; index >= 0; index -= 1) {
    const block = content[index];
    if (block?.type !== 'tool_use') {
      continue;
    }
    if (hidden_tool_names.includes(block.name)) {
      continue;
    }

    const tool_data = tool_use_map.get(block.id);
    if (!tool_data?.result) {
      return block;
    }
  }

  return null;
}

function find_latest_visible_block(
  content: ContentBlock[],
  rendered_indices: ReadonlySet<number>,
  hidden_tool_names: string[],
): ContentBlock | null {
  for (let index = content.length - 1; index >= 0; index -= 1) {
    const block = content[index];
    if (!block) {
      continue;
    }
    if (rendered_indices.has(index)) {
      continue;
    }
    if (block.type === 'tool_use' && hidden_tool_names.includes(block.name)) {
      continue;
    }
    if (block.type === 'text' && !block.text.trim()) {
      continue;
    }
    if (block.type === 'thinking' && !block.thinking.trim()) {
      continue;
    }
    return block;
  }

  return null;
}

function map_progress_to_activity_state(block: TaskProgressContent): MessageActivityState {
  return map_tool_name_to_activity_state(block.last_tool_name ?? null);
}

function map_tool_name_to_activity_state(tool_name?: string | null): MessageActivityState {
  if (!tool_name) {
    return 'executing';
  }

  const browsing_tools = new Set([
    'Read',
    'Glob',
    'LS',
    'Grep',
    'WebSearch',
    'WebFetch',
  ]);

  if (browsing_tools.has(tool_name)) {
    return 'browsing';
  }

  return 'executing';
}

function has_streaming_text_block(
  content: ContentBlock[],
  streaming_block_indexes?: ReadonlySet<number>,
): boolean {
  if (!streaming_block_indexes?.size) {
    return false;
  }

  for (const index of streaming_block_indexes) {
    const block = content[index];
    if (block?.type === 'text' && block.text.trim()) {
      return true;
    }
  }

  return false;
}
