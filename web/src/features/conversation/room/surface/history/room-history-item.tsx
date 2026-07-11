import { Check, Clock3, Pencil, Trash2, X } from "lucide-react";

import { cn, formatRelativeTime } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";

import type { RoomHistoryEntry } from "./room-history-model";
import { useConversationTitleEditor } from "./use-conversation-title-editor";

interface RoomHistoryItemProps {
  entry: RoomHistoryEntry;
  onDelete: () => void;
  onRename: (title: string) => void;
  onSelect: () => void;
}

export function RoomHistoryItem({
  entry,
  onDelete,
  onRename,
  onSelect,
}: RoomHistoryItemProps) {
  const {t} = useI18n();
  const {conversation} = entry;
  const editor = useConversationTitleEditor({
    title: conversation.title ?? "",
    onRename,
  });
  const showActions = !editor.isEditing && (entry.canRename || entry.canDelete);
  const activity = (
    <div className="mt-1 flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[10.5px] text-(--text-soft)">
      <span className="inline-flex items-center gap-1.5">
        <Clock3 className="h-3 w-3 shrink-0" />
        <span>{formatRelativeTime(conversation.last_activity_at)}</span>
      </span>
    </div>
  );

  return (
    <article
      className={cn(
        "group relative w-full overflow-hidden rounded-[14px] border px-3 py-2.5 text-left transition-[background-color,border-color,box-shadow] duration-(--motion-duration-fast) ease-out",
        entry.isActive
          ? "border-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]"
          : "border-transparent bg-transparent hover:border-[color:color-mix(in_srgb,var(--divider-subtle-color)_64%,transparent)] hover:bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_72%,transparent)]",
      )}
      style={entry.isActive
        ? {
          background: "color-mix(in srgb, var(--surface-interactive-active-background) 46%, transparent)",
          boxShadow: "inset 0 1px 0 rgba(255,255,255,0.56)",
        }
        : undefined}
    >
      {entry.isActive ? (
        <span
          aria-hidden="true"
          className="absolute left-0 top-2.5 bottom-2.5 w-px rounded-full bg-(--primary)"
        />
      ) : null}

      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0 flex-1">
          {editor.isEditing ? (
            <div className="flex items-center gap-1.5">
              <input
                aria-label="编辑对话标题"
                className="min-w-0 flex-1 rounded-[10px] border border-(--input-shell-border) bg-transparent px-2.5 py-1.5 text-[13px] font-semibold text-(--text-strong) outline-none transition focus:border-(--surface-interactive-active-border)"
                maxLength={64}
                onChange={(event) => editor.setDraft(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") editor.confirm();
                  if (event.key === "Escape") editor.cancel();
                }}
                ref={editor.inputRef}
                value={editor.draft}
              />
              <button
                aria-label="确认"
                className="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-(--primary) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background)"
                onClick={editor.confirm}
                type="button"
              >
                <Check className="h-3.5 w-3.5" />
              </button>
              <button
                aria-label="取消"
                className="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-(--icon-default) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
                onClick={editor.cancel}
                type="button"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          ) : (
            <button
              className="block w-full rounded-[10px] text-left outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_32%,transparent)]"
              onClick={onSelect}
              type="button"
            >
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex min-w-0 items-center gap-2">
                    <p className="min-w-0 truncate text-[13px] font-semibold text-(--text-strong)">
                      {conversation.title?.trim() || t("room.untitled_conversation")}
                    </p>
                    {entry.externalSessionLabel ? (
                      <span className="inline-flex shrink-0 items-center rounded-[6px] border border-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] px-1.5 py-0.5 text-[9.5px] font-medium text-(--primary)">
                        IM · {entry.externalSessionLabel}
                      </span>
                    ) : null}
                  </div>
                  {activity}
                </div>
                <span
                  aria-hidden={!entry.isActive}
                  className={cn(
                    "inline-flex shrink-0 items-center rounded-[6px] border px-1.5 py-0.5 text-[9.5px] font-medium transition-[border-color,color] duration-(--motion-duration-fast)",
                    entry.isActive
                      ? "border-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] text-(--primary)"
                      : "invisible border-transparent text-transparent",
                  )}
                >
                  {t("room.current_conversation")}
                </span>
              </div>
            </button>
          )}
          {editor.isEditing ? activity : null}
        </div>

        {showActions ? (
          <div className="flex shrink-0 items-center gap-1">
            {entry.canRename ? (
              <button
                aria-label="重命名"
                className="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-(--icon-default) opacity-0 transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong) focus-visible:opacity-100 group-hover:opacity-100"
                onClick={editor.start}
                type="button"
              >
                <Pencil className="h-3.5 w-3.5" />
              </button>
            ) : null}
            {entry.canDelete ? (
              <button
                aria-label="删除对话"
                className="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-(--destructive) opacity-0 transition duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] focus-visible:opacity-100 group-hover:opacity-100"
                onClick={(event) => {
                  event.stopPropagation();
                  onDelete();
                }}
                type="button"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </button>
            ) : null}
          </div>
        ) : null}
      </div>
    </article>
  );
}
