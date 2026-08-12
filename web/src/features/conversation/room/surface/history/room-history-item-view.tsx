/**
 * INPUT: 已投影的历史条目、标题编辑器与选择/切换/删除回调。
 * OUTPUT: 阅读、编辑和批量选择三种互斥模式的可访问条目视图。
 * POS: Room 历史单项纯视图，不判断会话协议与删除资格。
 */

import {
  type ComponentType,
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
  onToggleSelection: () => void;
  presentation: RoomHistoryItemPresentation;
  selectionLabel: string;
}

interface ItemContentProps extends RoomHistoryItemViewProps {}

interface ActionStyle {
  className: string;
  icon: ComponentType<{ className?: string }>;
}

const ENTRY_STYLES: Record<RoomHistoryItemState, string> = {
  active: "bg-(--surface-sidebar-active-background) text-(--text-strong)",
  idle: "bg-transparent text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
};

const ACTION_STYLES: Record<RoomHistoryItemAction, ActionStyle> = {
  delete: {
    className: "text-(--destructive) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)]",
    icon: Trash2,
  },
  rename: {
    className: "text-(--icon-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)",
    icon: Pencil,
  },
};

function RoomHistoryActivity({
  compact = false,
  hideForActions = false,
  persistActions = false,
  label,
}: {
  compact?: boolean;
  hideForActions?: boolean;
  persistActions?: boolean;
  label: string;
}) {
  return (
    <div className={cn(
      "flex items-center gap-1.5 text-(--text-soft) transition-opacity duration-(--motion-duration-fast)",
      compact ? "shrink-0 text-2xs" : "mt-1 flex-wrap gap-y-0.5 text-2xs",
      hideForActions && (
        persistActions
          ? "opacity-0"
          : "group-hover:opacity-0 group-focus-within:opacity-0"
      ),
    )}>
      <span className={cn(
        "inline-flex items-center gap-1.5",
        compact && "w-full justify-end",
      )}>
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
    <span className="inline-flex shrink-0 items-center whitespace-nowrap rounded-[6px] border border-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] px-1.5 py-0.5 text-[9px] font-medium text-(--primary)">
      IM · {label}
    </span>
  );
}

function RoomHistorySummary({
  presentation,
}: {
  presentation: RoomHistoryItemPresentation;
}) {
  return (
    <div className="grid min-w-full w-max grid-cols-[max-content_78px] items-center gap-3">
      <div className="flex min-w-max items-center gap-2">
        <p className={cn(
          "whitespace-nowrap text-compact",
          presentation.state === "active"
            ? "font-semibold text-(--text-strong)"
            : "font-medium text-(--text-default) group-hover:text-(--text-strong)",
        )}>
          {presentation.title}
        </p>
        <ExternalSessionLabel label={presentation.externalSessionLabel} />
      </div>
      <RoomHistoryActivity
        compact
        hideForActions={presentation.actions.length > 0}
        label={presentation.activityLabel}
        persistActions={presentation.actionsPersistent}
      />
    </div>
  );
}

function ReadingItemContent({
  onSelect,
  presentation,
}: ItemContentProps) {
  return (
    <button
      aria-current={presentation.state === "active" ? "page" : undefined}
      className="block w-full rounded-[10px] text-left outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]"
      onClick={onSelect}
      type="button"
    >
      <RoomHistorySummary presentation={presentation} />
    </button>
  );
}

function SelectingItemContent({
  onToggleSelection,
  presentation,
  selectionLabel,
}: ItemContentProps) {
  const selection = presentation.selection;
  if (!selection) {
    return null;
  }
  return (
    <label
      className={cn(
        "flex w-full items-center gap-2.5",
        selection.disabled ? "cursor-default" : "cursor-pointer",
      )}
      title={selection.disabled ? selectionLabel : undefined}
    >
      <input
        aria-label={selectionLabel}
        checked={selection.checked}
        className="h-3.5 w-3.5 shrink-0 accent-[var(--primary)] disabled:opacity-35"
        disabled={selection.disabled}
        onChange={onToggleSelection}
        type="checkbox"
      />
      <div className="min-w-0 flex-1">
        <RoomHistorySummary presentation={presentation} />
      </div>
    </label>
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
          aria-label={presentation.editorLabels.input}
          className="min-w-0 flex-1 rounded-[10px] border border-(--input-shell-border) bg-transparent px-2.5 py-1.5 text-sm font-semibold text-(--text-strong) outline-none transition focus:border-(--surface-interactive-active-border)"
          maxLength={64}
          onChange={(event) => editor.setDraft(event.target.value)}
          onKeyDown={(event) => handleTitleEditorKeyDown(event, editor)}
          ref={editor.inputRef}
          value={editor.draft}
        />
        <button
          aria-label={presentation.editorLabels.confirm}
          className="inline-flex h-6 w-6 items-center justify-center rounded-[8px] text-(--primary) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background)"
          onClick={editor.confirm}
          type="button"
        >
          <Check className="h-3.5 w-3.5" />
        </button>
        <button
          aria-label={presentation.editorLabels.cancel}
          className="inline-flex h-6 w-6 items-center justify-center rounded-[8px] text-(--icon-default) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
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
  selecting: SelectingItemContent,
};

function RoomHistoryItemActions({
  editor,
  onDelete,
  presentation,
}: ItemContentProps) {
  if (presentation.actions.length === 0) {
    return null;
  }
  const actionHandlers: Record<RoomHistoryItemAction, (event: MouseEvent) => void> = {
    delete: (event) => {
      event.stopPropagation();
      onDelete();
    },
    rename: editor.start,
  };
  return (
    <div className="sticky right-0 z-10 -my-1.5 ml-2 grid shrink-0 place-items-center self-stretch rounded-r-[10px] bg-inherit px-2.5">
      <div className={cn(
        "flex items-center gap-1 transition-opacity duration-(--motion-duration-fast)",
        presentation.actionsPersistent
          ? "opacity-100"
          : "opacity-0 group-hover:opacity-100 focus-within:opacity-100",
      )}>
        {presentation.actions.map((action) => {
          const style = ACTION_STYLES[action];
          const Icon = style.icon;
          return (
            <button
              aria-label={presentation.actionLabels[action]}
              className={cn(
                "inline-flex h-6 w-6 items-center justify-center rounded-[8px] focus-visible:opacity-100",
                style.className,
              )}
              key={action}
              onClick={actionHandlers[action]}
              type="button"
            >
              <Icon className="h-3 w-3" />
            </button>
          );
        })}
      </div>
    </div>
  );
}

export function RoomHistoryItemView(props: RoomHistoryItemViewProps) {
  const { presentation } = props;
  const stateClassName = ENTRY_STYLES[presentation.state];
  const Content = CONTENT_VIEWS[presentation.mode];
  return (
    <article
      className={cn(
        "group relative flex min-w-full w-max items-stretch rounded-[10px] py-1.5 pl-2.5 text-left transition-[background-color,color] duration-(--motion-duration-fast) ease-out",
        stateClassName,
        presentation.selection?.checked
          && "bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] text-(--text-strong)",
      )}
    >
      <div className="min-w-max flex-1">
        <div className="min-w-max">
          <Content {...props} />
        </div>
      </div>
      <RoomHistoryItemActions {...props} />
    </article>
  );
}
