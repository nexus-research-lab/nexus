/**
 * INPUT: Room 历史条目、切换/重命名/删除命令与批量选择状态。
 * OUTPUT: 绑定标题编辑器和本地化文案的单条历史会话视图。
 * POS: Room 历史单项控制器，视图不直接读取原始会话协议。
 */

import { useEffect } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";

import { buildRoomHistoryItemPresentation } from "./room-history-item-model";
import { RoomHistoryItemView } from "./room-history-item-view";
import type { RoomHistoryEntry } from "./room-history-model";
import { useConversationTitleEditor } from "./use-conversation-title-editor";

interface RoomHistoryItemProps {
  entry: RoomHistoryEntry;
  isSelected?: boolean;
  isSelecting?: boolean;
  onDelete: () => void;
  onRename: (title: string) => void;
  onSelect: () => void;
  onToggleSelection?: () => void;
}

export function RoomHistoryItem({
  entry,
  isSelected = false,
  isSelecting = false,
  onDelete,
  onRename,
  onSelect,
  onToggleSelection,
}: RoomHistoryItemProps) {
  const { locale, t } = useI18n();
  const editor = useConversationTitleEditor({
    onRename,
    title: entry.conversation.title ?? "",
  });
  const { cancel: cancelEditor, isEditing } = editor;

  useEffect(() => {
    if (isSelecting && isEditing) {
      cancelEditor();
    }
  }, [cancelEditor, isEditing, isSelecting]);

  const presentation = buildRoomHistoryItemPresentation(
    entry,
    {
      isEditing,
      isSelected,
      isSelecting,
    },
    {
      actionLabels: {
        delete: t("room.history_delete"),
        rename: t("room.history_rename"),
      },
      editorLabels: {
        cancel: t("common.cancel"),
        confirm: t("room.history_confirm_rename"),
        input: t("room.history_edit_title"),
      },
      locale,
      untitled: t("room.new_conversation"),
    },
  );
  return (
    <RoomHistoryItemView
      editor={editor}
      onDelete={onDelete}
      onSelect={onSelect}
      onToggleSelection={() => onToggleSelection?.()}
      presentation={presentation}
      selectionLabel={presentation.selection?.disabled
        ? t("room.history_batch_unavailable")
        : t("room.history_select_conversation", {
            title: presentation.title,
          })}
    />
  );
}
