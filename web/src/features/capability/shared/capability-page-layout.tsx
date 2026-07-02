"use client";

import {
  type CompositionEventHandler,
  type KeyboardEventHandler,
  type ReactNode,
} from "react";

import { cn } from "@/lib/utils";
import { UiSearchInput } from "@/shared/ui/form-control";
import { WORKSPACE_DETAIL_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { UiSelectMenu, type UiSelectMenuOption } from "@/shared/ui/select-menu";

interface CapabilityPageLayoutProps {
  children: ReactNode;
  class_name?: string;
  description: ReactNode;
  title: ReactNode;
}

interface CapabilityFilterBarProps {
  children: ReactNode;
  class_name?: string;
}

interface CapabilitySectionHeaderProps {
  count?: ReactNode;
  title: ReactNode;
}

interface CapabilityFilterSearchInputProps {
  action?: ReactNode;
  on_change: (value: string) => void;
  on_composition_end?: CompositionEventHandler<HTMLInputElement>;
  on_composition_start?: CompositionEventHandler<HTMLInputElement>;
  on_key_down?: KeyboardEventHandler<HTMLInputElement>;
  placeholder: string;
  value: string;
}

interface CapabilityFilterSelectProps {
  aria_label: string;
  class_name?: string;
  disabled?: boolean;
  label?: ReactNode;
  leading?: ReactNode;
  on_change: (value: string) => void;
  options: UiSelectMenuOption[];
  placeholder?: string;
  tour_anchor?: string;
  value: string;
}

/** 中文注释：能力区目录页共用版心和介绍区，保持技能、连接器和其它入口节奏一致。 */
export function CapabilityPageLayout({
  children,
  class_name: className,
  description,
  title,
}: CapabilityPageLayoutProps) {
  return (
    <div className={cn(WORKSPACE_DETAIL_PAGE_CLASS_NAME, className)}>
      <div className="mb-5">
        <h1 className="text-[24px] font-semibold tracking-[-0.03em] text-(--text-strong)">
          {title}
        </h1>
        <p className="mt-1 max-w-[680px] text-[13px] leading-6 text-(--text-muted)">
          {description}
        </p>
      </div>
      {children}
    </div>
  );
}

export function CapabilityFilterSearchInput({
  action,
  on_change: onChange,
  on_composition_end: onCompositionEnd,
  on_composition_start: onCompositionStart,
  on_key_down: onKeyDown,
  placeholder,
  value,
}: CapabilityFilterSearchInputProps) {
  return (
    <UiSearchInput
      class_name="h-10 min-w-0 flex-1 rounded-[13px] border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_92%,white)] px-3.5"
      input_class_name="text-[14px]"
      action={action}
      on_change={onChange}
      onCompositionEnd={onCompositionEnd}
      onCompositionStart={onCompositionStart}
      onKeyDown={onKeyDown}
      placeholder={placeholder}
      value={value}
    />
  );
}

export function CapabilityFilterSelect({
  aria_label: ariaLabel,
  class_name: className,
  disabled,
  label,
  leading,
  on_change: onChange,
  options,
  placeholder,
  tour_anchor: tourAnchor,
  value,
}: CapabilityFilterSelectProps) {
  return (
    <div
      className={cn("shrink-0 sm:w-[184px]", className)}
      data-tour-anchor={tourAnchor}
    >
      <UiSelectMenu
        aria_label={ariaLabel}
        disabled={disabled}
        label={label}
        leading={leading}
        on_change={onChange}
        options={options}
        placeholder={placeholder}
        value={value}
      />
    </div>
  );
}

export function CapabilityFilterBar({
  children,
  class_name: className,
}: CapabilityFilterBarProps) {
  return (
    <div className={cn("mb-5 flex w-full flex-col gap-2.5 sm:flex-row sm:items-center", className)}>
      {children}
    </div>
  );
}

export function CapabilitySectionHeader({
  count,
  title,
}: CapabilitySectionHeaderProps) {
  return (
    <div className="mb-3 flex items-end justify-between border-b border-(--divider-subtle-color) pb-2">
      <h2 className="text-[18px] font-medium tracking-[-0.025em] text-(--text-strong)">
        {title}
      </h2>
      {count ? (
        <span className="text-[12px] font-medium text-(--text-soft)">
          {count}
        </span>
      ) : null}
    </div>
  );
}
