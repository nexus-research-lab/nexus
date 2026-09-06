"use client";

/**
 * INPUT: Room 内各 Agent 的当前 Session 模型投影与更新动作。
 * OUTPUT: 同一模型列表在并排/逐级布局间保持焦点，尺寸服从公共视口边界，失效目标关闭。
 * POS: 群聊模型入口；保留用户选择的 Agent/Session 绑定，菜单与浮层合同归共享 owner。
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
import { useMediaQuery } from "@/shared/lib/react/use-media-query";
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
import { MENU_LIST_CLASS_NAME } from "@/shared/ui/menu/menu-styles";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import {
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

import {
  getRoomModelMenuLayout,
  ROOM_MODEL_AGENT_MENU_WIDTH,
  ROOM_MODEL_CASCADE_QUERY,
  ROOM_MODEL_HEADER_LAYOUT,
  ROOM_MODEL_MENU_GAP,
  SESSION_MODEL_MENU_WIDTH,
} from "./composer-session-control-layout";

interface ComposerRoomModelControlProps {
  controller: ComposerSessionSettingsController;
  disabled: boolean;
}

type RoomModelView = "agents" | "models";


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
  const boundSessionRef = useRef<string | null>(null);
  const lastFocusedRef = useRef<HTMLElement | null>(null);
  const canShowSideModels = useMediaQuery(ROOM_MODEL_CASCADE_QUERY);
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
    lastFocusedRef.current = null;
    boundSessionRef.current = null;
    setIsOpen(false);
    setView("agents");
    resetTarget();
  }, [resetTarget]);
  const showSideModels = canShowSideModels && view === "models";
  const menuLayout = getRoomModelMenuLayout({
    agentCount: controller.targetViews.length,
    modelCount: modelItems.length,
    modelsVisible: view === "models",
    sideBySide: showSideModels,
  });
  const estimatePosition = useCallback((anchor: HTMLButtonElement) => (
    resolveUiAnchoredOverlayPosition({
      align: "end",
      anchor,
      contentWidth: menuLayout.width,
      estimatedContentHeight: menuLayout.height,
      placement: "top",
      preset: "cascade-menu",
    })
  ), [menuLayout.height, menuLayout.width]);
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
  const panelStyle = { maxHeight: overlayStyle.maxHeight };

  useEffect(() => {
    if (!isOpen || !overlayPosition) return;
    const pendingView = pendingFocusRef.current;
    if (!pendingView) {
      // 模型列表在两种布局间保留 DOM；只有被移除的 Agent 行/返回按钮需要恢复焦点。
      if (lastFocusedRef.current && !lastFocusedRef.current.isConnected
        && document.activeElement === document.body && isTopAnchoredOverlay(overlayRef.current)) {
        focusFirstMenuItem(view === "models" ? modelMenuRef.current : agentMenuRef.current);
      }
      return;
    }
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
    if (!isOpen) return;
    if (disabled || controller.saving) {
      close();
      return;
    }
    if (view !== "models") return;
    const targetStillExists = controller.targetViews.some(({ target }) => (
      target.agentId === returnAgentIdRef.current && target.sessionKey === boundSessionRef.current
    ));
    if (!targetStillExists) {
      const hadMenuFocus = overlayRef.current?.contains(document.activeElement);
      close();
      if (hadMenuFocus) triggerRef.current?.focus();
    }
  }, [close, controller.saving, controller.targetViews, disabled, isOpen, overlayRef, view]);

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
    boundSessionRef.current = controller.targetViews.find(({ target }) => target.agentId === agentId)?.target.sessionKey ?? null;
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
    if (controller.target?.agentId !== returnAgentIdRef.current
      || controller.target?.sessionKey !== boundSessionRef.current) {
      closeAndRestoreFocus();
      return;
    }
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
            "fixed ui-layer-popover flex min-h-0",
            ANCHORED_OVERLAY_MOTION_CLASS_NAME,
            overlayPosition?.placement === "top"
              ? "items-end"
              : "items-start",
          )}
          data-placement={overlayPosition?.placement ?? "top"}
          id={overlayId}
          onFocusCapture={(event) => {
            if (event.target instanceof HTMLElement && event.currentTarget.contains(event.target)) {
              lastFocusedRef.current = event.target;
            }
          }}
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
          style={{ ...overlayStyle, columnGap: ROOM_MODEL_MENU_GAP }}
          {...OPEN_OVERLAY_DATA_ATTRIBUTES}
        >
          {(view === "agents" || showSideModels) && (
            <RoomModelPanel key="agents" style={panelStyle} width={showSideModels ? ROOM_MODEL_AGENT_MENU_WIDTH : "100%"}>
              <RoomModelAgentList
                activeAgentId={showSideModels ? controller.target?.agentId : undefined}
                canHoverSelect={canShowSideModels}
                controller={controller}
                disabled={disabled}
                menuRef={agentMenuRef}
                modelMenuId={`${overlayId}-models`}
                onSelect={selectAgent}
              />
            </RoomModelPanel>
          )}
          {view === "models" && (
            <RoomModelOptions
              key="models"
              controller={controller}
              disabled={disabled}
              items={modelItems}
              menuId={`${overlayId}-models`}
              menuRef={modelMenuRef}
              onBack={backToAgents}
              onSelect={selectModel}
              resetItem={resetItem}
              showHeader={!showSideModels}
              style={panelStyle}
              width={showSideModels ? SESSION_MODEL_MENU_WIDTH : "100%"}
            />
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
  width: CSSProperties["width"];
}) {
  return (
    <div
      className={cn(
        "flex min-h-0 min-w-0 shrink-0 flex-col overflow-hidden",
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

function RoomModelOptions({ controller, disabled, items, menuId, menuRef, onBack, onSelect, resetItem, showHeader, style, width }: {
  controller: ComposerSessionSettingsController;
  disabled: boolean;
  items: UiActionMenuItem[];
  menuId: string;
  menuRef: RefObject<HTMLDivElement | null>;
  onBack: () => void;
  onSelect: (value: string) => void;
  resetItem: UiActionMenuItem;
  showHeader: boolean;
  style: CSSProperties;
  width: CSSProperties["width"];
}) {
  const { t } = useI18n();
  return (
    <RoomModelPanel style={style} width={width}>
      {showHeader && <RoomModelHeader onBack={onBack} title={controller.target?.name ?? ""} />}
      <div
        aria-label={t("composer.room_model_agent", { name: controller.target?.name ?? "" })}
        className="soft-scrollbar min-h-0 overflow-y-auto overscroll-contain p-1"
        role="menu"
        id={menuId}
        ref={menuRef}
        tabIndex={-1}
      >
        <UiActionMenuContent
          density="compact"
          disabled={disabled || controller.modelBusy}
          footerItems={[resetItem]}
          items={items}
          onSelect={onSelect}
        />
      </div>
    </RoomModelPanel>
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
    <div className={ROOM_MODEL_HEADER_LAYOUT.className}>
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
