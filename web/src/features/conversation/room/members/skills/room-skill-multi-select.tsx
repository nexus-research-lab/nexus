"use client";

import {
  type ComponentType,
  type CSSProperties,
  type RefObject,
  useCallback,
  useMemo,
} from "react";
import { createPortal } from "react-dom";
import { Check, Loader2, Search, X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import {
  estimateSelectMenuHeight,
  getSelectMenuButtonClassName,
  getSelectMenuOptionStateClassName,
  getSelectMenuSizeConfig,
  resolveSelectMenuPosition,
  SELECT_MENU_SEARCH_ROW_HEIGHT,
} from "@/shared/ui/menu/select-menu-model";
import {
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
    <div className="flex min-h-10 items-center gap-2 px-2.5 text-[13px] text-(--text-muted)">
      <Loader2 className="h-4 w-4 animate-spin" />
      {presentation.message}
    </div>
  );
}

function ErrorMenuBody({ presentation }: MenuBodyViewProps) {
  return (
    <div className="m-1 rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_18%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--destructive)_7%,transparent)] px-2.5 py-2 text-[13px] leading-5 text-(--destructive)">
      {presentation.message}
    </div>
  );
}

function EmptyMenuBody({ presentation }: MenuBodyViewProps) {
  return (
    <div className="flex min-h-10 items-center px-2.5 text-[13px] text-(--text-muted)">
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
    <button
      aria-selected={isActive}
      className={cn(
        "flex w-full items-center gap-2 rounded-[10px] px-2.5 py-2 text-left text-[13px] transition-[background-color,color] duration-(--motion-duration-fast)",
        getSelectMenuOptionStateClassName("dialog", isActive),
      )}
      data-active={isActive ? "true" : undefined}
      onClick={() => onToggle(option.value)}
      role="option"
      type="button"
    >
      <span className="min-w-0 flex-1">
        <span className="block truncate">{option.label}</span>
        <span className="mt-0.5 block truncate text-[11px] font-normal text-(--text-muted)">
          {option.description}
        </span>
      </span>
      <span className="flex h-4 w-4 shrink-0 items-center justify-center text-(--primary)">
        {isActive ? <Check className="h-3.5 w-3.5" /> : null}
      </span>
    </button>
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
  onRemove,
  options,
  placeholder,
}: {
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
        <span
          className="inline-flex max-w-[11rem] items-center gap-1 rounded-[6px] border border-(--divider-subtle-color) bg-transparent py-0.5 pl-2 pr-1 text-[11px] font-medium text-(--text-strong)"
          key={option.value}
        >
          <span className="min-w-0 truncate">{option.label}</span>
          <span
            aria-label={`移除 ${option.label}`}
            className="flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-(--icon-muted) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
            onClick={(event) => {
              event.stopPropagation();
              onRemove(option.value);
            }}
            onKeyDown={(event) => event.stopPropagation()}
            role="button"
            tabIndex={-1}
          >
            <X className="h-2.5 w-2.5" />
          </span>
        </span>
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
      <label className="flex h-11 items-center gap-2 border-b border-(--divider-subtle-color) px-3">
        <Search className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
        <input
          className="min-w-0 flex-1 bg-transparent text-[13px] font-medium text-(--text-strong) outline-none placeholder:text-(--text-soft)"
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder={searchPlaceholder}
          type="search"
          value={query}
        />
      </label>
      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto p-1">
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
  const openState = overlay.isOpen ? "true" : undefined;
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
      data-ui-select-menu-open={openState}
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
          className: selectionStyle.buttonClassName,
        })}
        disabled={disabled}
        onClick={overlay.toggleMenu}
        onKeyDown={overlay.handleTriggerKeyDown}
        ref={overlay.buttonRef}
        type="button"
      >
        <SelectMenuTriggerContent isOpen={overlay.isOpen}>
          <span className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
            <SelectedSkillChips
              onRemove={removeValue}
              options={selectedOptions}
              placeholder={placeholder}
            />
          </span>
        </SelectMenuTriggerContent>
      </button>
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
