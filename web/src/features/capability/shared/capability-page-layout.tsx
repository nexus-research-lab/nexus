"use client";

import { type ReactNode } from "react";

import { cn } from "@/lib/utils";

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

/** 中文注释：能力区目录页共用版心和介绍区，保持技能、连接器和其它入口节奏一致。 */
export function CapabilityPageLayout({
  children,
  class_name,
  description,
  title,
}: CapabilityPageLayoutProps) {
  return (
    <div className={cn("mx-auto w-full max-w-[980px] px-5 py-6 xl:px-6", class_name)}>
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
