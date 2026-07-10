import type { RefObject } from "react";
import { ChevronRight, MessageSquareText } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  UiDialogBody,
  UiDialogHeader,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
} from "@/shared/ui/dialog/dialog-styles";

import type { ConversationRoundScrollHandleRef } from "../timeline/round-scroll";
import type { ConversationTimeline } from "../timeline/timeline-model";
import {
  buildTickBackground,
  buildTickVisual,
  formatSpeakerSummary,
  getRulerTrackHeight,
  getTickDisplayPercent,
  RULER_TRACK_BOTTOM_SAFE_INSET_PX,
  RULER_TRACK_TOP_SAFE_INSET_PX,
} from "./session-navigator-model";
import { useConversationSessionNavigation } from "./use-conversation-session-navigation";

interface ConversationSessionNavigatorProps {
  agentNameMap?: Record<string, string>;
  className?: string;
  /** 导航只消费统一时间线投影，不自行分组消息。 */
  timeline: ConversationTimeline;
  onLoadRoundWindow?: (roundId: string) => Promise<boolean>;
  onNavigateStart?: () => void;
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scrollRef: RefObject<HTMLDivElement | null>;
}

export function ConversationSessionNavigator({
  agentNameMap,
  className,
  timeline,
  onLoadRoundWindow,
  onNavigateStart,
  roundScrollRef,
  scrollRef,
}: ConversationSessionNavigatorProps) {
  const {
    activeItem,
    clearPreview,
    items,
    jumpToRound,
    previewIndex,
    previewItem,
    previewItemAt,
  } = useConversationSessionNavigation({
    timeline,
    onLoadRoundWindow,
    onNavigateStart,
    roundScrollRef,
    scrollRef,
  });

  if (items.length <= 1) {
    return null;
  }

  const trackHeight = getRulerTrackHeight(items.length);
  return (
    <nav
      aria-label="会话导航"
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
                  aria-label={`跳转到${item.title}`}
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
                    className="block h-[2px] rounded-full transition-[width,opacity,filter] duration-[90ms] ease-out"
                    style={tickVisual}
                  />
                </button>
              );
            })}

            {previewItem ? (
              <UiDialogShell
                className="pointer-events-auto absolute left-12 z-[60] w-[min(332px,calc(100vw-96px))] max-w-none -translate-y-1/2 outline-none"
                data-session-navigator-preview="true"
                size="sm"
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
              >
                <UiDialogHeader
                  className="cursor-pointer gap-2 px-3 py-2.5"
                  onClick={() => {
                    jumpToRound(previewItem);
                  }}
                >
                  <div
                    className={cn(
                      DIALOG_HEADER_LEADING_CLASS_NAME,
                      "min-w-0 flex-1 items-center",
                    )}
                  >
                    <div
                      className={cn(
                        DIALOG_HEADER_ICON_CLASS_NAME,
                        "h-7 w-7 rounded-[10px] bg-primary/10 text-primary",
                      )}
                    >
                      <MessageSquareText className="h-3.5 w-3.5" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <h3 className="truncate text-[13px] font-semibold leading-[18px] text-(--text-strong)">
                        {previewItem.title}
                      </h3>
                      <p className="mt-0.5 truncate text-[11px] leading-4 text-(--text-muted)">
                        {previewItem.time}
                      </p>
                    </div>
                    <ChevronRight className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
                  </div>
                </UiDialogHeader>
                <UiDialogBody
                  className="cursor-pointer px-3 py-2.5"
                  onClick={() => {
                    jumpToRound(previewItem);
                  }}
                >
                  <p className="line-clamp-2 text-[11px] leading-[18px] text-(--text-default)">
                    {previewItem.summary}
                  </p>
                  <div className="mt-2 flex min-w-0 items-center gap-1.5 text-[10px] font-medium leading-4 text-(--text-soft)">
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
                      {formatSpeakerSummary(previewItem, agentNameMap)}
                    </span>
                    <span className="text-(--text-soft)">·</span>
                    <span className="truncate">{previewItem.meta}</span>
                  </div>
                </UiDialogBody>
              </UiDialogShell>
            ) : null}
          </div>
        </div>
      </div>
    </nav>
  );
}
