// INPUT: 稳定资源种子、共享尺寸、瞬时运行状态与标准 span 属性。
// OUTPUT: 确定性数学曲线头像，并统一投影视觉尺寸、圆角和运行态外环。
// POS: 数学曲线资源头像的唯一 DOM/视觉 owner；业务层只传身份和语义状态。

"use client";

import { type HTMLAttributes, useMemo } from "react";

import { getSeededAvatarAppearance } from "@/lib/seeded-avatar";
import { cn } from "@/shared/ui/class-name";

type UiSeededAvatarSize = "2xs" | "xs" | "sm" | "md" | "lg";
type UiSeededAvatarState = "default" | "running";

interface UiSeededAvatarProps extends HTMLAttributes<HTMLSpanElement> {
  seed: string;
  size?: UiSeededAvatarSize;
  state?: UiSeededAvatarState;
}

const SEEDED_AVATAR_SIZE_CLASS_NAME: Readonly<
  Record<UiSeededAvatarSize, string>
> = {
  "2xs": "h-6 w-6",
  xs: "h-8 w-8",
  sm: "h-9 w-9",
  md: "h-10 w-10",
  lg: "h-12 w-12",
};

const SEEDED_AVATAR_RADIUS_CLASS_NAME: Readonly<
  Record<UiSeededAvatarSize, string>
> = {
  "2xs": "radius-control-xs",
  xs: "radius-control-sm",
  sm: "radius-control-sm",
  md: "radius-control-md",
  lg: "radius-control-lg",
};

const SEEDED_AVATAR_STATE_CLASS_NAME: Readonly<
  Record<UiSeededAvatarState, string>
> = {
  default: "",
  running:
    "ring-1 ring-[color:var(--status-running-soft-border)] ring-offset-1 ring-offset-(--background)",
};

export function UiSeededAvatar({
  className,
  seed,
  size = "md",
  state = "default",
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
        SEEDED_AVATAR_STATE_CLASS_NAME[state],
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
