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
  scrollRef: RefObject<HTMLDivElement | null>;
}

interface ActiveDragRuntime {
  container: HTMLDivElement;
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
    reorderMessages,
  } = commands;
  const [dragState, setDragState] = useState(EMPTY_DRAG_STATE);
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [isActionRunning, setIsActionRunning] = useState(false);
  const draggingMessageIdRef = useRef<string | null>(null);
  const pointerYRef = useRef<number | null>(null);
  const scrollFrameRef = useRef<number | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
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
    if (delta !== 0) {
      activeRuntime.container.scrollTop += delta;
    }
    runtime.scrollFrameRef.current = requestAnimationFrame(runAutoScroll);
  }, [runtime]);

  const startAutoScroll = useCallback((clientY: number) => {
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

  const startDrag = useCallback((messageId: string) => {
    runtime.draggingMessageIdRef.current = messageId;
    setDragState({ draggingMessageId: messageId, dragOverMessageId: null });
  }, [runtime]);

  const dragOver = useCallback((messageId: string, clientY: number) => {
    startAutoScroll(clientY);
    setDragState((current) => (
      current.dragOverMessageId === messageId
        ? current
        : { ...current, dragOverMessageId: messageId }
    ));
  }, [startAutoScroll]);

  const dropOnMessage = useCallback((targetId: string) => {
    const sourceId = dragState.draggingMessageId;
    if (!sourceId) {
      return;
    }
    void reorderMessages(
      reorderPendingMessageIds(items, sourceId, targetId),
    );
    finishDrag();
  }, [dragState.draggingMessageId, finishDrag, items, reorderMessages]);

  const guideMessage = useCallback(async (messageId: string) => {
    if (isActionRunning) {
      return;
    }
    setIsActionRunning(true);
    try {
      await runGuideMessage(messageId);
    } catch (error) {
      console.error("引导队列消息失败:", error);
    } finally {
      setIsActionRunning(false);
    }
  }, [isActionRunning, runGuideMessage]);

  const deleteMessage = useCallback((messageId: string) => {
    void runDeleteMessage(messageId);
  }, [runDeleteMessage]);

  const toggleCollapsed = useCallback(() => {
    setIsCollapsed((current) => !current);
  }, []);

  return {
    actions: {
      deleteMessage,
      dragOver,
      dropOnMessage,
      finishDrag,
      guideMessage,
      startAutoScroll,
      startDrag,
      toggleCollapsed,
    },
    refs: { scrollRef: runtime.scrollRef },
    state: { dragState, isActionRunning, isCollapsed },
  };
}

function readActiveDragRuntime(
  runtime: PendingQueueDragRuntime,
): ActiveDragRuntime | null {
  const container = runtime.scrollRef.current;
  const pointerY = runtime.pointerYRef.current;
  const isActive = [
    Boolean(container),
    pointerY !== null,
    Boolean(runtime.draggingMessageIdRef.current),
  ].every(Boolean);
  if (!isActive) {
    return null;
  }
  return {
    container: container as HTMLDivElement,
    pointerY: pointerY as number,
  };
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
