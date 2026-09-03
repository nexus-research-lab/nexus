"use client";

/**
 * INPUT: Room 内各 Agent 的当前 Session 模型投影与更新动作。
 * OUTPUT: Composer 右侧按 Agent 级联选择模型的紧凑浮层。
 * POS: 群聊模型入口；宽屏悬浮级联，窄屏点击逐级进入。
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
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import {
  UiActionMenuContent,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import {
  getMenuItemStateClassName,
  MENU_ITEM_BASE_CLASS_NAME,
  MENU_ITEM_GAP_PX,
  MENU_LIST_CLASS_NAME,
  MENU_SURFACE_VERTICAL_PADDING_PX,
} from "@/shared/ui/menu/menu-styles";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-model";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";
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

const ROOM_MODEL_AGENT_MENU_WIDTH = 224;
const ROOM_MODEL_MENU_WIDTH = 256;
const ROOM_MODEL_MENU_GAP = 8;
const ROOM_MODEL_MENU_MAX_HEIGHT = 320;
const ROOM_MODEL_MENU_MIN_HEIGHT = 32;
const ROOM_MODEL_MENU_VIEWPORT_MARGIN = 12;
const ROOM_MODEL_AGENT_ROW_HEIGHT = 36;
const ROOM_MODEL_ITEM_HEIGHT = 32;

export function ComposerRoomModelControl({
  controller,
  disabled,
}: ComposerRoomModelControlProps) {
  const { t } = useI18n();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [view, setView] = useState<RoomModelView>("agents");
  const resetTarget = controller.resetTarget;
  const modelItems = buildSessionModelItems(controller);
  const resetItem = buildResetSessionSettingItem(
    !controller.hasModelOverride,
    t,
  );
  const close = useCallback(() => {
    setIsOpen(false);
    setView("agents");
    resetTarget();
  }, [resetTarget]);
  const estimatePosition = useCallback((anchor: HTMLButtonElement) => (
    resolveAnchoredOverlayPosition({
      align: "end",
      anchor,
      estimatedHeight: estimateRoomModelMenuHeight({
        agentCount: controller.targetViews.length,
        modelCount: modelItems.length,
      }),
      maxHeight: ROOM_MODEL_MENU_MAX_HEIGHT,
      minHeight: ROOM_MODEL_MENU_MIN_HEIGHT,
      minWidth: ROOM_MODEL_AGENT_MENU_WIDTH,
      placement: "top",
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
  const canShowSideModels = typeof window !== "undefined"
    && window.innerWidth >= (
      ROOM_MODEL_AGENT_MENU_WIDTH
      + ROOM_MODEL_MENU_WIDTH
      + ROOM_MODEL_MENU_GAP
      + ROOM_MODEL_MENU_VIEWPORT_MARGIN * 2
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
          ROOM_MODEL_MENU_VIEWPORT_MARGIN,
          Math.min(
            showSideModels
              ? overlayPosition.left
              : overlayPosition.left
                + overlayPosition.width
                - layoutWidth,
            window.innerWidth
              - layoutWidth
              - ROOM_MODEL_MENU_VIEWPORT_MARGIN,
          ),
        ),
        width: layoutWidth,
      }
    : overlayStyle;
  const panelStyle = { maxHeight: overlayStyle.maxHeight };

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
    setIsOpen(true);
    void controller.ensureTargetsLoaded();
  };
  const selectAgent = (agentId: string) => {
    controller.selectTarget(agentId);
    setView("models");
  };
  const selectModel = (value: string) => {
    if (value === RESET_SESSION_SETTING_VALUE) {
      void controller.resetModel();
      close();
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
    close();
  };

  return (
    <>
      <UiButton
        ref={triggerRef}
        aria-controls={isOpen ? overlayId : undefined}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={t("composer.session_model")}
        className="h-7 min-w-0 max-w-44 px-1.5 text-(--text-default)"
        disabled={disabled || controller.saving}
        onClick={toggle}
        size="xs"
        title={t("composer.session_model")}
        variant="text"
      >
        <span className="truncate">
          {t("composer.room_model")}
        </span>
        <ChevronDown className="h-3 w-3 shrink-0" />
      </UiButton>

      {isOpen && portalContainer ? createPortal(
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
                  onSelect={selectAgent}
                />
              </RoomModelPanel>
              {showSideModels ? (
                <RoomModelOptions
                  controller={controller}
                  disabled={disabled}
                  items={modelItems}
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
                  onSelect={selectAgent}
                />
              ) : (
                <>
                  <RoomModelHeader
                    onBack={() => setView("agents")}
                    title={controller.target?.name ?? ""}
                  />
                  <div className="soft-scrollbar min-h-0 overflow-y-auto overscroll-contain p-1">
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
  onSelect,
}: {
  activeAgentId?: string;
  canHoverSelect?: boolean;
  controller: ComposerSessionSettingsController;
  disabled: boolean;
  onSelect: (agentId: string) => void;
}) {
  const { t } = useI18n();
  return (
    <div className={cn(
      MENU_LIST_CLASS_NAME,
      "soft-scrollbar min-h-0 overflow-y-auto overscroll-contain p-1",
    )}>
      {controller.targetViews.map((targetView) => {
        const isActive = targetView.target.agentId === activeAgentId;
        const select = () => onSelect(targetView.target.agentId);
        return (
          <button
            aria-label={t("composer.room_model_agent", {
              name: targetView.target.name,
            })}
            className={cn(
              MENU_ITEM_BASE_CLASS_NAME,
              "flex h-9 items-center gap-2 px-2",
              getMenuItemStateClassName({}),
              canHoverSelect
                && isActive
                && "bg-(--surface-interactive-active-background)",
            )}
            disabled={disabled || controller.saving}
            key={targetView.target.agentId}
            onClick={select}
            onPointerEnter={(event) => {
              if (
                canHoverSelect
                && event.pointerType === "mouse"
                && !isActive
              ) {
                select();
              }
            }}
            type="button"
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
          </button>
        );
      })}
    </div>
  );
}

function RoomModelOptions({
  controller,
  disabled,
  items,
  onSelect,
  resetItem,
  style,
}: {
  controller: ComposerSessionSettingsController;
  disabled: boolean;
  items: UiActionMenuItem[];
  onSelect: (value: string) => void;
  resetItem: UiActionMenuItem;
  style: CSSProperties;
}) {
  return (
    <div
      className={cn(
        "soft-scrollbar min-h-0 shrink-0 overflow-y-auto overscroll-contain p-1",
        OVERLAY_SURFACE_CLASS_NAME,
      )}
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
        className="shrink-0 text-(--icon-muted)"
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
    + agentCount * ROOM_MODEL_AGENT_ROW_HEIGHT
    + Math.max(0, agentCount - 1) * MENU_ITEM_GAP_PX;
  const modelItemCount = modelCount + 1;
  const modelHeight = 17
    + modelItemCount * ROOM_MODEL_ITEM_HEIGHT
    + modelItemCount * MENU_ITEM_GAP_PX;
  return Math.min(
    ROOM_MODEL_MENU_MAX_HEIGHT,
    Math.max(agentHeight, modelHeight),
  );
}
