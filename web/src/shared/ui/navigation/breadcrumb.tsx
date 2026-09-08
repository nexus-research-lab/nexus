// INPUT: 用户可见的位置层级、可选返回/链接动作、前导身份与密度。
// OUTPUT: 具有导航语义、统一箭头、单行截断和共享 Button 状态的层级路径。
// POS: 全站 Breadcrumb DOM 与视觉 owner；不读取路由，也不解释业务路径。
"use client";

import { ChevronRight } from "lucide-react";
import type { ReactNode } from "react";

import { UiButton, UiLinkButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export type UiBreadcrumbDensity = "default" | "compact";

interface UiBreadcrumbItemBase {
  icon?: ReactNode;
  id: string;
  label: ReactNode;
  title?: string;
}

export type UiBreadcrumbItem = UiBreadcrumbItemBase & (
  | { href: string; onSelect?: never }
  | { href?: never; onSelect: () => void }
  | { href?: never; onSelect?: never }
);

interface UiBreadcrumbProps {
  ariaLabel: string;
  className?: string;
  density?: UiBreadcrumbDensity;
  items: readonly UiBreadcrumbItem[];
  leading?: ReactNode;
}

function UiBreadcrumbSeparator() {
  return (
    <li aria-hidden="true" className="shrink-0" data-slot="breadcrumb-separator">
      <ChevronRight className="h-3 w-3 text-(--icon-muted)" />
    </li>
  );
}

function UiBreadcrumbItemContent({
  current,
  density,
  item,
}: {
  current: boolean;
  density: UiBreadcrumbDensity;
  item: UiBreadcrumbItem;
}) {
  const icon = item.icon ? (
    <span className="shrink-0 [&_svg]:h-3.5 [&_svg]:w-3.5" data-slot="breadcrumb-icon">
      {item.icon}
    </span>
  ) : null;
  const label = <span className="min-w-0 truncate">{item.label}</span>;
  const actionClassName = "min-w-0 max-w-full whitespace-nowrap";

  if (item.href) {
    return (
      <UiLinkButton
        className={actionClassName}
        href={item.href}
        size={density === "compact" ? "2xs" : "sm"}
        title={item.title}
        variant="text"
      >
        {icon}
        {label}
      </UiLinkButton>
    );
  }
  if (item.onSelect) {
    return (
      <UiButton
        className={actionClassName}
        onClick={item.onSelect}
        size={density === "compact" ? "2xs" : "sm"}
        title={item.title}
        variant="text"
      >
        {icon}
        {label}
      </UiButton>
    );
  }
  return (
    <>
      {icon}
      <span
        aria-current={current ? "page" : undefined}
        className={cn(
          "min-w-0 max-w-full truncate whitespace-nowrap",
          getUiTypographyClassName({
            role: "metadata",
            tone: current ? "strong" : "soft",
            weight: current ? "medium" : "regular",
          }),
        )}
        title={item.title}
      >
        {item.label}
      </span>
    </>
  );
}

export function UiBreadcrumb({
  ariaLabel,
  className,
  density = "default",
  items,
  leading,
}: UiBreadcrumbProps) {
  return (
    <nav
      aria-label={ariaLabel}
      className={cn("flex min-w-0 items-center", className)}
      data-density={density}
      data-slot="breadcrumb"
    >
      <ol className="flex min-w-0 flex-1 items-center gap-1.5 overflow-hidden">
        {leading ? (
          <li className="inline-flex shrink-0 items-center" data-slot="breadcrumb-leading">
            {leading}
          </li>
        ) : null}
        {items.map((item, index) => {
          const current = index === items.length - 1 && !item.href && !item.onSelect;
          return (
            <FragmentWithSeparator
              current={current}
              density={density}
              item={item}
              key={item.id}
              separated={Boolean(leading) || index > 0}
            />
          );
        })}
      </ol>
    </nav>
  );
}

function FragmentWithSeparator({
  current,
  density,
  item,
  separated,
}: {
  current: boolean;
  density: UiBreadcrumbDensity;
  item: UiBreadcrumbItem;
  separated: boolean;
}) {
  return (
    <>
      {separated ? <UiBreadcrumbSeparator /> : null}
      <li
        className={cn(
          "inline-flex min-w-0 items-center gap-1.5",
          current ? "basis-2/5 grow" : "shrink",
        )}
        data-current={current || undefined}
        data-slot="breadcrumb-item"
      >
        <UiBreadcrumbItemContent current={current} density={density} item={item} />
      </li>
    </>
  );
}
