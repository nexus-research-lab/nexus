/**
 * INPUT: runtime 每轮结束后上报的上下文占用快照。
 * OUTPUT: 紧凑环形指标与唯一详情；点击幂等展开、键盘焦点保留，快照消失即清空打开态。
 * POS: DM 与 Room 共用的只读上下文用量视图。
 */

import {
  useCallback,
  useEffect,
  useRef,
} from "react";
import { createPortal } from "react-dom";

import { formatTokens } from "@/lib/format/token-count";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveUiAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-layout";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "@/shared/ui/overlay/overlay-styles";
import { getUiToneClassName, getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ContextUsageData } from "@/types/generated/protocol";

import type { ComposerContextUsageItem } from "../../composer-model";
import {
  projectComposerContextUsage,
  type ContextUsageItemProjection,
  type ContextUsageProjection,
} from "./composer-context-usage-model";

interface ComposerContextUsageProps {
  items?: readonly ComposerContextUsageItem[];
  usage: ContextUsageData | null;
}

const CONTEXT_USAGE_CLOSE_DELAY_MS = 100;

export function ComposerContextUsage({
  items,
  usage,
}: ComposerContextUsageProps) {
  const { t } = useI18n();
  const anchorRef = useRef<HTMLButtonElement>(null);
  const closeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const projection = projectComposerContextUsage({ items, usage });
  const [isOpen, setIsOpen] = useResettableState(false, Boolean(projection));
  const grouped = projection?.grouped ?? false;
  const estimatePosition = useCallback(
    (anchor: HTMLButtonElement) => resolveUiAnchoredOverlayPosition({
      align: "end",
      anchor,
      placement: "top",
      // 双行用量、字体与成员名称决定实际高度；preset 只提供上限，不能用行数估算裁切内容。
      preset: grouped ? "status-list" : "status-summary",
    }),
    [grouped],
  );
  const cancelScheduledClose = useCallback(() => {
    if (closeTimerRef.current) {
      clearTimeout(closeTimerRef.current);
      closeTimerRef.current = null;
    }
  }, []);
  const close = useCallback(() => {
    cancelScheduledClose();
    setIsOpen(false);
  }, [cancelScheduledClose, setIsOpen]);
  const open = useCallback(() => {
    cancelScheduledClose();
    setIsOpen(true);
  }, [cancelScheduledClose, setIsOpen]);
  const scheduleClose = useCallback(() => {
    cancelScheduledClose();
    closeTimerRef.current = setTimeout(
      () => {
        closeTimerRef.current = null;
        if (document.activeElement !== anchorRef.current) setIsOpen(false);
      },
      CONTEXT_USAGE_CLOSE_DELAY_MS,
    );
  }, [cancelScheduledClose, setIsOpen]);
  useEffect(() => cancelScheduledClose, [cancelScheduledClose, setIsOpen]);
  const {
    overlayId,
    overlayPosition,
    overlayRef,
    overlayStyle,
    portalContainer,
  } = useAnchoredOverlayLayer({
    anchorRef,
    disabled: !projection,
    estimatePosition,
    isOpen,
    onClose: close,
    restoreFocus: false,
  });
  if (!projection) {
    return (
      <span
        aria-hidden="true"
        className="h-7 w-7 shrink-0"
        data-context-usage-slot="empty"
      />
    );
  }

  const summary = projection.summary;
  const usedTokens = formatTokens(summary.totalTokens);
  const maxTokens = formatTokens(summary.maxTokens);
  const ariaLabel = projection.grouped
    ? t("composer.context_room_usage_label", {
        count: projection.items.length,
        percentage: summary.percentage,
      })
    : t("composer.context_usage_label", {
        max: maxTokens,
        percentage: summary.percentage,
        used: usedTokens,
      });

  return (
    <>
      <UiIconButton
        ref={anchorRef}
        aria-describedby={isOpen ? overlayId : undefined}
        aria-label={ariaLabel}
        className="shrink-0"
        data-context-usage={summary.percentage}
        data-context-usage-slot="ready"
        onBlur={scheduleClose}
        onClick={open}
        onFocus={open}
        onMouseEnter={open}
        onMouseLeave={scheduleClose}
        size="sm"
        tooltip={null}
        variant="ghost"
      >
        <svg
          aria-hidden="true"
          className="h-4 w-4 -rotate-90"
          viewBox="0 0 20 20"
        >
          <circle
            className="text-[color:color-mix(in_srgb,var(--text-soft)_22%,transparent)]"
            cx="10"
            cy="10"
            fill="none"
            pathLength="100"
            r="7.5"
            stroke="currentColor"
            strokeWidth="2"
          />
          <circle
            className={getUiToneClassName(summary.tone)}
            cx="10"
            cy="10"
            fill="none"
            pathLength="100"
            r="7.5"
            stroke="currentColor"
            strokeDasharray={`${summary.percentage} 100`}
            strokeLinecap="round"
            strokeWidth="2"
          />
        </svg>
      </UiIconButton>
      {isOpen && anchorRef.current && portalContainer
        ? createPortal(
            <div
              ref={overlayRef}
              className={`pointer-events-auto fixed left-0 top-0 ui-layer-dialog-interaction flex flex-col overflow-hidden shadow-(--surface-popover-shadow) ${OVERLAY_SURFACE_CLASS_NAME} ${ANCHORED_OVERLAY_MOTION_CLASS_NAME}`}
              data-placement={overlayPosition?.placement ?? "top"}
              id={overlayId}
              onMouseEnter={cancelScheduledClose}
              onMouseLeave={scheduleClose}
              role="tooltip"
              style={overlayStyle}
              {...OPEN_OVERLAY_DATA_ATTRIBUTES}
            >
              {projection.grouped ? (
                <GroupedContextUsage
                  items={projection.items}
                  title={t("composer.context_window_by_agent")}
                />
              ) : (
                <SingleContextUsage
                  maxTokens={maxTokens}
                  projection={summary}
                  title={t("composer.context_window")}
                  usedTokens={usedTokens}
                />
              )}
            </div>,
            portalContainer,
          )
        : null}
    </>
  );
}

function SingleContextUsage({
  maxTokens,
  projection,
  title,
  usedTokens,
}: {
  maxTokens: string;
  projection: ContextUsageProjection;
  title: string;
  usedTokens: string;
}) {
  const { t } = useI18n();
  return (
    <div className="px-3 py-2 text-center">
      <span className={`block ${getUiTypographyClassName({ role: "caption", tone: "muted" })}`}>
        {title}
      </span>
      <span className={`mt-0.5 block ${getUiTypographyClassName({ role: "control", tone: "strong", weight: "medium" })}`}>
        {t("composer.context_used_percent", {
          percentage: projection.percentage,
        })}
      </span>
      <span className={`mt-0.5 block tabular-nums ${getUiTypographyClassName({ role: "caption", tone: "default" })}`}>
        {t("composer.context_token_usage", {
          max: maxTokens,
          used: usedTokens,
        })}
      </span>
    </div>
  );
}

function GroupedContextUsage({
  items,
  title,
}: {
  items: readonly ContextUsageItemProjection[];
  title: string;
}) {
  return (
    <>
      <div className={`shrink-0 border-b border-(--divider-subtle-color) px-2.5 py-1.5 ${getUiTypographyClassName({ role: "caption", tone: "strong", weight: "medium" })}`}>
        {title}
      </div>
      <ul aria-label={title} className="soft-scrollbar min-h-0 overflow-y-auto overscroll-contain p-1">
        {items.map((item) => (
          <ContextUsageAgentRow item={item} key={item.agentId} />
        ))}
      </ul>
    </>
  );
}

function ContextUsageAgentRow({
  item,
}: {
  item: ContextUsageItemProjection;
}) {
  const { t } = useI18n();
  return (
    <li className="radius-control-sm flex min-h-8 items-center gap-1.5 px-1.5 py-1">
      <UiAgentAvatar
        avatar={item.avatar}
        name={item.name}
        size="xs"
      />
      <span className={`min-w-0 flex-1 [overflow-wrap:anywhere] ${getUiTypographyClassName({ role: "caption", tone: "strong", weight: "medium" })}`}>
        {item.name}
      </span>
      {item.usage ? (
        <span className="shrink-0 text-right tabular-nums">
          <span className={`block ${getUiTypographyClassName({ role: "caption", tone: "default", weight: "medium" })}`}>
            {t("composer.context_used_percent", {
              percentage: item.usage.percentage,
            })}
          </span>
          <span className={`block ${getUiTypographyClassName({ role: "caption", tone: "muted" })}`}>
            {formatTokens(item.usage.totalTokens)} / {formatTokens(item.usage.maxTokens)}
          </span>
        </span>
      ) : (
        <span className={`shrink-0 ${getUiTypographyClassName({ role: "caption", tone: "muted" })}`}>
          {t("composer.context_no_snapshot")}
        </span>
      )}
    </li>
  );
}
