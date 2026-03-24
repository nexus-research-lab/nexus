import { BrainCircuit, MessageSquarePlus, Trash2 } from "lucide-react";

import { Conversation } from "@/types/conversation";
import { cn, formatRelativeTime, truncate } from "@/lib/utils";

interface RoomConversationsSectionProps {
  conversations: Conversation[];
  currentConversationId: string | null;
  onCreateConversation: () => void;
  onDeleteConversation: (conversationId: string) => void;
  onSelectConversation: (conversationId: string) => void;
}

export function RoomConversationsSection({
  conversations,
  currentConversationId,
  onCreateConversation,
  onDeleteConversation,
  onSelectConversation,
}: RoomConversationsSectionProps) {
  return (
    <section className="px-5 py-5">
      <div className="mb-3 flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/56">
          <BrainCircuit className="h-3.5 w-3.5" />
          Conversations
        </div>
        <button
          className="inline-flex items-center gap-1.5 rounded-full bg-[linear-gradient(135deg,rgba(166,255,194,0.92),rgba(102,217,143,0.88))] px-3 py-1.5 text-[11px] font-bold text-[#18653a] shadow-[0_14px_24px_rgba(102,217,143,0.24)]"
          onClick={onCreateConversation}
          type="button"
        >
          <MessageSquarePlus className="h-3.5 w-3.5" />
          新建对话
        </button>
      </div>

      <div className="space-y-2">
        {conversations.map((conversation) => {
          const isActive = conversation.session_key === currentConversationId;
          return (
            <div
              key={conversation.session_key}
              className={cn(
                "group cursor-pointer radius-shell-md px-4 py-3 text-left transition-all duration-300",
                isActive
                  ? "workspace-card-strong shadow-[0_16px_28px_rgba(111,126,162,0.14)]"
                  : "workspace-card hover:-translate-y-0.5",
              )}
              onClick={() => onSelectConversation(conversation.session_key)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  onSelectConversation(conversation.session_key);
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

                <button
                  aria-label="删除对话"
                  className="workspace-chip rounded-xl p-1.5 text-slate-700/54 opacity-0 transition-all group-hover:opacity-100 hover:text-destructive focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
                  onClick={(event) => {
                    event.stopPropagation();
                    onDeleteConversation(conversation.session_key);
                  }}
                  type="button"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}
