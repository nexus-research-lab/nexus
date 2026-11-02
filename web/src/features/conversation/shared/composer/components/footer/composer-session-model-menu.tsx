"use client";

/**
 * INPUT: 当前 Session 模型/强度投影、所在视图、强度侧栏状态与选择命令。
 * OUTPUT: 按 Provider 分组的模型列表 + 常驻强度入口/重置；强度面板既可作带返回栏的原位子页，也可作并排侧栏。
 * POS: DM 直接模型浮层与 Room 模型面板共用的菜单正文；浮层定位、开关与焦点归调用方。
 */

import { ArrowLeft, Brain, ChevronRight } from "lucide-react";
import type { ReactNode, RefObject } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiActionMenuContent } from "@/shared/ui/menu/action-menu";
import { UiMenuActionRow } from "@/shared/ui/menu/menu-action-row";
import { MENU_LIST_CLASS_NAME, MENU_SEPARATOR_CLASS_NAME } from "@/shared/ui/menu/menu-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import type {
  ComposerSessionSettingsController,
} from "../../controller/use-composer-session-settings";
import {
  buildResetSessionSettingItem,
  buildSessionEffortItems,
  buildSessionModelGroups,
  sessionEffortLabel,
} from "./composer-session-control-options";
import {
  SESSION_MODEL_GROUP_LABEL_LAYOUT,
  SESSION_MODEL_HEADER_LAYOUT,
} from "./composer-session-control-layout";

export type SessionModelMenuView = "models" | "effort";

/** 强度面板并排展示时由调用方提供：入口悬浮即展开，悬浮其他行收起。 */
export interface SessionEffortSideState {
  expanded: boolean;
  menuId: string;
  onHoverChange: (expanded: boolean) => void;
}

const MENU_SCROLL_CLASS_NAME = "soft-scrollbar min-h-0 overflow-y-auto overscroll-contain p-1";
const MUTED_METADATA_CLASS_NAME = getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "regular" });

export function SessionModelMenu({
  ariaLabel,
  controller,
  disabled,
  effortSide,
  menuId,
  menuRef,
  modelsHeader,
  onSelectEffort,
  onSelectModel,
  onViewChange,
  view,
}: {
  ariaLabel: string;
  controller: ComposerSessionSettingsController;
  disabled: boolean;
  /** 有值时强度在侧栏展开，本组件只渲染模型页。 */
  effortSide?: SessionEffortSideState;
  menuId?: string;
  menuRef: RefObject<HTMLDivElement | null>;
  /** Room 窄屏的 Agent 返回栏；强度子页使用自己的返回栏。 */
  modelsHeader?: ReactNode;
  onSelectEffort: (value: string) => void;
  onSelectModel: (value: string) => void;
  onViewChange: (view: SessionModelMenuView) => void;
  view: SessionModelMenuView;
}) {
  const { t } = useI18n();
  const busy = disabled || controller.modelBusy;
  if (view === "effort" && !effortSide) {
    return (
      <SessionEffortMenu
        controller={controller}
        disabled={disabled}
        header={(
          <SessionModelMenuHeader
            onBack={() => onViewChange("models")}
            title={t("composer.session_effort")}
          />
        )}
        menuId={menuId}
        menuRef={menuRef}
        onSelect={onSelectEffort}
      />
    );
  }
  const groups = buildSessionModelGroups(controller, t);
  return (
    <>
      {modelsHeader}
      <div
        aria-label={ariaLabel}
        className={MENU_SCROLL_CLASS_NAME}
        id={menuId}
        onPointerOver={(event) => {
          if (!effortSide?.expanded || event.pointerType !== "mouse" || !(event.target instanceof Element)) return;
          const row = event.target.closest('[role^="menuitem"]');
          if (row && !row.hasAttribute("data-session-effort-entry")) effortSide.onHoverChange(false);
        }}
        ref={menuRef}
        role="menu"
        tabIndex={-1}
      >
        {groups.map((group) => (
          <div key={group.key} role="none">
            {group.label ? (
              <div
                className={cn(SESSION_MODEL_GROUP_LABEL_LAYOUT.className, MUTED_METADATA_CLASS_NAME)}
                role="presentation"
              >
                <span className="truncate">{group.label}</span>
              </div>
            ) : null}
            <UiActionMenuContent
              density="dense"
              disabled={busy}
              items={group.items}
              onSelect={onSelectModel}
            />
          </div>
        ))}
        {groups.length > 0 ? <div className={MENU_SEPARATOR_CLASS_NAME} role="separator" /> : null}
        <div className={MENU_LIST_CLASS_NAME} role="none">
          <SessionEffortEntryRow
            busy={busy}
            controller={controller}
            effortSide={effortSide}
            onOpen={() => onViewChange("effort")}
          />
        </div>
        <UiActionMenuContent
          density="dense"
          disabled={busy}
          items={[buildResetSessionSettingItem(!controller.hasModelOverride, t)]}
          onSelect={onSelectModel}
        />
      </div>
    </>
  );
}

// 入口常驻：当前模型没有开放档位时置灰并说明，用户不会误以为功能缺失。
function SessionEffortEntryRow({
  busy,
  controller,
  effortSide,
  onOpen,
}: {
  busy: boolean;
  controller: ComposerSessionSettingsController;
  effortSide?: SessionEffortSideState;
  onOpen: () => void;
}) {
  const { t } = useI18n();
  const supported = (controller.effortLevels ?? []).length > 0;
  const expanded = Boolean(effortSide?.expanded);
  return (
    <UiMenuActionRow
      active={expanded}
      aria-controls={expanded ? effortSide?.menuId : undefined}
      aria-expanded={supported ? expanded : undefined}
      aria-haspopup={supported ? "menu" : undefined}
      contentSized
      data-session-effort-entry=""
      density="dense"
      disabled={busy || !supported}
      onClick={onOpen}
      onKeyDown={(event) => {
        // 侧栏在左侧时左方向键也能进入；原位子页里左方向键保留给上一级返回。
        const opens = event.key === "ArrowRight" || (event.key === "ArrowLeft" && Boolean(effortSide));
        if (event.defaultPrevented || isImeKeyboardEvent(event.nativeEvent) || !opens) return;
        event.preventDefault();
        event.stopPropagation();
        onOpen();
      }}
      onPointerEnter={(event) => {
        if (effortSide && supported && !busy && event.pointerType === "mouse" && !expanded) {
          effortSide.onHoverChange(true);
        }
      }}
    >
      <span className="flex min-w-0 flex-1 items-center gap-2">
        <span aria-hidden="true" className="flex h-4 w-4 shrink-0 items-center justify-center">
          <Brain className="h-3.5 w-3.5" />
        </span>
        <span className="min-w-0 flex-1 truncate">{t("composer.session_effort")}</span>
      </span>
      <span className="ml-2 flex shrink-0 items-center gap-1">
        <span className={MUTED_METADATA_CLASS_NAME}>
          {supported ? sessionEffortLabel(controller.effort, t) : t("composer.session_effort_unsupported")}
        </span>
        {supported ? <ChevronRight className="h-3.5 w-3.5 text-(--icon-muted)" /> : null}
      </span>
    </UiMenuActionRow>
  );
}

/** 强度档位面板：一句代价说明 + 自动/各档位；侧栏不带返回栏，原位子页由调用方传入 header。 */
export function SessionEffortMenu({
  controller,
  disabled,
  header,
  menuId,
  menuRef,
  onSelect,
}: {
  controller: ComposerSessionSettingsController;
  disabled: boolean;
  header?: ReactNode;
  menuId?: string;
  menuRef: RefObject<HTMLDivElement | null>;
  onSelect: (value: string) => void;
}) {
  const { t } = useI18n();
  return (
    <>
      {header}
      <div aria-label={t("composer.session_effort")} className={MENU_SCROLL_CLASS_NAME} id={menuId} ref={menuRef} role="menu" tabIndex={-1}>
        <UiActionMenuContent
          density="dense"
          disabled={disabled || controller.modelBusy}
          header={<p className={MUTED_METADATA_CLASS_NAME}>{t("composer.session_effort_description")}</p>}
          items={buildSessionEffortItems(controller, t)}
          onSelect={onSelect}
        />
      </div>
    </>
  );
}

export function SessionModelMenuHeader({
  onBack,
  title,
}: {
  onBack: () => void;
  title: string;
}) {
  const { t } = useI18n();
  return (
    <div className={SESSION_MODEL_HEADER_LAYOUT.className}>
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
