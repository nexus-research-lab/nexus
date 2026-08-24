/**
 * INPUT: 能力页面标题、可选说明、页面动作、筛选控件、分区与目录条目。
 * OUTPUT: 能力管理页的共享内容轴、用途说明、移动页头动作投影和响应式网格。
 * POS: 能力域页面级设计语法；通过应用布局动作槽适配手机页头，不解释具体领域状态。
 */
"use client";

import {
  type CompositionEventHandler,
  type KeyboardEventHandler,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";

import { useMobileAppPageHeaderActionsTarget } from "@/app/layout/mobile-app-page-header-actions-context";
import { cn } from "@/shared/ui/class-name";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import {
  WORKSPACE_CATALOG_GRID_CLASS_NAME,
  WORKSPACE_CONTENT_PAGE_CLASS_NAME,
} from "@/shared/ui/layout/workspace-content-layout";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { UiSelectMenuOption } from "@/shared/ui/menu/select-menu-model";

interface CapabilityPageLayoutProps {
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  description?: ReactNode;
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

interface CapabilityItemIconProps {
  children: ReactNode;
  className?: string;
  size?: "sm" | "md";
}

const CAPABILITY_ITEM_ICON_SIZE_CLASS_NAMES = {
  md: "h-9 w-9 rounded-[8px]",
  sm: "h-8 w-8 rounded-[8px]",
} as const;

/** 普通能力目录统一使用紧凑三列，避免各子域维护不同横纵间距。 */
export const CAPABILITY_DIRECTORY_GRID_CLASS_NAME =
  `${WORKSPACE_CATALOG_GRID_CLASS_NAME} gap-2.5`;

/** 目录条目保留清晰外框，让不同能力类型共享同一内容层级。 */
export const CAPABILITY_DIRECTORY_ROW_CLASS_NAME =
  "min-h-[80px] border-(--divider-subtle-color) bg-transparent px-3 py-3 hover:border-(--surface-interactive-hover-border)";

/** 能力目录复用共享管理内容轴，标题、工具和内容始终保持同一基线。 */
export function CapabilityPageLayout({
  actions,
  children,
  className: className,
  description,
  headerAnchor,
  title,
}: CapabilityPageLayoutProps) {
  const mobileHeaderActionsTarget = useMobileAppPageHeaderActionsTarget();
  const mobileActions = mobileHeaderActionsTarget && actions
    ? createPortal(
        <div
          className="flex items-center"
          data-tour-anchor={headerAnchor}
        >
          {actions}
        </div>,
        mobileHeaderActionsTarget,
      )
    : null;

  return (
    <>
      {mobileActions}
      <div className={cn(WORKSPACE_CONTENT_PAGE_CLASS_NAME, className)}>
        <WorkspaceContentHeader
          actions={!mobileHeaderActionsTarget && actions
            ? <div className="flex w-full justify-end">{actions}</div>
            : undefined}
          className="max-sm:hidden"
          description={description}
          headerAnchor={headerAnchor}
          title={title}
        />
        {description ? (
          <p className="mb-5 text-compact leading-5 text-(--text-muted) sm:hidden">
            {description}
          </p>
        ) : null}
        {children}
      </div>
    </>
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

/** 为没有品牌资源的能力条目提供统一的方形身份图标框。 */
export function CapabilityItemIcon({
  children,
  className,
  size = "md",
}: CapabilityItemIconProps) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "flex shrink-0 items-center justify-center border border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--icon-default)",
        CAPABILITY_ITEM_ICON_SIZE_CLASS_NAMES[size],
        className,
      )}
    >
      {children}
    </span>
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
