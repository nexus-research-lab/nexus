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
  useId,
} from "react";
import { Check, Clock3, Pencil, Trash2, X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiCheckbox } from "@/shared/ui/form/checkbox";
import { UiInput } from "@/shared/ui/form/form-control";
import {
  UiListActionButton,
} from "@/shared/ui/list/list-action";
import type { UiListActionTone } from "@/shared/ui/list/list-action-styles";
import { UiListRow } from "@/shared/ui/list/list-row";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import type {
  RoomHistoryItemAction,
  RoomHistoryItemMode,
  RoomHistoryItemPresentation,
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
  icon: ComponentType<{ className?: string }>;
  tone: UiListActionTone;
}

const ACTION_STYLES: Record<RoomHistoryItemAction, ActionStyle> = {
  delete: {
    icon: Trash2,
    tone: "danger",
  },
  rename: {
    icon: Pencil,
    tone: "default",
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
      "flex items-center gap-1.5 transition-opacity duration-(--motion-duration-fast)",
      getUiTypographyClassName({ role: "caption", tone: "soft" }),
      compact ? "shrink-0" : "mt-1 flex-wrap gap-y-0.5",
      hideForActions && (
        persistActions
          ? "opacity-0"
          : "group-hover/item:opacity-0 group-focus-within/item:opacity-0"
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
    <UiBadge className="whitespace-nowrap" size="xs" tone="primary">
      IM · {label}
    </UiBadge>
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
          "whitespace-nowrap",
          getUiTypographyClassName({
            role: "control",
            tone: presentation.state === "active" ? "strong" : "default",
            weight: presentation.state === "active" ? "semibold" : "medium",
          }),
          presentation.state !== "active" && "group-hover/item:text-(--text-strong)",
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
  presentation,
}: ItemContentProps) {
  return <RoomHistorySummary presentation={presentation} />;
}

function SelectingItemContent({
  onToggleSelection,
  presentation,
  selectionLabel,
}: ItemContentProps) {
  const checkboxId = useId();
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
      htmlFor={checkboxId}
      title={selection.disabled ? selectionLabel : undefined}
    >
      <UiCheckbox
        aria-label={selectionLabel}
        checked={selection.checked}
        checkboxSize="small"
        disabled={selection.disabled}
        id={checkboxId}
        onChange={onToggleSelection}
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
        <UiInput
          aria-label={presentation.editorLabels.input}
          className={cn(
            "min-w-0 flex-1",
            getUiTypographyClassName({ role: "control", weight: "semibold" }),
          )}
          controlSize="xs"
          maxLength={64}
          onChange={(event) => editor.setDraft(event.target.value)}
          onKeyDown={(event) => handleTitleEditorKeyDown(event, editor)}
          ref={editor.inputRef}
          value={editor.draft}
          variant="surface"
        />
        <UiListActionButton
          aria-label={presentation.editorLabels.confirm}
          onClick={editor.confirm}
          size="xs"
          tone="primary"
          visibility="visible"
        >
          <Check className="h-3.5 w-3.5" />
        </UiListActionButton>
        <UiListActionButton
          aria-label={presentation.editorLabels.cancel}
          onClick={editor.cancel}
          size="xs"
          visibility="visible"
        >
          <X className="h-3.5 w-3.5" />
        </UiListActionButton>
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
    delete: onDelete,
    rename: editor.start,
  };
  return (
    <div className="sticky right-0 z-10 -my-1.5 ml-2 grid shrink-0 place-items-center self-stretch bg-inherit px-2.5">
      <div className={cn(
        "flex items-center gap-1 transition-opacity duration-(--motion-duration-fast)",
        presentation.actionsPersistent
          ? "opacity-100"
          : "opacity-0 group-hover/item:opacity-100 focus-within:opacity-100",
      )}>
        {presentation.actions.map((action) => {
          const style = ACTION_STYLES[action];
          const Icon = style.icon;
          return (
            <UiListActionButton
              aria-label={presentation.actionLabels[action]}
              key={action}
              onClick={actionHandlers[action]}
              size="xs"
              stopPropagation
              tone={style.tone}
              visibility="visible"
            >
              <Icon className="h-3 w-3" />
            </UiListActionButton>
          );
        })}
      </div>
    </div>
  );
}

export function RoomHistoryItemView(props: RoomHistoryItemViewProps) {
  const { presentation } = props;
  const Content = CONTENT_VIEWS[presentation.mode];
  return (
    <UiListRow
      actions={<RoomHistoryItemActions {...props} />}
      active={presentation.state === "active" || Boolean(presentation.selection?.checked)}
      activeTone="sidebar"
      aria-current={presentation.state === "active" ? "page" : undefined}
      className="min-w-full w-max items-stretch pr-0"
      density="dense"
      onClick={presentation.mode === "reading" ? props.onSelect : undefined}
    >
      <div className="min-w-max flex-1">
        <div className="min-w-max">
          <Content {...props} />
        </div>
      </div>
    </UiListRow>
  );
}
