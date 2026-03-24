"use client";

import { MarkdownRenderer } from './markdown-renderer';
import { ToolBlock } from './block/tool-block';
import { AskUserQuestionBlock } from './block/ask-user-question-block';
import { CodeBlock } from './block/code-block';
import { ThinkingBlock } from './block/thinking-block';
import { cn } from '@/lib/utils';
import { ContentRendererProps } from '@/types/room-conversation';

export function ContentRenderer(
  {
    content,
    is_streaming = false,
    streaming_block_indexes,
    pending_permission,
    on_permission_response,
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
  const toolUseMap = new Map<string, { use: any; result?: any; index: number }>();
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

  return (
    <div className="min-w-0 space-y-4">
      {content.map((block, index) => {
        const blockIsStreaming = streaming_block_indexes?.has(index) ?? is_streaming;

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

        if (block.type === 'tool_use') {
          // 特殊处理 AskUserQuestion 工具
          if (block.name === 'AskUserQuestion') {
            const toolData = toolUseMap.get(block.id);
            const hasResult = !!toolData?.result;
            // 检查是否正在等待此工具的权限请求
            const isThisToolPending = pending_permission && pending_permission.tool_name === 'AskUserQuestion' && !hasResult;
            return (
              <div key={index}>
                <AskUserQuestionBlock
                  tool_use={block}
                  is_submitted={hasResult}
                  on_submit={(_, answers) => {
                    // 发送 permission_response 并附带用户答案
                    on_permission_response?.({
                      decision: 'allow',
                      user_answers: answers,
                    });
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
          // 判断权限匹配：匹配 + 无结果检查
          const isThisToolPendingPermission = pending_permission &&
            pending_permission.tool_name === block.name && !toolData?.result;

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
                    decision: 'allow',
                    updated_permissions,
                  }),
                  on_deny: (updated_permissions) => on_permission_response?.({
                    decision: 'deny',
                    updated_permissions,
                  }),
                } : undefined}
              />
            </div>
          );
        }

        // 独立的 tool_result（没有对应的 tool_use）
        if (block.type === 'tool_result') {
          return (
            <div key={index} className={cn(
              "radius-shell-md my-2 p-4",
              block.is_error
                ? "workspace-card bg-red-500/5"
                : "workspace-card bg-green-500/5"
            )}>
              <div className="text-xs font-bold uppercase tracking-wider mb-2 flex items-center gap-2">
                {block.is_error ? (
                  <span className="text-red-500">Error</span>
                ) : (
                  <span className="text-green-500">Result</span>
                )}
              </div>
              <div className="max-h-[300px] overflow-y-auto custom-scrollbar">
                {typeof block.content === 'string' ? (
                  <pre className="workspace-card radius-shell-sm p-4 text-xs font-mono whitespace-pre-wrap break-all text-slate-900/80">
                    {block.content}
                  </pre>
                ) : (
                  <CodeBlock language="json" value={JSON.stringify(block.content, null, 2)} />
                )}
              </div>
            </div>
          );
        }

        return null;
      })}
    </div>
  );
}
