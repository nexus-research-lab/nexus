"use client";

import {
  type CompositionEventHandler,
  type KeyboardEventHandler,
  type ReactNode,
} from "react";

import { cn } from "@/shared/ui/class-name";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { UiSelectMenuOption } from "@/shared/ui/menu/select-menu-model";

interface CapabilityPageLayoutProps {
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  description: ReactNode;
  headerAnchor?: string;
  title: ReactNode;
}

interface CapabilityFilterBarProps {
  children: ReactNode;
  className?: string;
}

interface CapabilitySectionHeaderProps {
  count?: ReactNode;
  description?: ReactNode;
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

/** 能力目录复用共享管理内容轴，标题、工具和内容始终保持同一基线。 */
export function CapabilityPageLayout({
  actions,
  children,
  className: className,
  description,
  headerAnchor,
  title,
}: CapabilityPageLayoutProps) {
  return (
    <div className={cn(WORKSPACE_CONTENT_PAGE_CLASS_NAME, className)}>
      <WorkspaceContentHeader
        actions={actions}
        description={description}
        headerAnchor={headerAnchor}
        title={title}
      />
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
    <div
      className={cn(
        "mb-4 flex w-full flex-col gap-2 sm:flex-row sm:items-center",
        className,
      )}
    >
      {children}
    </div>
  );
}

export function CapabilitySectionHeader({
  count,
  description,
  title,
}: CapabilitySectionHeaderProps) {
  return (
    <div className="mb-2 flex items-end justify-between gap-4 border-b border-(--divider-subtle-color) pb-1.5">
      <div className="min-w-0">
        <h2 className="truncate text-base font-medium tracking-[-0.01em] text-(--text-strong)">
          {title}
        </h2>
        {description ? (
          <p className="mt-0.5 truncate text-compact text-(--text-muted)">
            {description}
          </p>
        ) : null}
      </div>
      {count !== undefined && count !== null ? (
        <span className="text-xs font-medium text-(--text-soft)">
          {count}
        </span>
      ) : null}
    </div>
  );
}
