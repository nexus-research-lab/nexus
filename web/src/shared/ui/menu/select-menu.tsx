// INPUT: 当前单选值、有限选项、显示密度、选择上下文与变更命令。
// OUTPUT: 本地化单选字段、当前目录校验、受控方向键选值与显式打开后的焦点遍历/退出。
// POS: 单选菜单 pattern；不支持业务搜索、异步资源或多选状态机。
"use client";

import {
  type KeyboardEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useRef,
} from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";

import {
  buildSelectMenuModel,
  estimateSelectMenuHeight,
  resolveNextSelectMenuValue,
  resolveSelectMenuPosition,
  type UiSelectMenuOption,
  type UiSelectMenuPlacement,
  type UiSelectMenuSelectionDirection,
  type UiSelectMenuSize,
  type UiSelectMenuSurface,
} from "./select-menu-model";
import { getSelectMenuStyleProjection } from "./select-menu-styles";
import { SelectMenuView } from "./select-menu-view";
import { useSelectMenuOverlay } from "./use-select-menu-overlay";
import { focusSelectedListboxItem } from "./menu-keyboard";

interface UiSelectMenuProps {
  ariaLabel: string;
  allowLabelWrap?: boolean;
  className?: string;
  disabled?: boolean;
  id?: string;
  leading?: ReactNode;
  menuMinWidth?: number;
  onChange: (value: string) => void;
  onOpen?: () => void;
  options: UiSelectMenuOption[];
  placement?: UiSelectMenuPlacement;
  placeholder?: string;
  /** Changed selection context closes the menu while retaining the trigger DOM. */
  resetKey?: string;
  size?: UiSelectMenuSize;
  surface?: UiSelectMenuSurface;
  value: string;
}

/** 共享单选语义；真正的执行命令使用 Action Menu，不能借方向键选值执行。 */
export function UiSelectMenu({
  ariaLabel,
  allowLabelWrap = false,
  className,
  disabled: disabledProp = false,
  id,
  leading,
  menuMinWidth,
  onChange,
  onOpen,
  options,
  placement = "auto",
  placeholder: explicitPlaceholder,
  resetKey,
  size = "md",
  surface = "surface",
  value,
}: UiSelectMenuProps) {
  const { t } = useI18n();
  const placeholder = explicitPlaceholder ?? t("common.select_placeholder");
  const disabled = disabledProp || options.length === 0;
  const focusMenuOnOpenRef = useRef(false);
  const model = buildSelectMenuModel({
    options,
    placeholder,
    value,
  });
  const styles = getSelectMenuStyleProjection({ allowLabelWrap, size });

  const estimatePosition = useCallback((button: HTMLButtonElement) => {
    return resolveSelectMenuPosition({
      button,
      estimatedHeight: estimateSelectMenuHeight(
        options.length,
        styles.estimatedOptionHeight,
      ),
      estimatedOptionHeight: styles.estimatedOptionHeight,
      menuMinWidth,
      placement,
    });
  }, [menuMinWidth, options.length, placement, styles.estimatedOptionHeight]);

  const {
    buttonRef,
    closeMenu,
    handleTriggerKeyDown: handleOverlayTriggerKeyDown,
    isOpen,
    menuId,
    menuPosition,
    menuRef,
    menuStyle,
    portalContainer,
    toggleMenu,
  } = useSelectMenuOverlay({
    disabled,
    estimatePosition,
    resetKey,
  });

  const closeAndRestoreFocus = useCallback(() => {
    closeMenu();
    buttonRef.current?.focus();
  }, [buttonRef, closeMenu]);

  const changeValue = useCallback((nextValue: string) => {
    if (disabled || !options.some((option) => option.value === nextValue && !option.disabled)) return;
    onChange(nextValue);
    closeAndRestoreFocus();
  }, [closeAndRestoreFocus, disabled, onChange, options]);

  useEffect(() => {
    if (!isOpen) { focusMenuOnOpenRef.current = false; return; }
    if (!focusMenuOnOpenRef.current || !portalContainer || !menuPosition) return;
    focusMenuOnOpenRef.current = false;
    focusSelectedListboxItem(menuRef.current);
  }, [isOpen, menuPosition, menuRef, portalContainer]);

  const moveSelection = useCallback((direction: UiSelectMenuSelectionDirection): boolean => {
    if (disabled) {
      return false;
    }
    const nextValue = resolveNextSelectMenuValue({ direction, options, value });
    if (nextValue === null) {
      return false;
    }
    onChange(nextValue);
    return true;
  }, [disabled, onChange, options, value]);

  const onTriggerKeyDown = useCallback((event: KeyboardEvent<HTMLButtonElement>) => {
    if (disabled || event.defaultPrevented || isImeKeyboardEvent(event.nativeEvent)) return;
    if (event.key === "Tab") {
      closeMenu();
      return; // Trigger 仍在页面顺序中；让浏览器自然续接焦点。
    }
    focusMenuOnOpenRef.current = event.key === "Enter" || event.key === " ";
    handleOverlayTriggerKeyDown(event, moveSelection);
    // Only an accepted opening key may request privileged resources in this gesture.
    if (!isOpen && event.defaultPrevented) onOpen?.();
  }, [closeMenu, disabled, handleOverlayTriggerKeyDown, isOpen, moveSelection, onOpen]);

  return (
    <SelectMenuView
      ariaLabel={ariaLabel}
      buttonRef={buttonRef}
      className={className}
      disabled={disabled}
      id={id}
      isOpen={isOpen}
      leading={leading}
      menuId={menuId}
      menuPlacement={menuPosition?.placement}
      menuRef={menuRef}
      menuStyle={menuStyle}
      onSelect={changeValue}
      onTabExit={closeAndRestoreFocus}
      onTriggerClick={() => {
        if (!disabled && !isOpen) onOpen?.();
        focusMenuOnOpenRef.current = true;
        toggleMenu();
      }}
      onTriggerKeyDown={onTriggerKeyDown}
      options={options}
      portalContainer={portalContainer}
      model={model}
      surface={surface}
      styles={styles}
      value={value}
    />
  );
}
