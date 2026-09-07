// INPUT: Pending messages, drag identities and reorder targets.
// OUTPUT: Text-first content, guidance/drag identities and pure ordering; missing/equal targets preserve order.
// POS: Composer queue model; padding belongs to composer-styles and DOM drag state to its controller.

import type { InputQueueItem } from "@/types/agent/agent-conversation";

export interface PendingQueueDragState {
  dragOverMessageId: string | null;
  draggingMessageId: string | null;
}

export interface PendingQueueItemProjection {
  content: PendingQueueItemContent | null;
  isDragTarget: boolean;
  isDragging: boolean;
  isGuidanceWaiting: boolean;
}

export interface PendingQueueItemContent {
  kind: "attachments" | "text";
  text: string;
}

export function projectPendingQueueItem(
  item: InputQueueItem,
  dragState: PendingQueueDragState,
): PendingQueueItemProjection {
  return {
    content: projectPendingQueueContent(item),
    isDragging: dragState.draggingMessageId === item.id,
    isDragTarget: Boolean(dragState.draggingMessageId)
      && dragState.draggingMessageId !== item.id
      && dragState.dragOverMessageId === item.id,
    isGuidanceWaiting: item.delivery_policy === "guide",
  };
}

function projectPendingQueueContent(
  item: InputQueueItem,
): PendingQueueItemContent | null {
  const text = item.content.trim();
  if (text) return { kind: "text", text };
  const attachmentNames = (item.attachments ?? [])
    .map((attachment) => attachment.file_name || attachment.workspace_path)
    .filter(Boolean)
    .join("、");
  return attachmentNames ? { kind: "attachments", text: attachmentNames } : null;
}

export function reorderPendingMessageIds(
  items: InputQueueItem[],
  sourceId: string,
  targetId: string,
): string[] {
  const sourceIndex = items.findIndex((item) => item.id === sourceId);
  const targetIndex = items.findIndex((item) => item.id === targetId);
  if (sourceIndex < 0 || targetIndex < 0 || sourceIndex === targetIndex) {
    return items.map((item) => item.id);
  }
  const reorderedItems = [...items];
  const [source] = reorderedItems.splice(sourceIndex, 1);
  reorderedItems.splice(targetIndex, 0, source);
  return reorderedItems.map((item) => item.id);
}
