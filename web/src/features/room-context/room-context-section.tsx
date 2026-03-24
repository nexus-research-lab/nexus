import { Clock3, FilePlus2, FileText, FolderPlus, FolderTree } from "lucide-react";
import { ReactNode } from "react";

import { WorkspaceFileEntry } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { cn, formatRelativeTime, truncate } from "@/lib/utils";

interface RoomContextSectionProps {
  contextualFiles: WorkspaceFileEntry[];
  current_conversation: Conversation | null;
  file_explorer_content: ReactNode;
  filesystem_error: string | null;
  is_file_explorer_visible: boolean;
  memory_file_count: number;
  on_create_directory: () => void;
  on_create_file: () => void;
  on_open_workspace_file: (path: string) => void;
  on_toggle_file_explorer: () => void;
  total_conversation_count: number;
  active_workspace_path: string | null;
}

export function RoomContextSection({
  contextualFiles,
  current_conversation,
  file_explorer_content,
  filesystem_error,
  is_file_explorer_visible,
  memory_file_count,
  on_create_directory,
  on_create_file,
  on_open_workspace_file,
  on_toggle_file_explorer,
  total_conversation_count,
  active_workspace_path,
}: RoomContextSectionProps) {
  return (
    <section className="border-t workspace-divider px-5 py-5">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/56">
          <FolderTree className="h-3.5 w-3.5" />
          Context
        </div>
        <div className="flex items-center gap-1">
          <button
            aria-label={is_file_explorer_visible ? "收起文件树" : "展开文件树"}
            className="workspace-chip rounded-xl px-3 py-1.5 text-[11px] font-semibold text-slate-700/58 transition-colors hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
            onClick={on_toggle_file_explorer}
            type="button"
          >
            {is_file_explorer_visible ? "收起" : "浏览"}
          </button>
          <button
            aria-label="创建文件"
            className="workspace-chip rounded-xl p-1.5 text-slate-700/58 transition-colors hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
            onClick={on_create_file}
            type="button"
          >
            <FilePlus2 className="h-3.5 w-3.5" />
          </button>
          <button
            aria-label="创建目录"
            className="workspace-chip rounded-xl p-1.5 text-slate-700/58 transition-colors hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
            onClick={on_create_directory}
            type="button"
          >
            <FolderPlus className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      <div className="workspace-card rounded-[28px] px-4 py-4">
        <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-700/54">
          <FileText className="h-3.5 w-3.5" />
          协作上下文
        </div>
        <div className="mt-3 grid grid-cols-2 gap-2">
          <div className="workspace-card radius-shell-sm px-3 py-2">
            <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/50">对话数</p>
            <p className="mt-1 text-sm font-semibold text-slate-900/86">{total_conversation_count}</p>
          </div>
          <div className="workspace-card radius-shell-sm px-3 py-2">
            <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/50">记忆文件</p>
            <p className="mt-1 text-sm font-semibold text-slate-900/86">{memory_file_count}</p>
          </div>
        </div>
        <div className="workspace-card radius-shell-sm mt-3 flex items-center justify-between px-3 py-2">
          <div className="flex items-center gap-2 text-[11px] text-slate-700/54">
            <Clock3 className="h-3.5 w-3.5" />
            <span>当前对话</span>
          </div>
          <div className="text-right">
            <p className="text-[11px] font-medium text-slate-900/84">
              {current_conversation?.message_count ?? 0} 条消息
            </p>
            <p className="text-[11px] text-slate-700/52">
              {current_conversation ? formatRelativeTime(current_conversation.last_activity_at) : "暂无活动"}
            </p>
          </div>
        </div>
      </div>

      {filesystem_error && (
        <div className="radius-shell-sm mt-3 border border-destructive/20 bg-destructive/6 px-3 py-2 text-xs text-destructive">
          {filesystem_error}
        </div>
      )}

      {contextualFiles.length > 0 && (
        <div className="mt-3">
          <p className="mb-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-700/50">
            当前上下文
          </p>
          <div className="space-y-2">
            {contextualFiles.map((file) => {
              const isActive = file.path === active_workspace_path;
              return (
                <button
                  key={file.path}
                  className={cn(
                    "flex w-full items-center gap-3 rounded-[18px] px-3 py-2.5 text-left transition-all duration-300",
                    isActive ? "workspace-card-strong" : "workspace-card hover:-translate-y-0.5",
                  )}
                  onClick={() => on_open_workspace_file(file.path)}
                  type="button"
                >
                  <div className="workspace-chip flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-slate-700/60">
                    <FileText className="h-3.5 w-3.5" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-[12px] font-semibold text-slate-900/84">
                      {file.name}
                    </p>
                    <p className="truncate text-[11px] text-slate-700/50">
                      {truncate(file.path, 28)}
                    </p>
                  </div>
                </button>
              );
            })}
          </div>
        </div>
      )}

      {is_file_explorer_visible && (
        <div className="mt-3">
          <p className="mb-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-700/50">
            工作区文件
          </p>
          <div className="space-y-1">{file_explorer_content}</div>
        </div>
      )}
    </section>
  );
}
