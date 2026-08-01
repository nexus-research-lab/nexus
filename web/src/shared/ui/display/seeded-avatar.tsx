"use client";

import {
  Asterisk,
  Box,
  Braces,
  CircleDot,
  Diamond,
  Orbit,
  Sparkles,
  Zap,
  type LucideIcon,
} from "lucide-react";
import type { HTMLAttributes } from "react";

import {
  getSeededAvatarAppearance,
  type SeededAvatarIcon,
} from "@/lib/seeded-avatar";
import { cn } from "@/shared/ui/class-name";

type UiSeededAvatarSize = "sm" | "md";

interface UiSeededAvatarProps extends HTMLAttributes<HTMLSpanElement> {
  seed: string;
  size?: UiSeededAvatarSize;
}

const SEEDED_AVATAR_ICON: Readonly<Record<SeededAvatarIcon, LucideIcon>> = {
  asterisk: Asterisk,
  box: Box,
  braces: Braces,
  "circle-dot": CircleDot,
  diamond: Diamond,
  orbit: Orbit,
  sparkles: Sparkles,
  zap: Zap,
};

const SEEDED_AVATAR_SIZE_CLASS_NAME: Readonly<
  Record<UiSeededAvatarSize, string>
> = {
  sm: "h-9 w-9 [&_svg]:h-4 [&_svg]:w-4",
  md: "h-10 w-10 [&_svg]:h-[18px] [&_svg]:w-[18px]",
};

/** 中文注释：圆形标记只表示非人物资源，不改变 Agent 与 Room 的头像规范。 */
export function UiSeededAvatar({
  className,
  seed,
  size = "md",
  style,
  ...props
}: UiSeededAvatarProps) {
  const appearance = getSeededAvatarAppearance(seed);
  const Icon = SEEDED_AVATAR_ICON[appearance.icon];

  return (
    <span
      {...props}
      aria-hidden="true"
      className={cn(
        "flex shrink-0 items-center justify-center rounded-full border border-(--divider-subtle-color)",
        SEEDED_AVATAR_SIZE_CLASS_NAME[size],
        className,
      )}
      style={{
        backgroundColor: appearance.backgroundColor,
        color: appearance.foregroundColor,
        ...style,
      }}
    >
      <Icon strokeWidth={2.25} />
    </span>
  );
}
