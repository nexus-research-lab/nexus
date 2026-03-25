import { useState } from "react";
import { BrainCircuit, FilePlus2, FileText, FolderPlus, FolderTree, MessageSquarePlus, Trash2 } from "lucide-react";

import { WorkspaceFileEntry } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { cn, formatRelativeTime, truncate } from "@/lib/utils";
import { ConfirmDialog, PromptDialog } from "@/shared/ui/confirm-dialog";

interface RoomContextSectionProps {
  can_manage_conversations?: boolean;
  contextualFiles: WorkspaceFileEntry[];
  conversations: Conversation[];
  current_conversation_id: string | null;
  file_explorer_content: React.ReactNode;
  filesystem_error: string | null;
  is_file_explorer_visible: boolean;
  memory_file_count: number;
  on_create_conversation: (title?: string) => Promise<string | null>;
  on_create_directory: () => void;
  on_create_file: () => void;
  on_delete_conversation: (conversation_id: string) => Promise<string | null>;
  on_open_workspace_file: (path: string) => void;
  on_select_conversation: (conversation_id: string) => void;
  on_toggle_file_explorer: () => void;
  active_workspace_path: string | null;
}

export function RoomContextSection({
                                     can_manage_conversations = true,
                                     contextualFiles,
                                     conversations,
                                     current_conversation_id,
                                     file_explorer_content,
                                     filesystem_error,
                                     is_file_explorer_visible,
                                     memory_file_count,
                                     on_create_conversation,
                                     on_create_directory,
                                     on_create_file,
                                     on_delete_conversation,
                                     on_open_workspace_file,
                                     on_select_conversation,
                                     on_toggle_file_explorer,
                                     active_workspace_path,
                                   }: RoomContextSectionProps) {
  const [is_create_dialog_open, set_is_create_dialog_open] = useState(false);
  const [pending_delete_conversation_id, set_pending_delete_conversation_id] = useState<string | null>(null);
  const pending_delete_conversation = conversations.find(
    (conversation) => conversation.session_key === pending_delete_conversation_id,
  ) ?? null;

  return (
    <>
      <section className="border-t workspace-divider px-2 py-5">
        <div className="mb-3 flex items-center justify-between">
          <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/56">
            <FolderTree className="h-3.5 w-3.5"/>
            上下文
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
              <FilePlus2 className="h-3.5 w-3.5"/>
            </button>
            <button
              aria-label="创建目录"
              className="workspace-chip rounded-xl p-1.5 text-slate-700/58 transition-colors hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
              onClick={on_create_directory}
              type="button"
            >
              <FolderPlus className="h-3.5 w-3.5"/>
            </button>
          </div>
        </div>


      <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-700/54">
        <BrainCircuit className="h-3.5 w-3.5"/>
        会话
      </div>

        <div className="mt-3 space-y-2">
          {conversations.map((conversation) => {
            const isActive = conversation.session_key === current_conversation_id;
            const can_delete = conversation.conversation_type === "topic";
            return (
              <div
                key={conversation.session_key}
                className={cn(
                  "group cursor-pointer radius-shell-md px-4 py-3 text-left transition-all duration-300",
                  isActive
                    ? "workspace-card-strong shadow-[0_16px_28px_rgba(111,126,162,0.14)]"
                    : "workspace-card hover:-translate-y-0.5",
                )}
                onClick={() => on_select_conversation(conversation.session_key)}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    on_select_conversation(conversation.session_key);
                  }
                }}
                role="button"
                tabIndex={0}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="text-[13px] font-semibold text-slate-900/88">
                      {truncate(conversation.title || "未命名对话", 22)}
                    </p>
                    <p className="mt-1 text-[11px] text-slate-700/52">
                      {formatRelativeTime(conversation.last_activity_at)} · {conversation.message_count ?? 0} 条
                    </p>
                  </div>

                  {can_manage_conversations && can_delete ? (
                    <button
                      aria-label="删除对话"
                      className="workspace-chip rounded-xl p-1.5 text-slate-700/54 opacity-0 transition-all group-hover:opacity-100 hover:text-destructive focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
                      onClick={(event) => {
                        event.stopPropagation();
                        set_pending_delete_conversation_id(conversation.session_key);
                      }}
                      type="button"
                    >
                      <Trash2 className="h-3.5 w-3.5"/>
                    </button>
                  ) : null}
                </div>
              </div>
            );
          })}
        </div>

        {can_manage_conversations && (
          <button
            className="mt-2 inline-flex items-center gap-1.5 rounded-full bg-[linear-gradient(135deg,rgba(166,255,194,0.92),rgba(102,217,143,0.88))] px-3 py-1.5 text-[11px] font-bold text-[#18653a] shadow-[0_14px_24px_rgba(102,217,143,0.24)]"
            onClick={() => set_is_create_dialog_open(true)}
            type="button"
          >
            <MessageSquarePlus className="h-3.5 w-3.5"/>
            新建会话
          </button>
        )}

        {filesystem_error && (
          <div
            className="radius-shell-sm mt-3 border border-destructive/20 bg-destructive/6 px-3 py-2 text-xs text-destructive">
            {filesystem_error}
          </div>
        )}

        {contextualFiles.length > 0 && (
          <div className="mt-3">
          <p className="mb-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-700/50">
            记忆
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
                  <div
                    className="workspace-chip flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-slate-700/60">
                    <FileText className="h-3.5 w-3.5"/>
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

      <PromptDialog
        is_open={is_create_dialog_open}
        message="输入新对话标题，不填则按 room 自动命名。"
        on_cancel={() => set_is_create_dialog_open(false)}
        on_confirm={(title) => {
          void on_create_conversation(title.trim() || undefined);
          set_is_create_dialog_open(false);
        }}
        placeholder="例如：竞品分析"
        title="新建会话"
      />

      <ConfirmDialog
        cancel_text="取消"
        confirm_text="删除"
        is_open={Boolean(pending_delete_conversation)}
        message={`确定要删除对话「${pending_delete_conversation?.title ?? ""}」吗？`}
        on_cancel={() => set_pending_delete_conversation_id(null)}
        on_confirm={() => {
          if (pending_delete_conversation_id) {
            void on_delete_conversation(pending_delete_conversation_id);
          }
          set_pending_delete_conversation_id(null);
        }}
        title="删除对话"
        variant="danger"
      />
    </>
  );
}
