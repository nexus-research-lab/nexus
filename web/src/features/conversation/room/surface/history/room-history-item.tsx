import { useI18n } from "@/shared/i18n/i18n-context";

import { buildRoomHistoryItemPresentation } from "./room-history-item-model";
import { RoomHistoryItemView } from "./room-history-item-view";
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
  const { t } = useI18n();
  const editor = useConversationTitleEditor({
    onRename,
    title: entry.conversation.title ?? "",
  });
  const presentation = buildRoomHistoryItemPresentation(
    entry,
    editor.isEditing,
    {
      current: t("room.current_conversation"),
      untitled: t("room.untitled_conversation"),
    },
  );
  return (
    <RoomHistoryItemView
      editor={editor}
      onDelete={onDelete}
      onSelect={onSelect}
      presentation={presentation}
    />
  );
}
