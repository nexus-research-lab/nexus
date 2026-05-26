"use client";

import { type ReactNode } from "react";

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
  on_change: (value: string) => void;
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
  class_name,
  description,
  title,
}: CapabilityPageLayoutProps) {
  return (
    <div className={cn(WORKSPACE_DETAIL_PAGE_CLASS_NAME, class_name)}>
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
  on_change,
  placeholder,
  value,
}: CapabilityFilterSearchInputProps) {
  return (
    <UiSearchInput
      class_name="h-10 min-w-0 flex-1 rounded-[13px] border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_92%,white)] px-3.5"
      input_class_name="text-[14px]"
      on_change={on_change}
      placeholder={placeholder}
      value={value}
    />
  );
}

export function CapabilityFilterSelect({
  aria_label,
  class_name,
  disabled,
  label,
  leading,
  on_change,
  options,
  placeholder,
  tour_anchor,
  value,
}: CapabilityFilterSelectProps) {
  return (
    <div
      className={cn("shrink-0 sm:w-[184px]", class_name)}
      data-tour-anchor={tour_anchor}
    >
      <UiSelectMenu
        aria_label={aria_label}
        disabled={disabled}
        label={label}
        leading={leading}
        on_change={on_change}
        options={options}
        placeholder={placeholder}
        value={value}
      />
    </div>
  );
}

export function CapabilityFilterBar({
  children,
  class_name,
}: CapabilityFilterBarProps) {
  return (
    <div className={cn("mb-5 flex w-full flex-col gap-2.5 sm:flex-row sm:items-center", class_name)}>
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
