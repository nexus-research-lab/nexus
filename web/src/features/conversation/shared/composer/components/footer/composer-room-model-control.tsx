"use client";

/**
 * INPUT: Room 内各 Agent 的当前 Session 模型投影与更新动作。
 * OUTPUT: Composer 右侧按 Agent 级联选择模型的紧凑浮层，支持逐级键盘进入/返回与 Tab 退出。
 * POS: 群聊模型入口；宽屏悬浮级联，窄屏点击逐级进入；菜单遍历与行尺寸由 shared/menu 拥有。
 */

import {
  ArrowLeft,
  ChevronDown,
  ChevronRight,
  LoaderCircle,
} from "lucide-react";
import {
  type CSSProperties,
  type ReactNode,
  type RefObject,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

import { useI18n } from "@/shared/i18n/i18n-context";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import {
  UiActionMenuContent,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import { UiMenuActionRow } from "@/shared/ui/menu/menu-action-row";
import { focusFirstMenuItem, handleMenuKeyDown } from "@/shared/ui/menu/menu-keyboard";
import {
  getMenuItemLayout,
  MENU_ITEM_GAP_PX,
  MENU_LIST_CLASS_NAME,
  MENU_SURFACE_VERTICAL_PADDING_PX,
} from "@/shared/ui/menu/menu-styles";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import {
  getUiAnchoredOverlayMinimumWidth,
  getUiAnchoredOverlayViewportInset,
  resolveUiAnchoredOverlayPosition,
} from "@/shared/ui/overlay/anchored-overlay-layout";
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
  buildResetSessionSettingItem,
  buildSessionModelItems,
  decodeSessionModelValue,
  RESET_SESSION_SETTING_VALUE,
} from "./composer-session-control-options";

interface ComposerRoomModelControlProps {
  controller: ComposerSessionSettingsController;
  disabled: boolean;
}

type RoomModelView = "agents" | "models";

const ROOM_MODEL_AGENT_MENU_WIDTH = getUiAnchoredOverlayMinimumWidth("cascade-menu");
const ROOM_MODEL_MENU_WIDTH = 256;
const ROOM_MODEL_MENU_GAP = 8;

export function ComposerRoomModelControl({
  controller,
  disabled,
}: ComposerRoomModelControlProps) {
  const { t } = useI18n();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const agentMenuRef = useRef<HTMLDivElement>(null);
  const modelMenuRef = useRef<HTMLDivElement>(null);
  const pendingFocusRef = useRef<RoomModelView | null>(null);
  const returnAgentIdRef = useRef<string | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [view, setView] = useState<RoomModelView>("agents");
  const resetTarget = controller.resetTarget;
  const modelItems = buildSessionModelItems(controller);
  const resetItem = buildResetSessionSettingItem(
    !controller.hasModelOverride,
    t,
  );
  const close = useCallback(() => {
    pendingFocusRef.current = null;
    setIsOpen(false);
    setView("agents");
    resetTarget();
  }, [resetTarget]);
  const estimatePosition = useCallback((anchor: HTMLButtonElement) => (
    resolveUiAnchoredOverlayPosition({
      align: "end",
      anchor,
      estimatedContentHeight: estimateRoomModelMenuHeight({
        agentCount: controller.targetViews.length,
        modelCount: modelItems.length,
      }),
      placement: "top",
      preset: "cascade-menu",
    })
  ), [controller.targetViews.length, modelItems.length]);
  const {
    overlayId,
    overlayPosition,
    overlayRef,
    overlayStyle,
    portalContainer,
  } = useAnchoredOverlayLayer({
    anchorRef: triggerRef,
    disabled,
    estimatePosition,
    isOpen,
    onClose: close,
  });
  const viewportInset = getUiAnchoredOverlayViewportInset("cascade-menu");
  const canShowSideModels = typeof window !== "undefined"
    && window.innerWidth >= (
      ROOM_MODEL_AGENT_MENU_WIDTH
      + ROOM_MODEL_MENU_WIDTH
      + ROOM_MODEL_MENU_GAP
      + viewportInset * 2
    );
  const showSideModels = canShowSideModels && view === "models";
  const expandedWidth =
    ROOM_MODEL_AGENT_MENU_WIDTH
    + ROOM_MODEL_MENU_WIDTH
    + ROOM_MODEL_MENU_GAP;
  const singlePanelWidth = view === "agents"
    ? ROOM_MODEL_AGENT_MENU_WIDTH
    : ROOM_MODEL_MENU_WIDTH;
  const layoutWidth = showSideModels
    ? expandedWidth
    : singlePanelWidth;
  const layoutStyle = overlayPosition && typeof window !== "undefined"
    ? {
        ...overlayStyle,
        left: Math.max(
          viewportInset,
          Math.min(
            showSideModels
              ? overlayPosition.left
              : overlayPosition.left
                + overlayPosition.width
                - layoutWidth,
            window.innerWidth
              - layoutWidth
              - viewportInset,
          ),
        ),
        width: layoutWidth,
      }
    : overlayStyle;
  const panelStyle = { maxHeight: overlayStyle.maxHeight };

  useEffect(() => {
    if (!isOpen || !overlayPosition || !pendingFocusRef.current) return;
    const pendingView = pendingFocusRef.current;
    const menu = pendingView === "agents" ? agentMenuRef.current : modelMenuRef.current;
    if (!menu) return;
    pendingFocusRef.current = null;
    const agent = pendingView === "agents" ? Array.from(
      menu.querySelectorAll<HTMLElement>("[data-agent-id]"),
    ).find((item) => item.dataset.agentId === returnAgentIdRef.current) : undefined;
    if (agent) agent.focus();
    else focusFirstMenuItem(menu);
  });

  useEffect(() => {
    if ((disabled || controller.saving) && isOpen) {
      close();
    }
  }, [close, controller.saving, disabled, isOpen]);

  const toggle = () => {
    if (isOpen) {
      close();
      return;
    }
    setView("agents");
    pendingFocusRef.current = "agents";
    returnAgentIdRef.current = null;
    setIsOpen(true);
    void controller.ensureTargetsLoaded();
  };
  const selectAgent = (agentId: string, moveFocus = false) => {
    returnAgentIdRef.current = agentId;
    if (moveFocus) {
      if (view === "models" && controller.target?.agentId === agentId && modelMenuRef.current) {
        // 悬浮已打开同一目标时不会产生新提交，键盘进入应立即移动焦点。
        focusFirstMenuItem(modelMenuRef.current);
      } else {
        pendingFocusRef.current = "models";
      }
    }
    controller.selectTarget(agentId);
    setView("models");
  };
  const closeAndRestoreFocus = () => {
    close();
    triggerRef.current?.focus();
  };
  const backToAgents = () => {
    pendingFocusRef.current = "agents";
    setView("agents");
  };
  const selectModel = (value: string) => {
    if (value === RESET_SESSION_SETTING_VALUE) {
      void controller.resetModel();
      closeAndRestoreFocus();
      return;
    }
    const [provider, model] = decodeSessionModelValue(value);
    if (
      provider === controller.inheritedProvider
      && model === controller.inheritedModel
    ) {
      void controller.resetModel();
    } else {
      void controller.updateModel(provider, model);
    }
    closeAndRestoreFocus();
  };

  return (
    <>
      <UiButton
        ref={triggerRef}
        aria-controls={isOpen ? overlayId : undefined}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={t("composer.session_model")}
        className="min-w-0 max-w-44"
        disabled={disabled || controller.saving}
        onClick={toggle}
        size="xs"
        title={t("composer.session_model")}
        variant="ghost"
      >
        <span className="truncate">
          {t("composer.room_model")}
        </span>
        <ChevronDown className="h-3 w-3 shrink-0" />
      </UiButton>

      {isOpen && portalContainer ? createPortal(
        // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- 非模态 dialog 统一委派内部菜单导航、逐级返回和 Tab 退出，不充当动作按钮。
        <div
          ref={overlayRef}
          aria-label={t("composer.session_model")}
          className={cn(
            "fixed ui-layer-popover flex min-h-0 gap-2",
            ANCHORED_OVERLAY_MOTION_CLASS_NAME,
            overlayPosition?.placement === "top"
              ? "items-end"
              : "items-start",
          )}
          data-placement={overlayPosition?.placement ?? "top"}
          id={overlayId}
          onKeyDown={(event) => {
            if (event.defaultPrevented || isImeKeyboardEvent(event.nativeEvent)
              || !isTopAnchoredOverlay(overlayRef.current)
              || !(event.target instanceof Node) || !event.currentTarget.contains(event.target)) return;
            if (view === "models" && (event.key === "Escape"
              || (event.key === "ArrowLeft" && modelMenuRef.current?.contains(event.target)))) {
              event.preventDefault();
              event.stopPropagation();
              backToAgents();
              return;
            }
            handleMenuKeyDown(event, closeAndRestoreFocus);
          }}
          role="dialog"
          style={layoutStyle}
          {...OPEN_OVERLAY_DATA_ATTRIBUTES}
        >
          {canShowSideModels ? (
            <>
              <RoomModelPanel
                style={panelStyle}
                width={ROOM_MODEL_AGENT_MENU_WIDTH}
              >
                <RoomModelAgentList
                  activeAgentId={
                    showSideModels
                      ? controller.target?.agentId
                      : undefined
                  }
                  canHoverSelect
                  controller={controller}
                  disabled={disabled}
                  menuRef={agentMenuRef}
                  modelMenuId={`${overlayId}-models`}
                  onSelect={selectAgent}
                />
              </RoomModelPanel>
              {showSideModels ? (
                <RoomModelOptions
                  controller={controller}
                  disabled={disabled}
                  items={modelItems}
                  menuId={`${overlayId}-models`}
                  menuRef={modelMenuRef}
                  onSelect={selectModel}
                  resetItem={resetItem}
                  style={panelStyle}
                />
              ) : null}
            </>
          ) : (
            <RoomModelPanel
              style={panelStyle}
              width={singlePanelWidth}
            >
              {view === "agents" ? (
                <RoomModelAgentList
                  controller={controller}
                  disabled={disabled}
                  menuRef={agentMenuRef}
                  modelMenuId={`${overlayId}-models`}
                  onSelect={selectAgent}
                />
              ) : (
                <>
                  <RoomModelHeader
                    onBack={backToAgents}
                    title={controller.target?.name ?? ""}
                  />
                  <div
                    aria-label={t("composer.room_model_agent", { name: controller.target?.name ?? "" })}
                    className="soft-scrollbar min-h-0 overflow-y-auto overscroll-contain p-1"
                    id={`${overlayId}-models`}
                    ref={modelMenuRef}
                    role="menu"
                    tabIndex={-1}
                  >
                    <UiActionMenuContent
                      density="compact"
                      disabled={disabled || controller.modelBusy}
                      footerItems={[resetItem]}
                      items={modelItems}
                      onSelect={selectModel}
                    />
                  </div>
                </>
              )}
            </RoomModelPanel>
          )}
        </div>,
        portalContainer,
      ) : null}
    </>
  );
}

function RoomModelPanel({
  children,
  style,
  width,
}: {
  children: ReactNode;
  style: CSSProperties;
  width: number;
}) {
  return (
    <div
      className={cn(
        "flex min-h-0 shrink-0 flex-col overflow-hidden",
        OVERLAY_SURFACE_CLASS_NAME,
      )}
      style={{ ...style, width }}
    >
      {children}
    </div>
  );
}

function RoomModelAgentList({
  activeAgentId,
  canHoverSelect = false,
  controller,
  disabled,
  menuRef,
  modelMenuId,
  onSelect,
}: {
  activeAgentId?: string;
  canHoverSelect?: boolean;
  controller: ComposerSessionSettingsController;
  disabled: boolean;
  menuRef: RefObject<HTMLDivElement | null>;
  modelMenuId: string;
  onSelect: (agentId: string, moveFocus?: boolean) => void;
}) {
  const { t } = useI18n();
  return (
    <div className={cn(
      MENU_LIST_CLASS_NAME,
      "soft-scrollbar min-h-0 overflow-y-auto overscroll-contain p-1",
    )} aria-label={t("composer.room_model")} ref={menuRef} role="menu" tabIndex={-1}>
      {controller.targetViews.map((targetView) => {
        const isActive = targetView.target.agentId === activeAgentId;
        return (
          <UiMenuActionRow
            active={canHoverSelect && isActive}
            aria-controls={isActive ? modelMenuId : undefined}
            aria-expanded={isActive}
            aria-haspopup="menu"
            aria-label={t("composer.room_model_agent", {
              name: targetView.target.name,
            })}
            disabled={disabled || controller.saving}
            key={targetView.target.agentId}
            data-agent-id={targetView.target.agentId}
            onClick={() => onSelect(targetView.target.agentId, true)}
            onKeyDown={(event) => {
              if (event.defaultPrevented || isImeKeyboardEvent(event.nativeEvent) || event.key !== "ArrowRight") return;
              event.preventDefault();
              onSelect(targetView.target.agentId, true);
            }}
            onPointerEnter={(event) => {
              if (
                canHoverSelect
                && event.pointerType === "mouse"
                && !isActive
              ) {
                onSelect(targetView.target.agentId);
              }
            }}
          >
            <UiAgentAvatar
              avatar={targetView.target.avatar}
              name={targetView.target.name}
              size="sm"
            />
            <span className="min-w-0 flex-1 truncate text-compact font-medium text-(--text-strong)">
              {targetView.target.name}
            </span>
            <span className="max-w-24 truncate text-2xs font-normal text-(--text-soft)">
              {targetView.modelLabel}
            </span>
            {targetView.busy ? (
              <LoaderCircle
                className={getUiSpinnerClassName({ size: "sm", tone: "muted" })}
              />
            ) : (
              <ChevronRight className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
            )}
          </UiMenuActionRow>
        );
      })}
    </div>
  );
}

function RoomModelOptions({
  controller,
  disabled,
  items,
  menuId,
  menuRef,
  onSelect,
  resetItem,
  style,
}: {
  controller: ComposerSessionSettingsController;
  disabled: boolean;
  items: UiActionMenuItem[];
  menuId: string;
  menuRef: RefObject<HTMLDivElement | null>;
  onSelect: (value: string) => void;
  resetItem: UiActionMenuItem;
  style: CSSProperties;
}) {
  const { t } = useI18n();
  return (
    <div
      aria-label={t("composer.room_model_agent", { name: controller.target?.name ?? "" })}
      className={cn(
        "soft-scrollbar min-h-0 shrink-0 overflow-y-auto overscroll-contain p-1",
        OVERLAY_SURFACE_CLASS_NAME,
      )}
      role="menu"
      id={menuId}
      ref={menuRef}
      tabIndex={-1}
      style={{ ...style, width: ROOM_MODEL_MENU_WIDTH }}
    >
      <UiActionMenuContent
        density="compact"
        disabled={disabled || controller.modelBusy}
        footerItems={[resetItem]}
        items={items}
        onSelect={onSelect}
      />
    </div>
  );
}

function RoomModelHeader({
  onBack,
  title,
}: {
  onBack: () => void;
  title: string;
}) {
  const { t } = useI18n();
  return (
    <div className="flex h-10 shrink-0 items-center gap-2 border-b border-(--divider-subtle-color) px-2">
      <UiIconButton
        aria-label={t("composer.session_settings_back")}
        className="shrink-0"
        onClick={onBack}
        size="sm"
        tooltip={t("composer.session_settings_back")}
        variant="ghost"
      >
        <ArrowLeft className="h-3.5 w-3.5" />
      </UiIconButton>
      <span className="min-w-0 flex-1 truncate text-sm font-semibold text-(--text-strong)">
        {title}
      </span>
    </div>
  );
}

function estimateRoomModelMenuHeight({
  agentCount,
  modelCount,
}: {
  agentCount: number;
  modelCount: number;
}): number {
  const agentHeight = MENU_SURFACE_VERTICAL_PADDING_PX
    + agentCount * getMenuItemLayout().height
    + Math.max(0, agentCount - 1) * MENU_ITEM_GAP_PX;
  const modelItemCount = modelCount + 1;
  const modelHeight = 17
    + modelItemCount * getMenuItemLayout({ density: "compact" }).height
    + modelItemCount * MENU_ITEM_GAP_PX;
  return Math.max(agentHeight, modelHeight);
}
