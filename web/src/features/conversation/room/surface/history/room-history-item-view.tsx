import {
  type ComponentType,
  type CSSProperties,
  type KeyboardEvent,
  type MouseEvent,
  type RefObject,
} from "react";
import { Check, Clock3, Pencil, Trash2, X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import type {
  RoomHistoryItemAction,
  RoomHistoryItemMode,
  RoomHistoryItemPresentation,
  RoomHistoryItemState,
} from "./room-history-item-model";

interface TitleEditorView {
  cancel: () => void;
  confirm: () => void;
  draft: string;
  inputRef: RefObject<HTMLInputElement | null>;
  setDraft: (value: string) => void;
  start: (event: MouseEvent) => void;
}

interface RoomHistoryItemViewProps {
  editor: TitleEditorView;
  onDelete: () => void;
  onSelect: () => void;
  presentation: RoomHistoryItemPresentation;
}

interface ItemContentProps extends RoomHistoryItemViewProps {}

interface EntryStyle {
  articleClassName: string;
  currentClassName: string;
  markerClassName: string;
  style?: CSSProperties;
}

interface ActionStyle {
  ariaLabel: string;
  className: string;
  icon: ComponentType<{ className?: string }>;
}

const ENTRY_STYLES: Record<RoomHistoryItemState, EntryStyle> = {
  active: {
    articleClassName: "border-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]",
    currentClassName: "border-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] text-(--primary)",
    markerClassName: "bg-(--primary)",
    style: {
      background: "color-mix(in srgb, var(--surface-interactive-active-background) 46%, transparent)",
      boxShadow: "inset 0 1px 0 rgba(255,255,255,0.56)",
    },
  },
  idle: {
    articleClassName: "border-transparent bg-transparent hover:border-[color:color-mix(in_srgb,var(--divider-subtle-color)_64%,transparent)] hover:bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_72%,transparent)]",
    currentClassName: "invisible border-transparent text-transparent",
    markerClassName: "hidden",
  },
};

const ACTION_STYLES: Record<RoomHistoryItemAction, ActionStyle> = {
  delete: {
    ariaLabel: "删除对话",
    className: "text-(--destructive) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)]",
    icon: Trash2,
  },
  rename: {
    ariaLabel: "重命名",
    className: "text-(--icon-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)",
    icon: Pencil,
  },
};

function RoomHistoryActivity({ label }: { label: string }) {
  return (
    <div className="mt-1 flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[10.5px] text-(--text-soft)">
      <span className="inline-flex items-center gap-1.5">
        <Clock3 className="h-3 w-3 shrink-0" />
        <span>{label}</span>
      </span>
    </div>
  );
}

function ExternalSessionLabel({ label }: { label: string | null }) {
  if (!label) {
    return null;
  }
  return (
    <span className="inline-flex shrink-0 items-center rounded-[6px] border border-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] px-1.5 py-0.5 text-[9.5px] font-medium text-(--primary)">
      IM · {label}
    </span>
  );
}

function ReadingItemContent({
  onSelect,
  presentation,
}: ItemContentProps) {
  const style = ENTRY_STYLES[presentation.state];
  return (
    <button
      className="block w-full rounded-[10px] text-left outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_32%,transparent)]"
      onClick={onSelect}
      type="button"
    >
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-2">
            <p className="min-w-0 truncate text-[13px] font-semibold text-(--text-strong)">
              {presentation.title}
            </p>
            <ExternalSessionLabel label={presentation.externalSessionLabel} />
          </div>
          <RoomHistoryActivity label={presentation.activityLabel} />
        </div>
        <span
          aria-hidden={presentation.state !== "active"}
          className={cn(
            "inline-flex shrink-0 items-center rounded-[6px] border px-1.5 py-0.5 text-[9.5px] font-medium transition-[border-color,color] duration-(--motion-duration-fast)",
            style.currentClassName,
          )}
        >
          {presentation.currentLabel}
        </span>
      </div>
    </button>
  );
}

function handleTitleEditorKeyDown(
  event: KeyboardEvent<HTMLInputElement>,
  editor: TitleEditorView,
) {
  const actions: Partial<Record<string, () => void>> = {
    Enter: editor.confirm,
    Escape: editor.cancel,
  };
  actions[event.key]?.();
}

function EditingItemContent({
  editor,
  presentation,
}: ItemContentProps) {
  return (
    <>
      <div className="flex items-center gap-1.5">
        <input
          aria-label="编辑对话标题"
          className="min-w-0 flex-1 rounded-[10px] border border-(--input-shell-border) bg-transparent px-2.5 py-1.5 text-[13px] font-semibold text-(--text-strong) outline-none transition focus:border-(--surface-interactive-active-border)"
          maxLength={64}
          onChange={(event) => editor.setDraft(event.target.value)}
          onKeyDown={(event) => handleTitleEditorKeyDown(event, editor)}
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
      <RoomHistoryActivity label={presentation.activityLabel} />
    </>
  );
}

const CONTENT_VIEWS: Record<
  RoomHistoryItemMode,
  ComponentType<ItemContentProps>
> = {
  editing: EditingItemContent,
  reading: ReadingItemContent,
};

function RoomHistoryItemActions({
  editor,
  onDelete,
  presentation,
}: ItemContentProps) {
  const actionHandlers: Record<RoomHistoryItemAction, (event: MouseEvent) => void> = {
    delete: (event) => {
      event.stopPropagation();
      onDelete();
    },
    rename: editor.start,
  };
  return (
    <div className="flex shrink-0 items-center gap-1">
      {presentation.actions.map((action) => {
        const style = ACTION_STYLES[action];
        const Icon = style.icon;
        return (
          <button
            aria-label={style.ariaLabel}
            className={cn(
              "inline-flex h-7 w-7 items-center justify-center rounded-[9px] opacity-0 transition duration-(--motion-duration-fast) focus-visible:opacity-100 group-hover:opacity-100",
              style.className,
            )}
            key={action}
            onClick={actionHandlers[action]}
            type="button"
          >
            <Icon className="h-3.5 w-3.5" />
          </button>
        );
      })}
    </div>
  );
}

export function RoomHistoryItemView(props: RoomHistoryItemViewProps) {
  const { presentation } = props;
  const style = ENTRY_STYLES[presentation.state];
  const Content = CONTENT_VIEWS[presentation.mode];
  return (
    <article
      className={cn(
        "group relative w-full overflow-hidden rounded-[14px] border px-3 py-2.5 text-left transition-[background-color,border-color,box-shadow] duration-(--motion-duration-fast) ease-out",
        style.articleClassName,
      )}
      style={style.style}
    >
      <span
        aria-hidden="true"
        className={cn(
          "absolute left-0 top-2.5 bottom-2.5 w-px rounded-full",
          style.markerClassName,
        )}
      />
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0 flex-1">
          <Content {...props} />
        </div>
        <RoomHistoryItemActions {...props} />
      </div>
    </article>
  );
}
