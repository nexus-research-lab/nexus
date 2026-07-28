/**
 * INPUT: Room 历史条目的批量删除资格与滚动列表条目。
 * OUTPUT: 按当前可删除快照派生收敛的全列表批量选择状态与切换命令。
 * POS: Room 历史菜单的选择状态域，不执行删除请求或渲染控件。
 */

import { useCallback, useMemo, useState } from "react";

import type { RoomHistoryEntry } from "./room-history-model";

export type RoomHistorySelectionState = "all" | "mixed" | "none";

function setsEqual(left: ReadonlySet<string>, right: ReadonlySet<string>): boolean {
  return left.size === right.size && [...left].every((id) => right.has(id));
}

export function getBulkSelectableConversationIds(
  entries: readonly RoomHistoryEntry[],
): Set<string> {
  return new Set(
    entries
      .filter((entry) => entry.canBulkDelete)
      .map((entry) => entry.conversation.conversation_id),
  );
}

export function reconcileRoomHistorySelection(
  selectedIds: Set<string>,
  selectableIds: ReadonlySet<string>,
): Set<string> {
  const reconciled = new Set(
    [...selectedIds].filter((id) => selectableIds.has(id)),
  );
  return setsEqual(selectedIds, reconciled)
    ? selectedIds
    : reconciled;
}

export function getRoomHistorySelectionState(
  selectedIds: ReadonlySet<string>,
  entries: readonly RoomHistoryEntry[],
): RoomHistorySelectionState {
  const selectableIds = [...getBulkSelectableConversationIds(entries)];
  const selectedCount = selectableIds.filter((id) => selectedIds.has(id)).length;
  if (selectedCount === 0) {
    return "none";
  }
  return selectedCount === selectableIds.length ? "all" : "mixed";
}

export function toggleAllRoomHistorySelection(
  selectedIds: ReadonlySet<string>,
  entries: readonly RoomHistoryEntry[],
): Set<string> {
  const selectableIds = [...getBulkSelectableConversationIds(entries)];
  const shouldClearAll = selectableIds.length > 0
    && selectableIds.every((id) => selectedIds.has(id));
  const next = new Set(selectedIds);
  selectableIds.forEach((id) => {
    if (shouldClearAll) {
      next.delete(id);
    } else {
      next.add(id);
    }
  });
  return next;
}

export function useRoomHistorySelection({
  entries,
}: {
  entries: readonly RoomHistoryEntry[];
}) {
  const selectableIds = useMemo(
    () => getBulkSelectableConversationIds(entries),
    [entries],
  );
  const [isSelecting, setIsSelecting] = useState(false);
  const [storedSelectedIds, setStoredSelectedIds] = useState<Set<string>>(
    () => new Set(),
  );
  const selectedIds = useMemo(
    () => reconcileRoomHistorySelection(storedSelectedIds, selectableIds),
    [selectableIds, storedSelectedIds],
  );

  const clearSelection = useCallback(() => {
    setIsSelecting(false);
    setStoredSelectedIds((current) => current.size === 0 ? current : new Set());
  }, []);
  const startSelection = useCallback(() => {
    if (selectableIds.size === 0) {
      return;
    }
    setStoredSelectedIds(new Set());
    setIsSelecting(true);
  }, [selectableIds]);
  const restoreSelection = useCallback((ids: readonly string[]) => {
    const restored = reconcileRoomHistorySelection(new Set(ids), selectableIds);
    setStoredSelectedIds(restored);
    setIsSelecting(restored.size > 0);
  }, [selectableIds]);
  const toggleSelection = useCallback((conversationId: string) => {
    if (!selectableIds.has(conversationId)) {
      return;
    }
    setStoredSelectedIds((current) => {
      const next = new Set(
        reconcileRoomHistorySelection(current, selectableIds),
      );
      if (next.has(conversationId)) {
        next.delete(conversationId);
      } else {
        next.add(conversationId);
      }
      return next;
    });
  }, [selectableIds]);
  const toggleAllSelection = useCallback(() => {
    setStoredSelectedIds((current) => (
      toggleAllRoomHistorySelection(
        reconcileRoomHistorySelection(current, selectableIds),
        entries,
      )
    ));
  }, [entries, selectableIds]);

  return {
    clearSelection,
    hasSelectableEntries: selectableIds.size > 0,
    isSelecting,
    restoreSelection,
    selectedIds,
    startSelection,
    toggleAllSelection,
    toggleSelection,
  };
}
