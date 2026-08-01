"use client";

import { type HTMLAttributes, useMemo } from "react";

import { getSeededAvatarAppearance } from "@/lib/seeded-avatar";
import { cn } from "@/shared/ui/class-name";

type UiSeededAvatarSize = "xs" | "sm" | "md" | "lg";

interface UiSeededAvatarProps extends HTMLAttributes<HTMLSpanElement> {
  seed: string;
  size?: UiSeededAvatarSize;
}

const SEEDED_AVATAR_SIZE_CLASS_NAME: Readonly<
  Record<UiSeededAvatarSize, string>
> = {
  xs: "h-8 w-8",
  sm: "h-9 w-9",
  md: "h-10 w-10",
  lg: "h-12 w-12",
};

const SEEDED_AVATAR_RADIUS_CLASS_NAME: Readonly<
  Record<UiSeededAvatarSize, string>
> = {
  xs: "rounded-[8px]",
  sm: "rounded-[9px]",
  md: "rounded-[10px]",
  lg: "rounded-[12px]",
};

/** 中文注释：所有数学曲线资源头像统一由此组件渲染，不向业务层泄漏 SVG 细节。 */
export function UiSeededAvatar({
  className,
  seed,
  size = "md",
  style,
  ...props
}: UiSeededAvatarProps) {
  const appearance = useMemo(
    () => getSeededAvatarAppearance(seed),
    [seed],
  );

  return (
    <span
      {...props}
      aria-hidden="true"
      className={cn(
        "flex shrink-0 items-center justify-center overflow-hidden border border-(--surface-avatar-border) shadow-(--surface-avatar-shadow)",
        SEEDED_AVATAR_SIZE_CLASS_NAME[size],
        SEEDED_AVATAR_RADIUS_CLASS_NAME[size],
        className,
      )}
      style={{
        backgroundColor: appearance.backgroundColor,
        color: appearance.foregroundColor,
        ...style,
      }}
    >
      <svg
        aria-hidden="true"
        className="block h-full w-full"
        fill="none"
        viewBox="0 0 100 100"
      >
        <path
          d={appearance.pathData}
          stroke="currentColor"
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth="3.75"
        />
      </svg>
    </span>
  );
}
