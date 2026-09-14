/**
 * INPUT: 已投影的历史条目、标题编辑器与选择/切换/删除回调。
 * OUTPUT: 具公共次动作显隐和可读时间的历史条目；编辑键盘隔离输入法与菜单关闭。
 * POS: Room 历史单项纯视图，不判断会话协议与删除资格。
 */

import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import {
  type ComponentType,
  type KeyboardEvent,
  type MouseEvent,
  type RefObject,
  useId,
} from "react";
import { Check, Pencil, Trash2, X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
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
  triggerRef: RefObject<HTMLButtonElement | null>;
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
      getUiTypographyClassName({ role: "metadata", tone: "muted" }),
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
}: RoomHistoryItemViewProps) {
  const checkboxId = useId();
  const selection = presentation.selection;
  if (!selection) {
    return null;
  }
  return (
    <UiTooltip label={selection.disabled ? selectionLabel : undefined}><label
      className={cn(
        "flex w-full items-center gap-2.5",
        selection.disabled ? "cursor-default" : "cursor-pointer",
      )}
      htmlFor={checkboxId}

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
    </label></UiTooltip>
  );
}

function handleTitleEditorKeyDown(
  event: KeyboardEvent<HTMLInputElement>,
  editor: TitleEditorView,
) {
  if (event.key !== "Enter" && event.key !== "Escape") return;
  // Composition keys belong to the input, including Escape candidate dismissal.
  event.stopPropagation();
  if (isImeKeyboardEvent(event.nativeEvent)) return;
  event.preventDefault();
  if (event.key === "Enter") editor.confirm();
  else editor.cancel();
}

function EditingItemContent({
  editor,
  presentation,
}: RoomHistoryItemViewProps) {
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
  ComponentType<RoomHistoryItemViewProps>
> = {
  editing: EditingItemContent,
  selecting: SelectingItemContent,
};

function RoomHistoryItemActions({
  editor,
  onDelete,
  presentation,
}: RoomHistoryItemViewProps) {
  if (presentation.actions.length === 0) {
    return null;
  }
  const actionHandlers: Record<RoomHistoryItemAction, (event: MouseEvent) => void> = {
    delete: onDelete,
    rename: editor.start,
  };
  return (
    <div className="flex shrink-0 items-center gap-1">
      {presentation.actions.map((action) => {
        const style = ACTION_STYLES[action];
        const Icon = style.icon;
        return (
          <UiListActionButton
            aria-label={presentation.actionLabels[action]}
            key={action}
            onClick={actionHandlers[action]}
            ref={action === "rename" ? editor.triggerRef : undefined}
            size="xs"
            stopPropagation
            tone={style.tone}
            visibility={presentation.actionsPersistent ? "visible" : "hover"}
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
