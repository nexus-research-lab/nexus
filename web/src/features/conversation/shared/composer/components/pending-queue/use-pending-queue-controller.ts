// INPUT: 当前队列与 Conversation 所有者提供的派发命令。
// OUTPUT: 原生拖动/相邻移动、有限边缘滚动与同步串行派发保护。
// POS: Queue 瞬时交互控制器；受理、错误展示和队列顺序仍服从 transport/服务端。

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type RefObject,
} from "react";

import type { InputQueueItem } from "@/types/agent/agent-conversation";

import {
  type PendingQueueDragState,
  reorderPendingMessageIds,
} from "./pending-queue-model";

interface PendingQueueCommands {
  deleteMessage: (itemId: string) => void | Promise<void>;
  guideMessage: (itemId: string) => void | Promise<void>;
  reorderMessages: (orderedIds: string[]) => void | Promise<void>;
}

interface PendingQueueDragRuntime {
  draggingMessageIdRef: RefObject<string | null>;
  pointerYRef: RefObject<number | null>;
  scrollFrameRef: RefObject<number | null>;
  scrollRef: RefObject<HTMLOListElement | null>;
}

interface ActiveDragRuntime {
  container: HTMLOListElement;
  pointerY: number;
}

const AUTO_SCROLL_ZONE_PX = 28;
const AUTO_SCROLL_MAX_DELTA_PX = 10;
const EMPTY_DRAG_STATE: PendingQueueDragState = {
  draggingMessageId: null,
  dragOverMessageId: null,
};

export function usePendingQueueController({
  commands,
  items,
}: {
  commands: PendingQueueCommands;
  items: InputQueueItem[];
}) {
  const {
    deleteMessage: runDeleteMessage,
    guideMessage: runGuideMessage,
    reorderMessages: runReorderMessages,
  } = commands;
  const [dragState, setDragState] = useState(EMPTY_DRAG_STATE);
  const [isActionRunning, setIsActionRunning] = useState(false);
  const actionRunningRef = useRef(false);
  const draggingMessageIdRef = useRef<string | null>(null);
  const pointerYRef = useRef<number | null>(null);
  const scrollFrameRef = useRef<number | null>(null);
  const scrollRef = useRef<HTMLOListElement>(null);
  const runtime = useMemo<PendingQueueDragRuntime>(() => ({
    draggingMessageIdRef,
    pointerYRef,
    scrollFrameRef,
    scrollRef,
  }), []);

  const stopAutoScroll = useCallback(() => {
    if (runtime.scrollFrameRef.current !== null) {
      cancelAnimationFrame(runtime.scrollFrameRef.current);
      runtime.scrollFrameRef.current = null;
    }
    runtime.pointerYRef.current = null;
  }, [runtime]);

  const runAutoScroll = useCallback(() => {
    const activeRuntime = readActiveDragRuntime(runtime);
    if (!activeRuntime) {
      runtime.scrollFrameRef.current = null;
      return;
    }
    const delta = resolveAutoScrollDelta(activeRuntime);
    const container = activeRuntime.container;
    const maxScroll = Math.max(0, container.scrollHeight - container.clientHeight);
    const nextScroll = Math.max(0, Math.min(maxScroll, container.scrollTop + delta));
    if (nextScroll === container.scrollTop) {
      runtime.scrollFrameRef.current = null;
      return;
    }
    container.scrollTop = nextScroll;
    runtime.scrollFrameRef.current = requestAnimationFrame(runAutoScroll);
  }, [runtime]);

  const startAutoScroll = useCallback((clientY: number) => {
    if (!runtime.draggingMessageIdRef.current || actionRunningRef.current) return;
    runtime.pointerYRef.current = clientY;
    if (runtime.scrollFrameRef.current === null) {
      runtime.scrollFrameRef.current = requestAnimationFrame(runAutoScroll);
    }
  }, [runAutoScroll, runtime]);

  const finishDrag = useCallback(() => {
    runtime.draggingMessageIdRef.current = null;
    stopAutoScroll();
    setDragState(EMPTY_DRAG_STATE);
  }, [runtime, stopAutoScroll]);

  useEffect(() => stopAutoScroll, [stopAutoScroll]);

  useEffect(() => {
    const source = runtime.draggingMessageIdRef.current;
    if (source && !items.some((item) => item.id === source)) finishDrag();
  }, [finishDrag, items, runtime]);

  const startDrag = useCallback((messageId: string) => {
    if (actionRunningRef.current || !items.some((item) => item.id === messageId)) return;
    stopAutoScroll();
    runtime.draggingMessageIdRef.current = messageId;
    setDragState({ draggingMessageId: messageId, dragOverMessageId: null });
  }, [items, runtime, stopAutoScroll]);

  const dragOver = useCallback((messageId: string, clientY: number) => {
    if (!runtime.draggingMessageIdRef.current) return;
    startAutoScroll(clientY);
    setDragState((current) => (
      current.dragOverMessageId === messageId
        ? current
        : { ...current, dragOverMessageId: messageId }
    ));
  }, [runtime, startAutoScroll]);

  const runCommand = useCallback(async (command: () => void | Promise<void>) => {
    if (actionRunningRef.current) return;
    finishDrag();
    actionRunningRef.current = true;
    setIsActionRunning(true);
    try {
      await command();
    } catch (error) {
      // Conversation 已投影失败；这里只收口 Promise，不创建第二个错误面或重放命令。
      console.error("Queue command dispatch failed:", error);
    } finally {
      actionRunningRef.current = false;
      setIsActionRunning(false);
    }
  }, [finishDrag]);

  const moveToMessage = useCallback((sourceId: string, targetId: string) => {
    const orderedIds = reorderPendingMessageIds(items, sourceId, targetId);
    if (orderedIds.every((id, index) => id === items[index].id)) return;
    return runCommand(() => runReorderMessages(orderedIds));
  }, [items, runCommand, runReorderMessages]);

  const dropOnMessage = useCallback((targetId: string) => {
    const sourceId = runtime.draggingMessageIdRef.current;
    finishDrag();
    if (sourceId) return moveToMessage(sourceId, targetId);
  }, [finishDrag, moveToMessage, runtime]);

  const moveMessage = useCallback((messageId: string, direction: -1 | 1) => {
    const index = items.findIndex((item) => item.id === messageId);
    const target = items[index + direction];
    if (index >= 0 && target) return moveToMessage(messageId, target.id);
  }, [items, moveToMessage]);

  const guideMessage = useCallback((messageId: string) => {
    if (items.some((item) => item.id === messageId)) {
      return runCommand(() => runGuideMessage(messageId));
    }
  }, [items, runCommand, runGuideMessage]);

  const deleteMessage = useCallback((messageId: string) => {
    if (items.some((item) => item.id === messageId)) {
      return runCommand(() => runDeleteMessage(messageId));
    }
  }, [items, runCommand, runDeleteMessage]);

  return {
    actions: {
      deleteMessage,
      dragOver,
      dropOnMessage,
      finishDrag,
      guideMessage,
      startAutoScroll,
      startDrag,
      moveMessage,
    },
    refs: { scrollRef: runtime.scrollRef },
    state: { dragState, isActionRunning },
  };
}

function readActiveDragRuntime(
  runtime: PendingQueueDragRuntime,
): ActiveDragRuntime | null {
  const container = runtime.scrollRef.current;
  const pointerY = runtime.pointerYRef.current;
  if (!container || pointerY === null || !runtime.draggingMessageIdRef.current) return null;
  return { container, pointerY };
}

function resolveAutoScrollDelta({
  container,
  pointerY,
}: ActiveDragRuntime): number {
  const rect = container.getBoundingClientRect();
  const edges = [
    { direction: -1, distance: pointerY - rect.top },
    { direction: 1, distance: rect.bottom - pointerY },
  ];
  const activeEdge = edges.find((edge) => edge.distance < AUTO_SCROLL_ZONE_PX);
  if (!activeEdge) {
    return 0;
  }
  const ratio = (
    AUTO_SCROLL_ZONE_PX - Math.max(activeEdge.distance, 0)
  ) / AUTO_SCROLL_ZONE_PX;
  return activeEdge.direction * Math.ceil(ratio * AUTO_SCROLL_MAX_DELTA_PX);
}
