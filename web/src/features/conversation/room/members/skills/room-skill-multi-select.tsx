// INPUT: Room Skill 目录、已选值、查询状态、禁用态与集合更新命令。
// OUTPUT: 打开菜单和移除实体互不嵌套的可搜索多选字段。
// POS: Room Skill 领域多选组合；目录请求和 Room 草稿由上层持有。

"use client";

import {
  type ComponentType,
  type CSSProperties,
  type RefObject,
  useCallback,
  useMemo,
} from "react";
import { createPortal } from "react-dom";
import { Check, Loader2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { UiRemovableChip } from "@/shared/ui/form/removable-chip";
import {
  MENU_LIST_CLASS_NAME,
} from "@/shared/ui/menu/menu-styles";
import {
  estimateSelectMenuHeight,
  getSelectMenuButtonClassName,
  getSelectMenuOptionStateClassName,
  getSelectMenuSizeConfig,
  resolveSelectMenuPosition,
  SELECT_MENU_SEARCH_ROW_HEIGHT,
} from "@/shared/ui/menu/select-menu-model";
import {
  SelectMenuOptionRow,
  SelectMenuPanel,
  SelectMenuTriggerContent,
} from "@/shared/ui/menu/select-menu-primitives";
import { useSelectMenuOverlay } from "@/shared/ui/menu/use-select-menu-overlay";
import type { UiAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-model";

import {
  buildRoomSkillMenuBody,
  buildSelectedRoomSkills,
  removeRoomSkill,
  toggleRoomSkill,
  type RoomSkillMenuBodyKind,
  type RoomSkillMenuBodyPresentation,
  type RoomSkillOption,
} from "./room-skill-multi-select-model";

interface RoomSkillMultiSelectProps {
  ariaLabel: string;
  disabled: boolean;
  emptyText: string;
  errorText: string | null;
  isLoading: boolean;
  loadingText: string;
  onChange: (value: string[]) => void;
  onQueryChange: (value: string) => void;
  options: RoomSkillOption[];
  placeholder: string;
  query: string;
  searchPlaceholder: string;
  value: string[];
}

interface MenuBodyViewProps {
  onToggle: (value: string) => void;
  presentation: RoomSkillMenuBodyPresentation;
  selectedValues: ReadonlySet<string>;
}

interface RoomSkillMenuPortalProps extends MenuBodyViewProps {
  ariaLabel: string;
  isOpen: boolean;
  menuId: string;
  menuRef: RefObject<HTMLDivElement | null>;
  menuStyle: CSSProperties;
  onQueryChange: (value: string) => void;
  placement?: UiAnchoredOverlayPosition["placement"];
  portalContainer: Element | null;
  query: string;
  searchPlaceholder: string;
}

interface TriggerSelectionStyle {
  buttonClassName?: string;
  rootClassName: string;
}

const TRIGGER_SELECTION_STYLES: Record<
  "empty" | "selected",
  TriggerSelectionStyle
> = {
  empty: { rootClassName: "h-10" },
  selected: {
    buttonClassName: "min-h-10 py-1.5",
    rootClassName: "min-h-10",
  },
};

function LoadingMenuBody({ presentation }: MenuBodyViewProps) {
  return (
    <div className="flex min-h-10 items-center gap-2 px-2.5 text-sm text-(--text-muted)">
      <Loader2 className={getUiSpinnerClassName({ size: "md", tone: "muted" })} />
      {presentation.message}
    </div>
  );
}

function ErrorMenuBody({ presentation }: MenuBodyViewProps) {
  return (
    <UiInlineNotice
      className="m-1 w-auto"
      message={presentation.message}
      role="alert"
      tone="danger"
    />
  );
}

function EmptyMenuBody({ presentation }: MenuBodyViewProps) {
  return (
    <div className="flex min-h-10 items-center px-2.5 text-sm text-(--text-muted)">
      {presentation.message}
    </div>
  );
}

function RoomSkillOptionRow({
  isActive,
  onToggle,
  option,
}: {
  isActive: boolean;
  onToggle: (value: string) => void;
  option: RoomSkillOption;
}) {
  return (
    <SelectMenuOptionRow
      active={isActive}
      className={cn(
        "flex items-center gap-2 px-2.5 py-2 text-sm",
        getSelectMenuOptionStateClassName("dialog", isActive),
      )}
      onClick={() => onToggle(option.value)}
    >
      <span className="min-w-0 flex-1">
        <span className="block truncate">{option.label}</span>
        <span className="mt-0.5 block truncate text-xs font-normal text-(--text-muted)">
          {option.description}
        </span>
      </span>
      <span className="flex h-4 w-4 shrink-0 items-center justify-center text-(--primary)">
        {isActive ? <Check className="h-3.5 w-3.5" /> : null}
      </span>
    </SelectMenuOptionRow>
  );
}

function OptionsMenuBody({
  onToggle,
  presentation,
  selectedValues,
}: MenuBodyViewProps) {
  return (
    <>
      {presentation.options.map((option) => (
        <RoomSkillOptionRow
          isActive={selectedValues.has(option.value)}
          key={option.value}
          onToggle={onToggle}
          option={option}
        />
      ))}
    </>
  );
}

const MENU_BODY_VIEWS: Record<
  RoomSkillMenuBodyKind,
  ComponentType<MenuBodyViewProps>
> = {
  empty: EmptyMenuBody,
  error: ErrorMenuBody,
  loading: LoadingMenuBody,
  options: OptionsMenuBody,
};

function RoomSkillMenuBody(props: MenuBodyViewProps) {
  const Body = MENU_BODY_VIEWS[props.presentation.kind];
  return <Body {...props} />;
}

function SelectedSkillChips({
  disabled,
  onRemove,
  options,
  placeholder,
}: {
  disabled: boolean;
  onRemove: (value: string) => void;
  options: RoomSkillOption[];
  placeholder: string;
}) {
  if (options.length === 0) {
    return (
      <span className="truncate font-semibold text-(--text-muted)">
        {placeholder}
      </span>
    );
  }
  return (
    <>
      {options.map((option) => (
        <UiRemovableChip
          className="pointer-events-none max-w-[11rem]"
          disabled={disabled}
          key={option.value}
          onRemove={() => onRemove(option.value)}
          removeLabel={`移除 ${option.label}`}
          size="xs"
        >
          {option.label}
        </UiRemovableChip>
      ))}
    </>
  );
}

function RoomSkillMenuPortal({
  ariaLabel,
  isOpen,
  menuId,
  menuRef,
  menuStyle,
  onQueryChange,
  onToggle,
  placement,
  portalContainer,
  presentation,
  query,
  searchPlaceholder,
  selectedValues,
}: RoomSkillMenuPortalProps) {
  if (!isOpen || !portalContainer) {
    return null;
  }
  return createPortal(
    <SelectMenuPanel
      ariaLabel={ariaLabel}
      id={menuId}
      layoutClassName="flex flex-col overflow-hidden"
      panelRef={menuRef}
      placement={placement}
      style={menuStyle}
      surface="dialog"
    >
      <UiSearchInput
        aria-label={searchPlaceholder}
        className="shrink-0"
        inputClassName="font-medium"
        onChange={onQueryChange}
        placeholder={searchPlaceholder}
        value={query}
        variant="menu"
      />
      <div className={cn(
        MENU_LIST_CLASS_NAME,
        "soft-scrollbar min-h-0 flex-1 overflow-y-auto p-1",
      )}>
        <RoomSkillMenuBody
          onToggle={onToggle}
          presentation={presentation}
          selectedValues={selectedValues}
        />
      </div>
    </SelectMenuPanel>,
    portalContainer,
  );
}

function triggerSelectionStyle(value: string[]): TriggerSelectionStyle {
  const state = value.length > 0 ? "selected" : "empty";
  return TRIGGER_SELECTION_STYLES[state];
}

export function RoomSkillMultiSelect({
  ariaLabel,
  disabled,
  emptyText,
  errorText,
  isLoading,
  loadingText,
  onChange,
  onQueryChange,
  options,
  placeholder,
  query,
  searchPlaceholder,
  value,
}: RoomSkillMultiSelectProps) {
  const selectedValues = useMemo(() => new Set(value), [value]);
  const selectedOptions = useMemo(
    () => buildSelectedRoomSkills(options, value),
    [options, value],
  );
  const menuBody = buildRoomSkillMenuBody({
    emptyText,
    errorText,
    isLoading,
    loadingText,
    options,
  });
  const { roundedClassName, textClassName } = getSelectMenuSizeConfig("md");
  const estimatePosition = useCallback((button: HTMLButtonElement) => (
    resolveSelectMenuPosition({
      button,
      estimatedHeight: estimateSelectMenuHeight(
        Math.max(options.length, 1),
        52,
        SELECT_MENU_SEARCH_ROW_HEIGHT + 8,
      ),
      estimatedOptionHeight: 52,
      placement: "top",
    })
  ), [options.length]);
  const overlay = useSelectMenuOverlay({ disabled, estimatePosition });
  const selectionStyle = triggerSelectionStyle(value);
  const controlledMenuId = overlay.isOpen ? overlay.menuId : undefined;

  const toggleValue = (nextValue: string) => {
    if (disabled) {
      return;
    }
    onChange(toggleRoomSkill(value, nextValue));
    overlay.updateMenuPosition();
  };
  const removeValue = (nextValue: string) => {
    onChange(removeRoomSkill(value, nextValue));
    overlay.updateMenuPosition();
  };

  return (
    <div
      className={cn("relative w-full", selectionStyle.rootClassName)}
    >
      <button
        aria-controls={controlledMenuId}
        aria-disabled={disabled}
        aria-expanded={overlay.isOpen}
        aria-haspopup="listbox"
        aria-label={ariaLabel}
        className={getSelectMenuButtonClassName({
          roundedClassName,
          surface: "dialog",
          textClassName,
          className: cn("absolute inset-0", selectionStyle.buttonClassName),
        })}
        disabled={disabled}
        onClick={overlay.toggleMenu}
        onKeyDown={overlay.handleTriggerKeyDown}
        ref={overlay.buttonRef}
        type="button"
      >
        <SelectMenuTriggerContent isOpen={overlay.isOpen}>
          <span className="sr-only">
            {selectedOptions.length > 0
              ? selectedOptions.map((option) => option.label).join(", ")
              : placeholder}
          </span>
        </SelectMenuTriggerContent>
      </button>
      <span className="pointer-events-none relative flex min-h-10 min-w-0 items-center py-1.5 pl-3 pr-10">
        <span className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
          <SelectedSkillChips
            disabled={disabled}
            onRemove={removeValue}
            options={selectedOptions}
            placeholder={placeholder}
          />
        </span>
      </span>
      <RoomSkillMenuPortal
        ariaLabel={ariaLabel}
        isOpen={overlay.isOpen}
        menuId={overlay.menuId}
        menuRef={overlay.menuRef}
        menuStyle={overlay.menuStyle}
        onQueryChange={onQueryChange}
        onToggle={toggleValue}
        placement={overlay.menuPosition?.placement}
        portalContainer={overlay.portalContainer}
        presentation={menuBody}
        query={query}
        searchPlaceholder={searchPlaceholder}
        selectedValues={selectedValues}
      />
    </div>
  );
}
