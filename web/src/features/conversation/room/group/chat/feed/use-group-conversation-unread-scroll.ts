/**
 * INPUT: Room 轮次身份、共享滚动 refs、FOLLOW 暂停动作与会话作用域。
 * OUTPUT: 已挂载节点直接定位，或虚拟节点先挂载再有限帧收口的可取消未读跳转。
 * POS: Room 未读协调器的滚动事务；不读取 Store、不消费未读消息。
 */
import { useCallback, useEffect, useRef } from "react";

import {
  findConversationRoundElement,
  scrollToConversationRoundElement,
  type ConversationRoundScrollHandleRef,
} from "@/features/conversation/shared/timeline/scroll/round-scroll";

interface UseGroupConversationUnreadScrollOptions {
  pauseFollowLatest: () => void;
  roundIds: string[];
  roundScrollRef: ConversationRoundScrollHandleRef;
  scopeKey: string;
  scrollRef: React.RefObject<HTMLDivElement | null>;
}

interface GroupConversationUnreadScrollModel {
  cancelPendingPosition: () => void;
  scrollToUnreadNode: (
    nodeId: string,
    behavior: ScrollBehavior,
  ) => boolean;
}

export function useGroupConversationUnreadScroll({
  pauseFollowLatest,
  roundIds,
  roundScrollRef,
  scopeKey,
  scrollRef,
}: UseGroupConversationUnreadScrollOptions): GroupConversationUnreadScrollModel {
  const virtualRefinementFrameRef = useRef<number | null>(null);

  const cancelPendingPosition = useCallback(() => {
    if (virtualRefinementFrameRef.current === null) {
      return;
    }
    window.cancelAnimationFrame(virtualRefinementFrameRef.current);
    virtualRefinementFrameRef.current = null;
  }, []);

  const refineVirtualPosition = useCallback((
    nodeId: string,
    behavior: ScrollBehavior,
  ) => {
    cancelPendingPosition();
    let attemptsRemaining = 6;
    const refine = () => {
      virtualRefinementFrameRef.current = null;
      const scrollElement = scrollRef.current;
      const target = scrollElement
        ? findConversationRoundElement(scrollElement, nodeId)
        : null;
      if (scrollElement && target) {
        scrollToConversationRoundElement(scrollElement, target, {
          align: "focus",
          behavior,
          target: "round",
        });
        return;
      }
      const handle = roundScrollRef.current;
      if (!handle || attemptsRemaining <= 0) {
        return;
      }
      attemptsRemaining -= 1;
      handle.scrollToRoundId(nodeId, {
        align: "focus",
        behavior: "auto",
        target: "round",
      });
      virtualRefinementFrameRef.current =
        window.requestAnimationFrame(refine);
    };
    virtualRefinementFrameRef.current = window.requestAnimationFrame(refine);
  }, [
    cancelPendingPosition,
    roundScrollRef,
    scrollRef,
  ]);

  const scrollToUnreadNode = useCallback((
    nodeId: string,
    behavior: ScrollBehavior,
  ): boolean => {
    if (!roundIds.includes(nodeId)) {
      return false;
    }
    const scrollElement = scrollRef.current;
    const target = scrollElement
      ? findConversationRoundElement(scrollElement, nodeId)
      : null;
    if (scrollElement && target) {
      cancelPendingPosition();
      pauseFollowLatest();
      scrollToConversationRoundElement(scrollElement, target, {
        align: "focus",
        behavior,
        target: "round",
      });
      return true;
    }
    const handle = roundScrollRef.current;
    if (!handle) {
      return false;
    }
    pauseFollowLatest();
    const didScroll = handle.scrollToRoundId(nodeId, {
      align: "focus",
      behavior: "auto",
      target: "round",
    });
    if (didScroll) {
      refineVirtualPosition(nodeId, behavior);
    }
    return didScroll;
  }, [
    cancelPendingPosition,
    pauseFollowLatest,
    refineVirtualPosition,
    roundIds,
    roundScrollRef,
    scrollRef,
  ]);

  useEffect(
    () => cancelPendingPosition,
    [cancelPendingPosition, scopeKey],
  );

  return {
    cancelPendingPosition,
    scrollToUnreadNode,
  };
}
