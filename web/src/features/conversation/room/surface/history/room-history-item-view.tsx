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
import { Check, Pencil, Trash2, X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiCheckbox } from "@/shared/ui/form/checkbox";
import { UiInput } from "@/shared/ui/form/form-control";
import {
  UiListActionButton,
} from "@/shared/ui/list/list-action";
import type { UiListActionTone } from "@/shared/ui/list/list-action";
import { UiListRow, UiListRowContent } from "@/shared/ui/list/list-row";
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
  className,
  label,
}: {
  className?: string;
  label: string;
}) {
  return (
    <span className={cn(
      "shrink-0 tabular-nums",
      getUiTypographyClassName({ role: "caption", tone: "soft" }),
      className,
    )}>
      {label}
    </span>
  );
}

function RoomHistoryContent({
  presentation,
}: {
  presentation: RoomHistoryItemPresentation;
}) {
  return (
    <UiListRowContent
      description={presentation.externalSessionLabel
        ? presentation.externalSessionLabel
        : undefined}
      meta={<RoomHistoryActivity label={presentation.activityLabel} />}
      title={presentation.title}
    />
  );
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
      <RoomHistoryContent presentation={presentation} />
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
      <RoomHistoryActivity className="mt-1 block" label={presentation.activityLabel} />
    </>
  );
}

const CONTENT_VIEWS: Record<
  Exclude<RoomHistoryItemMode, "reading">,
  ComponentType<ItemContentProps>
> = {
  editing: EditingItemContent,
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
    <div className={cn(
      "flex shrink-0 items-center gap-1 transition-opacity duration-(--motion-duration-fast)",
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
  );
}

export function RoomHistoryItemView(props: RoomHistoryItemViewProps) {
  const { presentation } = props;
  const Content = presentation.mode === "reading"
    ? null
    : CONTENT_VIEWS[presentation.mode];
  return (
    <UiListRow
      actions={<RoomHistoryItemActions {...props} />}
      active={presentation.state === "active" || Boolean(presentation.selection?.checked)}
      activeTone="sidebar"
      aria-current={presentation.state === "active" ? "page" : undefined}
      className="items-stretch"
      description={Content || !presentation.externalSessionLabel
        ? undefined
        : presentation.externalSessionLabel}
      density="dense"
      meta={Content ? undefined : (
        <RoomHistoryActivity label={presentation.activityLabel} />
      )}
      onClick={presentation.mode === "reading" ? props.onSelect : undefined}
      title={Content ? undefined : presentation.title}
    >
      {Content ? (
        <div className="min-w-0 flex-1">
          <Content {...props} />
        </div>
      ) : undefined}
    </UiListRow>
  );
}
