/**
 * INPUT: 能力页面标题、说明、动作、筛选控件、目录条目及详情导航/正文/配置内容。
 * OUTPUT: 能力目录与详情页的共享内容轴、统一标签筛选、移动页头动作、二级导航、对象身份区、目录网格和响应式分栏。
 * POS: 能力域页面级设计语法；通过中立页头动作 Context 适配宿主挂载点，不依赖 App 装配或解释具体领域状态。
 */
"use client";

import {
  type CompositionEventHandler,
  type KeyboardEventHandler,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { ArrowLeft } from "lucide-react";

import { usePageHeaderActionsTarget } from "@/shared/lib/react/page-header-actions-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import {
  WORKSPACE_CATALOG_GRID_CLASS_NAME,
  WORKSPACE_CONTENT_PAGE_CLASS_NAME,
} from "@/shared/ui/layout/workspace-content-layout";
import {
  WorkspaceContentDetailHeader,
  WorkspaceContentHeader,
} from "@/shared/ui/layout/workspace-content-header";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { UiSelectMenuOption } from "@/shared/ui/menu/select-menu-model";
import { UiBreadcrumb } from "@/shared/ui/navigation/breadcrumb";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface CapabilityPageLayoutProps {
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  description?: ReactNode;
  headerAnchor?: string;
  title: ReactNode;
}

interface CapabilityDetailHeaderProps {
  backLabel: ReactNode;
  currentTitle?: ReactNode;
  onBack: () => void;
}

interface CapabilityDetailPageProps extends CapabilityDetailHeaderProps {
  children: ReactNode;
  className?: string;
}

interface CapabilityDetailIdentityProps {
  actions?: ReactNode;
  className?: string;
  description?: ReactNode;
  descriptionClassName?: string;
  descriptionRole?: "caption" | "code" | "supporting";
  descriptionTitle?: string;
  leading?: ReactNode;
  title: ReactNode;
  titleMeta?: ReactNode;
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

interface CapabilityDetailSplitLayoutProps {
  aside: ReactNode;
  children: ReactNode;
  className?: string;
  header?: ReactNode;
}

interface CapabilityDetailSectionHeaderProps {
  description?: ReactNode;
  meta?: ReactNode;
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
  label: string;
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
  md: "h-9 w-9 radius-control-sm",
  sm: "h-8 w-8 radius-control-sm",
} as const;

/** 普通能力目录统一使用紧凑三列，避免各子域维护不同横纵间距。 */
export const CAPABILITY_DIRECTORY_GRID_CLASS_NAME =
  `${WORKSPACE_CATALOG_GRID_CLASS_NAME} gap-2.5`;

/** 目录条目保留清晰外框，让不同能力类型共享同一内容层级。 */
export const CAPABILITY_DIRECTORY_ROW_CLASS_NAME =
  "min-h-[80px] border-(--divider-subtle-color) bg-transparent px-3 py-3 hover:border-(--surface-interactive-hover-border)";

/** 能力二级页统一使用“返回目录 / 当前对象”的单行桌面导航。 */
function CapabilityDetailHeader({
  backLabel,
  currentTitle,
  onBack,
}: CapabilityDetailHeaderProps) {
  const { t } = useI18n();
  return (
    <WorkspaceContentDetailHeader>
      <div
        className="min-w-0 flex-1"
        data-slot="capability-detail-header"
      >
        <UiBreadcrumb
          ariaLabel={t("common.location_aria")}
          items={[
            {
              icon: <ArrowLeft aria-hidden />,
              id: "directory",
              label: backLabel,
              onSelect: onBack,
            },
            ...(currentTitle ? [{ id: "current", label: currentTitle }] : []),
          ]}
        />
      </div>
    </WorkspaceContentDetailHeader>
  );
}

/** 能力详情页统一持有内容轴、顶部导航和导航后的正文起点，业务组件只提供对象内容。 */
export function CapabilityDetailPage({
  backLabel,
  children,
  className,
  currentTitle,
  onBack,
}: CapabilityDetailPageProps) {
  return (
    <div
      className={cn(WORKSPACE_CONTENT_PAGE_CLASS_NAME, className)}
      data-slot="capability-detail-page"
    >
      <CapabilityDetailHeader
        backLabel={backLabel}
        currentTitle={currentTitle}
        onBack={onBack}
      />
      <div
        className="flex min-h-0 flex-1 flex-col pt-5"
        data-slot="capability-detail-body"
      >
        {children}
      </div>
    </div>
  );
}

/** 详情对象的前导身份、标题、元数据、说明和操作共享同一响应式对齐规则。 */
export function CapabilityDetailIdentity({
  actions,
  className,
  description,
  descriptionClassName,
  descriptionRole = "supporting",
  descriptionTitle,
  leading,
  title,
  titleMeta,
}: CapabilityDetailIdentityProps) {
  return (
    <div
      className={cn(
        "flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between",
        className,
      )}
      data-slot="capability-detail-identity"
    >
      <div className="flex min-w-0 flex-1 items-start gap-4">
        {leading ? (
          <div className="shrink-0" data-slot="capability-detail-identity-leading">
            {leading}
          </div>
        ) : null}
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className={getUiTypographyClassName({ role: "objectTitle", tone: "strong" })}>
              {title}
            </h1>
            {titleMeta}
          </div>
          {description ? (
            <p
              className={cn(
                "mt-1",
                getUiTypographyClassName({ role: descriptionRole, tone: "muted" }),
                descriptionClassName,
              )}
              title={descriptionTitle}
            >
              {description}
            </p>
          ) : null}
        </div>
      </div>
      {actions ? (
        <div
          className="flex shrink-0 flex-wrap items-center justify-end gap-2"
          data-slot="capability-detail-identity-actions"
        >
          {actions}
        </div>
      ) : null}
    </div>
  );
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
  const mobileHeaderActionsTarget = usePageHeaderActionsTarget();
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
          <p className={cn(
            "mb-5 sm:hidden",
            getUiTypographyClassName({ role: "supporting", tone: "muted" }),
          )}>
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
  onChange,
  onCompositionEnd,
  onCompositionStart,
  onKeyDown,
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
  ariaLabel,
  className,
  disabled,
  label,
  onChange,
  options,
  placeholder,
  tourAnchor,
  value,
}: CapabilityFilterSelectProps) {
  return (
    <div
      className={cn("shrink-0 sm:w-[176px]", className)}
      data-tour-anchor={tourAnchor}
    >
      <UiSelectMenu
        ariaLabel={ariaLabel}
        disabled={disabled}
        label={label}
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
        <h2 className={cn(
          "truncate",
          getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
        )}>
          {title}
        </h2>
        {description ? (
          <p className={cn(
            "mt-0.5 truncate",
            getUiTypographyClassName({ role: "metadata", tone: "muted" }),
          )}>
            {description}
          </p>
        ) : null}
      </div>
      {count !== undefined && count !== null ? (
        <span className={getUiTypographyClassName({
          role: "caption",
          tone: "soft",
          weight: "medium",
        })}>
          {count}
        </span>
      ) : null}
    </div>
  );
}

/**
 * 详情页在宽工作面使用“可读正文 + 配置侧栏”，窄窗把配置放到长正文之前。
 * 业务页面只提供语义内容，不得自行复制断点、列宽或跨平台窗口公式。
 */
export function CapabilityDetailSplitLayout({
  aside,
  children,
  className,
  header,
}: CapabilityDetailSplitLayoutProps) {
  return (
    <div
      className={cn("w-full max-w-[1180px]", className)}
      data-slot="capability-detail-layout"
    >
      {header ? <div className="mb-6">{header}</div> : null}
      <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,760px)_minmax(280px,360px)] xl:gap-8">
        <aside
          className="min-w-0 xl:col-start-2 xl:row-start-1"
          data-slot="capability-detail-aside"
        >
          {aside}
        </aside>
        <div
          className="min-w-0 xl:col-start-1 xl:row-start-1"
          data-slot="capability-detail-main"
        >
          {children}
        </div>
      </div>
    </div>
  );
}

/** 详情正文与配置区共享标题、说明和右侧元数据节奏。 */
export function CapabilityDetailSectionHeader({
  description,
  meta,
  title,
}: CapabilityDetailSectionHeaderProps) {
  return (
    <div className="mb-3 flex items-end justify-between gap-3">
      <div className="min-w-0">
        <h2 className={getUiTypographyClassName({
          role: "sectionTitle",
          tone: "strong",
        })}>
          {title}
        </h2>
        {description ? (
          <p className={cn(
            "mt-1",
            getUiTypographyClassName({ role: "supporting", tone: "muted" }),
          )}>
            {description}
          </p>
        ) : null}
      </div>
      {meta ? (
        <span className={cn(
          "shrink-0",
          getUiTypographyClassName({ role: "caption", tone: "soft" }),
        )}>
          {meta}
        </span>
      ) : null}
    </div>
  );
}
