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

interface PendingQueueContentCandidate {
  active: boolean;
  content: PendingQueueItemContent;
}

const QUEUE_PADDING_CLASS_NAME = {
  compact: "px-2 pb-0.5 pt-1",
  regular: "px-3 pb-1 pt-1",
} as const;

export function projectPendingQueueItem(
  item: InputQueueItem,
  dragState: PendingQueueDragState,
): PendingQueueItemProjection {
  return {
    content: projectPendingQueueContent(item),
    isDragging: dragState.draggingMessageId === item.id,
    isDragTarget: [
      Boolean(dragState.draggingMessageId),
      dragState.draggingMessageId !== item.id,
      dragState.dragOverMessageId === item.id,
    ].every(Boolean),
    isGuidanceWaiting: item.delivery_policy === "guide",
  };
}

function projectPendingQueueContent(
  item: InputQueueItem,
): PendingQueueItemContent | null {
  const text = item.content.trim();
  const attachmentNames = (item.attachments ?? [])
    .map((attachment) => attachment.file_name || attachment.workspace_path)
    .filter(Boolean)
    .join("、");
  const candidates: PendingQueueContentCandidate[] = [
    { active: Boolean(text), content: { kind: "text", text } },
    {
      active: Boolean(attachmentNames),
      content: { kind: "attachments", text: attachmentNames },
    },
  ];
  return candidates.find((candidate) => candidate.active)?.content ?? null;
}

export function reorderPendingMessageIds(
  items: InputQueueItem[],
  sourceId: string,
  targetId: string,
): string[] {
  const sourceIndex = items.findIndex((item) => item.id === sourceId);
  const targetIndex = items.findIndex((item) => item.id === targetId);
  const canReorder = [
    sourceIndex >= 0,
    targetIndex >= 0,
    sourceIndex !== targetIndex,
  ].every(Boolean);
  if (!canReorder) {
    return items.map((item) => item.id);
  }
  const reorderedItems = [...items];
  const [source] = reorderedItems.splice(sourceIndex, 1);
  reorderedItems.splice(targetIndex, 0, source);
  return reorderedItems.map((item) => item.id);
}

export function getPendingQueuePaddingClassName(compact: boolean): string {
  return QUEUE_PADDING_CLASS_NAME[compact ? "compact" : "regular"];
}
