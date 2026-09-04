// INPUT: 当前单选值、有限选项、显示密度与变更命令。
// OUTPUT: 可点击和键盘切换、选中后归还触发器焦点的共享 Select Menu。
// POS: 单选菜单 pattern；不支持业务搜索、异步资源或多选状态机。
"use client";

import {
  type KeyboardEvent,
  type ReactNode,
  useCallback,
} from "react";

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

interface UiSelectMenuProps {
  ariaLabel: string;
  allowLabelWrap?: boolean;
  buttonClassName?: string;
  className?: string;
  disabled?: boolean;
  id?: string;
  label?: ReactNode;
  leading?: ReactNode;
  menuMinWidth?: number;
  onChange: (value: string) => void;
  options: UiSelectMenuOption[];
  placement?: UiSelectMenuPlacement;
  placeholder?: string;
  size?: UiSelectMenuSize;
  surface?: UiSelectMenuSurface;
  value: string;
}

const SELECT_MENU_DEFAULT_PROPS = {
  allowLabelWrap: false,
  disabled: false,
  placement: "auto",
  placeholder: "请选择",
  size: "md",
  surface: "surface",
} as const satisfies Partial<UiSelectMenuProps>;

type SelectMenuDefaultProp = keyof typeof SELECT_MENU_DEFAULT_PROPS;
type ResolvedUiSelectMenuProps = Omit<UiSelectMenuProps, SelectMenuDefaultProp>
  & Required<Pick<UiSelectMenuProps, SelectMenuDefaultProp>>;

/** 共享自定义下拉菜单，避免业务侧重复实现原生 select 无法控制的弹层定位。 */
export function UiSelectMenu(props: UiSelectMenuProps) {
  const resolvedProps: ResolvedUiSelectMenuProps = {
    ...props,
    allowLabelWrap:
      props.allowLabelWrap ?? SELECT_MENU_DEFAULT_PROPS.allowLabelWrap,
    disabled: props.disabled ?? SELECT_MENU_DEFAULT_PROPS.disabled,
    placement: props.placement ?? SELECT_MENU_DEFAULT_PROPS.placement,
    placeholder: props.placeholder ?? SELECT_MENU_DEFAULT_PROPS.placeholder,
    size: props.size ?? SELECT_MENU_DEFAULT_PROPS.size,
    surface: props.surface ?? SELECT_MENU_DEFAULT_PROPS.surface,
  };
  return <UiSelectMenuController {...resolvedProps} />;
}

function UiSelectMenuController({
  ariaLabel,
  allowLabelWrap,
  buttonClassName,
  className,
  disabled,
  id,
  label,
  leading,
  menuMinWidth,
  onChange,
  options,
  placement,
  placeholder,
  size,
  surface,
  value,
}: ResolvedUiSelectMenuProps) {
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
  });

  const changeValue = useCallback((nextValue: string) => {
    if (disabled) {
      return;
    }
    onChange(nextValue);
    closeMenu();
    buttonRef.current?.focus();
  }, [buttonRef, closeMenu, disabled, onChange]);

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
    handleOverlayTriggerKeyDown(event, moveSelection);
  }, [handleOverlayTriggerKeyDown, moveSelection]);

  return (
    <SelectMenuView
      ariaLabel={ariaLabel}
      buttonClassName={buttonClassName}
      buttonRef={buttonRef}
      className={className}
      disabled={disabled}
      id={id}
      isOpen={isOpen}
      label={label}
      leading={leading}
      menuId={menuId}
      menuPlacement={menuPosition?.placement}
      menuRef={menuRef}
      menuStyle={menuStyle}
      onSelect={changeValue}
      onTriggerClick={toggleMenu}
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
