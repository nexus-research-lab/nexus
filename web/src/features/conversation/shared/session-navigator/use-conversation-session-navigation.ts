import { useCallback, useMemo, useState } from "react";
import type { RefObject } from "react";

import type { ConversationRoundScrollHandleRef } from "../timeline/round-scroll";
import type { ConversationTimeline } from "../timeline/timeline-model";
import {
  buildSessionNavigationItems,
  type SessionNavigationItem,
} from "./session-navigator-model";
import { useActiveRound } from "./use-active-round";
import { useRoundJump } from "./use-round-jump";

interface UseConversationSessionNavigationParams {
  timeline: ConversationTimeline;
  onLoadRoundWindow?: (roundId: string) => Promise<boolean>;
  onNavigateStart?: () => void;
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scopeKey: string | null;
  scrollRef: RefObject<HTMLDivElement | null>;
}

interface PreviewSelection {
  roundId: string;
  scopeKey: string;
}

/** 只组合导航展示状态；滚动同步和跳转事务由各自控制器维护。 */
export function useConversationSessionNavigation({
  timeline,
  onLoadRoundWindow,
  onNavigateStart,
  roundScrollRef,
  scopeKey,
  scrollRef,
}: UseConversationSessionNavigationParams) {
  const items = useMemo(
    () => buildSessionNavigationItems(timeline),
    [timeline],
  );
  const roundIds = useMemo(
    () => items.map((item) => item.roundId),
    [items],
  );
  const { activeRoundId, activateRound } = useActiveRound({
    roundIds,
    scopeKey,
    scrollRef,
  });
  const { jumpToRound } = useRoundJump({
    activateRound,
    onLoadRoundWindow,
    onNavigateStart,
    roundScrollRef,
    scopeKey,
    scrollRef,
    timeline,
  });
  const [previewSelection, setPreviewSelection] =
    useState<PreviewSelection | null>(null);
  const previewItem =
    previewSelection?.scopeKey === scopeKey
      ? items.find((item) => item.roundId === previewSelection.roundId) ?? null
      : null;

  const clearPreview = useCallback((): void => {
    setPreviewSelection(null);
  }, []);
  const previewItemAt = useCallback(
    (item: SessionNavigationItem): void => {
      if (scopeKey) {
        setPreviewSelection({ roundId: item.roundId, scopeKey });
      }
    },
    [scopeKey],
  );

  return {
    activeItem:
      items.find((item) => item.roundId === activeRoundId) ?? items[0] ?? null,
    clearPreview,
    items,
    jumpToRound,
    previewIndex: previewItem?.index ?? null,
    previewItem,
    previewItemAt,
  };
}
