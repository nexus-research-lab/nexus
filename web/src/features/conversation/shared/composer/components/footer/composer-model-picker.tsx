"use client";

/**
 * INPUT: DM 当前 Session 的模型/思考强度投影与更新命令。
 * OUTPUT: 显示“模型 · 强度”的入口与模型浮层；宽视口时强度档位作为侧栏在模型面板左侧级联，窄视口在面板内原位进入；选择后关闭并归还焦点。
 * POS: DM 直接模型入口；菜单正文与 Room 共用，持久化与失败对账归控制器。
 */

import { ChevronDown } from "lucide-react";
import { type CSSProperties, type ReactNode, useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import { useI18n } from "@/shared/i18n/i18n-context";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { useMediaQuery } from "@/shared/lib/react/use-media-query";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { focusFirstMenuItem, handleMenuKeyDown } from "@/shared/ui/menu/menu-keyboard";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveUiAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-layout";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";
import { isTopAnchoredOverlay } from "@/shared/ui/overlay/overlay-dismissal-runtime";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "@/shared/ui/overlay/overlay-styles";

import type {
  ComposerSessionSettingsController,
} from "../../controller/use-composer-session-settings";
import {
  getSessionEffortMenuContentHeight,
  getSessionModelMenuContentHeight,
  ROOM_MODEL_MENU_GAP,
  SESSION_DIRECT_MODEL_MENU_WIDTH,
  SESSION_EFFORT_CASCADE_QUERY,
  SESSION_EFFORT_MENU_WIDTH,
} from "./composer-session-control-layout";
import {
  applySessionEffortSelection,
  applySessionModelSelection,
  getSessionModelMenuShape,
  sessionEffortLevels,
  sessionEffortLabel,
} from "./composer-session-control-options";
import {
  SessionEffortMenu,
  SessionModelMenu,
} from "./composer-session-model-menu";

/** 打开或切换层级后要落焦的位置；entry 是从强度面板退回时的强度入口。 */
type PendingFocus = "models" | "effort" | "entry";

export function ComposerModelPicker({
  controller,
  disabled,
}: {
  controller: ComposerSessionSettingsController;
  disabled: boolean;
}) {
  const { t } = useI18n();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const modelMenuRef = useRef<HTMLDivElement>(null);
  const effortMenuRef = useRef<HTMLDivElement>(null);
  const pendingFocusRef = useRef<PendingFocus | null>(null);
  const canShowSideEffort = useMediaQuery(SESSION_EFFORT_CASCADE_QUERY);
  const [isOpen, setIsOpen] = useResettableState(false, controller.target?.sessionKey);
  const [effortOpen, setEffortOpen] = useState(false);
  const blocked = disabled || controller.modelBusy;
  const close = useCallback(() => {
    pendingFocusRef.current = null;
    setIsOpen(false);
    setEffortOpen(false);
  }, [setIsOpen]);
  useEffect(() => {
    if (blocked) close();
  }, [blocked, close]);

  const sideEffort = canShowSideEffort && effortOpen && sessionEffortLevels(controller).length > 0;
  const inlineEffort = !canShowSideEffort && effortOpen;
  const shape = getSessionModelMenuShape(controller);
  const modelHeight = getSessionModelMenuContentHeight(shape, inlineEffort ? "effort" : "models");
  const contentWidth = sideEffort
    ? SESSION_DIRECT_MODEL_MENU_WIDTH + ROOM_MODEL_MENU_GAP + SESSION_EFFORT_MENU_WIDTH
    : SESSION_DIRECT_MODEL_MENU_WIDTH;
  const contentHeight = sideEffort
    ? Math.max(modelHeight, getSessionEffortMenuContentHeight(shape, false))
    : modelHeight;
  const estimatePosition = useCallback((anchor: HTMLButtonElement) => (
    resolveUiAnchoredOverlayPosition({
      align: "end",
      anchor,
      contentWidth,
      estimatedContentHeight: contentHeight,
      placement: "top",
      preset: "cascade-menu",
    })
  ), [contentHeight, contentWidth]);
  const {
    overlayId,
    overlayPosition,
    overlayRef,
    overlayStyle,
    portalContainer,
  } = useAnchoredOverlayLayer({
    anchorRef: triggerRef,
    disabled: blocked,
    estimatePosition,
    isOpen: isOpen && !blocked,
    onClose: close,
  });
  const panelStyle = { maxHeight: overlayStyle.maxHeight };
  const effortMenuId = `${overlayId}-effort`;

  // 打开或切换层级后，等浮层定位可见再落焦：当前项优先，其次首项；退回时回到强度入口。
  useEffect(() => {
    const pending = pendingFocusRef.current;
    if (!isOpen || !overlayPosition || !pending) return;
    const menu = pending === "effort" ? effortMenuRef.current : modelMenuRef.current;
    if (!menu) return;
    pendingFocusRef.current = null;
    const current = pending === "entry"
      ? menu.querySelector<HTMLElement>("[data-session-effort-entry]")
      : menu.querySelector<HTMLElement>('[data-active="true"], [aria-checked="true"]');
    if (current) current.focus();
    else focusFirstMenuItem(menu);
  });

  const closeAndRestoreFocus = () => {
    close();
    triggerRef.current?.focus();
  };
  const openEffort = (moveFocus: boolean) => {
    if (moveFocus) pendingFocusRef.current = "effort";
    setEffortOpen(true);
  };
  const closeEffort = (moveFocus: boolean) => {
    // 原位子页退回时入口所在列表重新挂载，焦点必须显式归还。
    if (moveFocus || inlineEffort) pendingFocusRef.current = "entry";
    setEffortOpen(false);
  };
  const effortVisible = sessionEffortLevels(controller).length > 0 && Boolean(controller.effort);

  return (
    <>
      <UiButton
        ref={triggerRef}
        aria-controls={isOpen ? overlayId : undefined}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={t("composer.session_model")}
        className="min-w-0 max-w-52"
        disabled={blocked}
        onClick={() => {
          if (isOpen) {
            close();
            return;
          }
          pendingFocusRef.current = "models";
          setEffortOpen(false);
          setIsOpen(true);
        }}
        size="xs"
        variant="ghost"
      >
        <span className="truncate">{controller.modelLabel}</span>
        {effortVisible ? (
          <span className="shrink-0 text-(--text-soft)">
            · {sessionEffortLabel(controller.effort, t)}
          </span>
        ) : null}
        <ChevronDown className="h-3 w-3 shrink-0" />
      </UiButton>

      {isOpen && !blocked && portalContainer ? createPortal(
        // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- 非模态 dialog 委派菜单导航、强度面板返回和 Tab 退出，不充当动作按钮。
        <div
          ref={overlayRef}
          aria-label={t("composer.session_model")}
          className={cn(
            "fixed ui-layer-popover flex min-h-0",
            ANCHORED_OVERLAY_MOTION_CLASS_NAME,
            overlayPosition?.placement === "top" ? "items-end" : "items-start",
          )}
          data-placement={overlayPosition?.placement ?? "top"}
          id={overlayId}
          onKeyDown={(event) => {
            if (event.defaultPrevented || isImeKeyboardEvent(event.nativeEvent)
              || !isTopAnchoredOverlay(overlayRef.current)
              || !(event.target instanceof Node) || !event.currentTarget.contains(event.target)) return;
            const inEffortMenu = Boolean(effortMenuRef.current?.contains(event.target));
            // 侧栏在左：右方向键回到模型面板；原位子页：左方向键返回。Escape 两种布局都先退出强度。
            const leavesEffort = effortOpen && (event.key === "Escape" || (inEffortMenu && (
              sideEffort ? event.key === "ArrowRight" : event.key === "ArrowLeft"
            )));
            if (leavesEffort) {
              event.preventDefault();
              event.stopPropagation();
              closeEffort(true);
              return;
            }
            handleMenuKeyDown(event, closeAndRestoreFocus);
          }}
          role="dialog"
          style={{ ...overlayStyle, columnGap: ROOM_MODEL_MENU_GAP }}
          {...OPEN_OVERLAY_DATA_ATTRIBUTES}
        >
          {sideEffort ? (
            <ModelPickerPanel style={panelStyle} width={SESSION_EFFORT_MENU_WIDTH}>
              <SessionEffortMenu
                controller={controller}
                disabled={disabled}
                menuId={effortMenuId}
                menuRef={effortMenuRef}
                onSelect={(value) => {
                  applySessionEffortSelection(controller, value);
                  closeAndRestoreFocus();
                }}
              />
            </ModelPickerPanel>
          ) : null}
          <ModelPickerPanel style={panelStyle} width={sideEffort ? SESSION_DIRECT_MODEL_MENU_WIDTH : "100%"}>
            <SessionModelMenu
              ariaLabel={t("composer.session_model")}
              controller={controller}
              disabled={disabled}
              effortSide={canShowSideEffort ? {
                expanded: sideEffort,
                menuId: effortMenuId,
                onHoverChange: (expanded) => (expanded ? openEffort(false) : closeEffort(false)),
              } : undefined}
              menuRef={inlineEffort ? effortMenuRef : modelMenuRef}
              onSelectEffort={(value) => {
                applySessionEffortSelection(controller, value);
                closeAndRestoreFocus();
              }}
              onSelectModel={(value) => {
                applySessionModelSelection(controller, value);
                closeAndRestoreFocus();
              }}
              onViewChange={(next) => (next === "effort" ? openEffort(true) : closeEffort(true))}
              view={inlineEffort ? "effort" : "models"}
            />
          </ModelPickerPanel>
        </div>,
        portalContainer,
      ) : null}
    </>
  );
}

function ModelPickerPanel({
  children,
  style,
  width,
}: {
  children: ReactNode;
  style: CSSProperties;
  width: CSSProperties["width"];
}) {
  return (
    <div
      className={cn("flex min-h-0 min-w-0 shrink-0 flex-col overflow-hidden", OVERLAY_SURFACE_CLASS_NAME)}
      style={{ ...style, width }}
    >
      {children}
    </div>
  );
}
