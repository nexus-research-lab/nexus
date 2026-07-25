"use client";

import {
  type CompositionEventHandler,
  type KeyboardEventHandler,
  type ReactNode,
} from "react";

import { cn } from "@/shared/ui/class-name";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { WORKSPACE_DETAIL_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { UiSelectMenuOption } from "@/shared/ui/menu/select-menu-model";

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

interface CapabilityPageHeaderProps {
  description: ReactNode;
  title: ReactNode;
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
      <CapabilityPageHeader description={description} title={title} />
      {children}
    </div>
  );
}

function CapabilityPageHeader({
  description,
  title,
}: CapabilityPageHeaderProps) {
  return (
    <header className="mb-5 border-b border-(--divider-subtle-color) pb-3">
      <h1 className="text-lg font-semibold tracking-[-0.02em] text-(--text-strong)">
        {title}
      </h1>
      <p className="mt-1 max-w-[680px] text-compact leading-5 text-(--text-muted)">
        {description}
      </p>
    </header>
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
      className="min-w-0 flex-1"
      controlSize="sm"
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
      className={cn("shrink-0 sm:w-[144px]", className)}
      data-tour-anchor={tourAnchor}
    >
      <UiSelectMenu
        ariaLabel={ariaLabel}
        buttonClassName="gap-1.5 px-2.5 shadow-none"
        className="h-8"
        disabled={disabled}
        label={label}
        leading={leading}
        onChange={onChange}
        options={options}
        placeholder={placeholder}
        size="sm"
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
    <div className={cn("mb-4 flex w-full flex-col gap-2 sm:flex-row sm:items-center", className)}>
      {children}
    </div>
  );
}

export function CapabilitySectionHeader({
  count,
  title,
}: CapabilitySectionHeaderProps) {
  return (
    <div className="mb-2 flex items-end justify-between border-b border-(--divider-subtle-color) pb-1.5">
      <h2 className="text-base font-medium tracking-[-0.01em] text-(--text-strong)">
        {title}
      </h2>
      {count ? (
        <span className="text-xs font-medium text-(--text-soft)">
          {count}
        </span>
      ) : null}
    </div>
  );
}
