"use client";

import {
  type CompositionEventHandler,
  type KeyboardEventHandler,
  type ReactNode,
} from "react";

import { cn } from "@/lib/utils";
import { UiSearchInput } from "@/shared/ui/form-control";
import { WORKSPACE_DETAIL_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { UiSelectMenu, type UiSelectMenuOption } from "@/shared/ui/menu/select-menu";

interface CapabilityPageLayoutProps {
  children: ReactNode;
  className?: string;
  description: ReactNode;
  title: ReactNode;
}

interface CapabilityFilterBarProps {
  children: ReactNode;
  className?: string;
}

interface CapabilitySectionHeaderProps {
  count?: ReactNode;
  title: ReactNode;
}

interface CapabilityFilterSearchInputProps {
  action?: ReactNode;
  onChange: (value: string) => void;
  onCompositionEnd?: CompositionEventHandler<HTMLInputElement>;
  onCompositionStart?: CompositionEventHandler<HTMLInputElement>;
  onKeyDown?: KeyboardEventHandler<HTMLInputElement>;
  placeholder: string;
  value: string;
}

interface CapabilityFilterSelectProps {
  ariaLabel: string;
  className?: string;
  disabled?: boolean;
  label?: ReactNode;
  leading?: ReactNode;
  onChange: (value: string) => void;
  options: UiSelectMenuOption[];
  placeholder?: string;
  tourAnchor?: string;
  value: string;
}

/** 中文注释：能力区目录页共用版心和介绍区，保持技能、连接器和其它入口节奏一致。 */
export function CapabilityPageLayout({
  children,
  className: className,
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
  onChange: onChange,
  onCompositionEnd: onCompositionEnd,
  onCompositionStart: onCompositionStart,
  onKeyDown: onKeyDown,
  placeholder,
  value,
}: CapabilityFilterSearchInputProps) {
  return (
    <UiSearchInput
      className="h-10 min-w-0 flex-1 rounded-[13px] border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_92%,white)] px-3.5"
      inputClassName="text-[14px]"
      action={action}
      onChange={onChange}
      onCompositionEnd={onCompositionEnd}
      onCompositionStart={onCompositionStart}
      onKeyDown={onKeyDown}
      placeholder={placeholder}
      value={value}
    />
  );
}

export function CapabilityFilterSelect({
  ariaLabel: ariaLabel,
  className: className,
  disabled,
  label,
  leading,
  onChange: onChange,
  options,
  placeholder,
  tourAnchor: tourAnchor,
  value,
}: CapabilityFilterSelectProps) {
  return (
    <div
      className={cn("shrink-0 sm:w-[184px]", className)}
      data-tour-anchor={tourAnchor}
    >
      <UiSelectMenu
        ariaLabel={ariaLabel}
        disabled={disabled}
        label={label}
        leading={leading}
        onChange={onChange}
        options={options}
        placeholder={placeholder}
        value={value}
      />
    </div>
  );
}

export function CapabilityFilterBar({
  children,
  className: className,
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
