"use client";

import { type HTMLAttributes, useMemo } from "react";

import { getSeededAvatarAppearance } from "@/lib/seeded-avatar";
import { cn } from "@/shared/ui/class-name";

type UiSeededAvatarSize = "sm" | "md";

interface UiSeededAvatarProps extends HTMLAttributes<HTMLSpanElement> {
  seed: string;
  size?: UiSeededAvatarSize;
}

const SEEDED_AVATAR_SIZE_CLASS_NAME: Readonly<
  Record<UiSeededAvatarSize, string>
> = {
  sm: "h-9 w-9",
  md: "h-10 w-10",
};

/** 中文注释：圆形标记只表示非人物资源，不改变 Agent 与 Room 的头像规范。 */
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
      <svg
        aria-hidden="true"
        className="h-full w-full"
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
