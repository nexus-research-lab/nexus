/**
 * INPUT: 统一会话时间线、当前滚动位置与跳转命令。
 * OUTPUT: 时间刻度和轻量轮次预览浮层。
 * POS: Conversation 桌面宽屏导航；预览不是模态弹窗。
 */
import type { RefObject } from "react";
import { ChevronRight } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";

import type { ConversationRoundScrollHandleRef } from "../timeline/scroll/round-scroll";
import type { ConversationTimeline } from "../timeline/timeline-model";
import {
  buildTickBackground,
  buildTickVisual,
  formatSpeakerSummary,
  getRulerTrackHeight,
  getTickDisplayPercent,
  RULER_TRACK_BOTTOM_SAFE_INSET_PX,
  RULER_TRACK_TOP_SAFE_INSET_PX,
} from "./session-navigator-ruler-model";
import { useConversationSessionNavigation } from "./use-conversation-session-navigation";

interface ConversationSessionNavigatorProps {
  agentNameMap?: Record<string, string>;
  className?: string;
  /** 导航只消费统一时间线投影，不自行分组消息。 */
  timeline: ConversationTimeline;
  onLoadRoundWindow?: (roundId: string) => Promise<boolean>;
  onNavigateStart?: () => void;
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scopeKey: string | null;
  scrollRef: RefObject<HTMLDivElement | null>;
}

export function ConversationSessionNavigator({
  agentNameMap,
  className,
  timeline,
  onLoadRoundWindow,
  onNavigateStart,
  roundScrollRef,
  scopeKey,
  scrollRef,
}: ConversationSessionNavigatorProps) {
  const localization = useI18n();
  const { t } = localization;
  const {
    activeItem,
    clearPreview,
    items,
    jumpToRound,
    previewIndex,
    previewItem,
    previewItemAt,
  } = useConversationSessionNavigation({
    localization,
    timeline,
    onLoadRoundWindow,
    onNavigateStart,
    roundScrollRef,
    scopeKey,
    scrollRef,
  });

  if (items.length <= 1) {
    return null;
  }

  const trackHeight = getRulerTrackHeight(items.length);
  return (
    <nav
      aria-label={t("room.session_navigator_label")}
      className={cn(
        "pointer-events-none hidden h-auto w-11 select-none xl:block",
        className,
      )}
      onMouseLeave={clearPreview}
    >
      <div className="relative h-full min-h-[220px] w-full">
        <div
          className="pointer-events-auto absolute left-0 flex w-12 flex-col justify-center overflow-visible"
          style={{
            bottom: `${RULER_TRACK_BOTTOM_SAFE_INSET_PX}px`,
            top: `${RULER_TRACK_TOP_SAFE_INSET_PX}px`,
          }}
          onPointerLeave={clearPreview}
        >
          <div
            className="relative flex w-12 flex-col overflow-visible"
            style={{ height: `min(100%, ${trackHeight}px)` }}
          >
            {items.map((item) => {
              const isActive = item.roundId === activeItem?.roundId;
              const tickVisual = buildTickVisual(
                item,
                activeItem?.roundId ?? null,
                previewIndex,
                previewItem?.roundId ?? null,
              );
              return (
                <button
                  key={item.roundId}
                  type="button"
                  aria-current={isActive ? "true" : undefined}
                  aria-label={t("room.session_navigator_jump", {
                    title: item.title,
                  })}
                  className="flex min-h-0 w-12 flex-1 items-center justify-start rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-primary/35"
                  onClick={() => {
                    jumpToRound(item);
                  }}
                  onFocus={() => {
                    previewItemAt(item);
                  }}
                  onPointerEnter={() => {
                    previewItemAt(item);
                  }}
                >
                  <span
                    className="block h-[2px] rounded-full"
                    style={tickVisual}
                  />
                </button>
              );
            })}

            {previewItem ? (
              <button
                className="dialog-shell surface-radius-lg pointer-events-auto absolute left-12 z-[60] w-[min(332px,calc(100vw-96px))] -translate-y-1/2 overflow-hidden text-left outline-none focus-visible:ring-2 focus-visible:ring-primary/35"
                data-session-navigator-preview="true"
                style={{
                  top: `${getTickDisplayPercent(
                    previewItem.index,
                    items.length,
                  )}%`,
                }}
                onPointerDown={(event) => {
                  event.stopPropagation();
                }}
                onPointerEnter={(event) => {
                  event.stopPropagation();
                }}
                onPointerMove={(event) => {
                  event.stopPropagation();
                }}
                onClick={() => {
                  jumpToRound(previewItem);
                }}
                type="button"
              >
                <span className="flex min-w-0 items-center gap-2 border-b border-(--divider-subtle-color) px-3 py-2.5">
                  <span className="min-w-0 flex-1 truncate text-sm font-semibold leading-[18px] text-(--text-strong)">
                    {previewItem.title}
                  </span>
                  <span className="shrink-0 text-xs leading-4 text-(--text-muted)">
                    {previewItem.time}
                  </span>
                  <ChevronRight className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
                </span>
                <span className="block px-3 py-2.5">
                  <span className="line-clamp-2 text-xs leading-[18px] text-(--text-default)">
                    {previewItem.summary}
                  </span>
                  <span className="mt-2 flex min-w-0 items-center gap-1.5 text-2xs font-medium leading-4 text-(--text-soft)">
                    <span
                      className={cn(
                        "h-1.5 w-1.5 shrink-0 rounded-full",
                        previewItem.isLive
                          ? "bg-primary"
                          : "bg-(--icon-muted)",
                      )}
                      style={{
                        background: buildTickBackground(previewItem),
                      }}
                    />
                    <span className="truncate">
                      {formatSpeakerSummary(previewItem, t, agentNameMap)}
                    </span>
                    <span className="text-(--text-soft)">·</span>
                    <span className="truncate">{previewItem.meta}</span>
                  </span>
                </span>
              </button>
            ) : null}
          </div>
        </div>
      </div>
    </nav>
  );
}
