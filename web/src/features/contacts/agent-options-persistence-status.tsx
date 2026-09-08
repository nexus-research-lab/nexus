// INPUT: 精确 Agent 身份、详情 Header 密度、自动保存阶段与原始用户文案。
// OUTPUT: 语义保存反馈与共享 Portal/键盘退出/视口约束的完整错误详情；作用域变化关闭旧详情。
// POS: Contacts Agent 详情头部状态组件；不执行保存或解释失败原因。

"use client";

import { useCallback, useEffect, useId, useRef } from "react";
import { createPortal } from "react-dom";

import { Check, CircleAlert, LoaderCircle } from "lucide-react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveUiAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-layout";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";
import { focusAfterAnchoredOverlayExit } from "@/shared/ui/overlay/overlay-focus-navigation";

import type { AgentOptionsPersistenceState } from "@/features/agents/options/agent-options-editor-model";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { getUiOverlayLayerClassName } from "@/shared/ui/overlay/layer-styles";
import { OVERLAY_SURFACE_CLASS_NAME } from "@/shared/ui/overlay/overlay-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function AgentOptionsPersistenceStatus({
  agentId,
  compact,
  state,
}: {
  agentId: string;
  compact: boolean;
  state: AgentOptionsPersistenceState;
}) {
  const { t } = useI18n();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const statusId = useId();
  const isError = state.phase === "error";
  const [detailsOpen, setDetailsOpen] = useResettableState(false,
    JSON.stringify([agentId, compact, state.message, state.phase]));
  const closeDetails = useCallback(() => setDetailsOpen(false), [setDetailsOpen]);
  const { overlayId, overlayPosition, overlayRef, overlayStyle, portalContainer } = useAnchoredOverlayLayer({
    anchorRef: triggerRef,
    disabled: !isError,
    estimatePosition: resolveSaveDetailsPosition,
    isOpen: detailsOpen && isError,
    onClose: closeDetails,
  });
  const positioned = overlayPosition !== null;
  useEffect(() => {
    const root = overlayRef.current;
    if (!detailsOpen || !isError || !positioned || !root) return;
    root.focus({ preventScroll: true });
    // 非模态只读详情只管理退出键，不把整个文本区域伪装成可点击控件。
    const handleTabExit = (event: KeyboardEvent) => {
      if (event.key !== "Tab" || event.defaultPrevented || isImeKeyboardEvent(event)) return;
      event.preventDefault();
      event.stopPropagation();
      closeDetails();
      triggerRef.current?.focus();
      focusAfterAnchoredOverlayExit([root], event.shiftKey);
    };
    root.addEventListener("keydown", handleTabExit);
    return () => root.removeEventListener("keydown", handleTabExit);
  }, [closeDetails, detailsOpen, isError, overlayRef, positioned]);

  const StatusIcon = state.phase === "saving"
    ? LoaderCircle
    : state.phase === "success"
      ? Check
      : null;

  return (
    <>
      <span
        className={cn(
          "mr-1 inline-flex h-8 min-w-0 items-center gap-1",
          getUiTypographyClassName({ role: "metadata", tone: isError ? "danger" : state.phase === "success" ? "success" : "muted" }),
        )}
      >
        <span aria-live="polite" className="sr-only" id={statusId} role="status">{state.message}</span>
        {isError ? (
          <UiIconButton
            ref={triggerRef}
            aria-controls={detailsOpen ? overlayId : undefined}
            aria-describedby={statusId}
            aria-expanded={detailsOpen}
            aria-haspopup="dialog"
            aria-label={t("agent_options.save_status")}
            data-agent-save-error-details
            onClick={() => setDetailsOpen((current) => !current)}
            size="md"
            tone="danger"
            tooltip={null}
            variant="ghost"
          >
            <CircleAlert aria-hidden="true" className="h-3.5 w-3.5" />
          </UiIconButton>
        ) : StatusIcon ? (
          <StatusIcon
            aria-hidden="true"
            className={state.phase === "saving"
              ? getUiSpinnerClassName({ size: "sm" })
              : "h-3.5 w-3.5"}
          />
        ) : null}
        {!compact ? <span aria-hidden="true" className="max-w-40 truncate">{state.message}</span> : null}
      </span>
      {isError && detailsOpen && portalContainer ? createPortal(
        <div
          ref={overlayRef}
          aria-describedby={`${overlayId}-message`}
          aria-label={t("agent_options.save_status")}
          className={cn(
            "fixed overflow-y-auto whitespace-pre-wrap wrap-anywhere px-3 py-2.5 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]",
            getUiOverlayLayerClassName("popover"),
            OVERLAY_SURFACE_CLASS_NAME,
            getUiTypographyClassName({ role: "supporting", tone: "default" }),
          )}
          data-agent-save-error-popover
          data-placement={overlayPosition?.placement ?? "bottom"}
          data-state="open"
          id={overlayId}
          role="dialog"
          style={overlayStyle}
          tabIndex={-1}
          {...OPEN_OVERLAY_DATA_ATTRIBUTES}
        >
          <span id={`${overlayId}-message`}>{state.message}</span>
        </div>, portalContainer,
      ) : null}
    </>
  );
}

function resolveSaveDetailsPosition(anchor: HTMLButtonElement) {
  return resolveUiAnchoredOverlayPosition({ anchor, align: "end", placement: "bottom", preset: "reference-list" });
}
