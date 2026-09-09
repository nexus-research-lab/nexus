"use client";

/**
 * INPUT: Agent/Room 身份、头像路径、成员集合与调用方投影的运行态。
 * OUTPUT: 从紧凑选择器到完整身份的唯一具名 rounded-square 头像，图片失败时保留完整字符回退。
 * POS: 头像尺寸、图片回退与按稳定成员身份排序的最多九成员拼图的唯一 UI owner；不读取业务状态。
 */
import { type HTMLAttributes, type ReactNode, useState } from "react";
import { Hash } from "lucide-react";

import {
  getIconAvatarSrc,
  getInitials,
  getRoomAvatarIconId,
} from "@/lib/avatar";
import { cn } from "@/shared/ui/class-name";

type UiAvatarSize = "xxs" | "xs" | "sm" | "md" | "lg" | "xl";
type UiRoomAvatarSize = "sm" | "md" | "lg";

interface UiAvatarMember {
  id: string;
  name: string;
  avatar?: string | null;
}

interface UiAgentAvatarProps extends HTMLAttributes<HTMLSpanElement> {
  avatar?: string | null;
  className?: string;
  imageClassName?: string;
  isWorking?: boolean;
  name: string;
  size?: UiAvatarSize;
}

interface UiRoomAvatarProps extends HTMLAttributes<HTMLSpanElement> {
  avatar?: string | null;
  className?: string;
  isWorking?: boolean;
  maxMembers?: number;
  members: UiAvatarMember[];
  roomId?: string | null;
  size?: UiRoomAvatarSize;
  title: string;
}

const AVATAR_SIZE_CLASS_MAP: Record<UiAvatarSize, string> = {
  xxs: "h-4 w-4 text-[8px]",
  xs: "h-5.5 w-5.5 text-[8px]",
  sm: "h-7 w-7 text-2xs",
  md: "h-10 w-10 text-compact",
  lg: "h-14 w-14 text-base",
  xl: "h-16 w-16 text-md",
};

const AVATAR_RADIUS_CLASS_MAP: Record<UiAvatarSize, string> = {
  xxs: "rounded-(--radius-control-xs)",
  xs: "rounded-(--radius-control-xs)",
  sm: "rounded-(--radius-control-sm)",
  md: "rounded-(--radius-control-md)",
  lg: "rounded-(--radius-control-lg)",
  xl: "rounded-(--radius-control-lg)",
};

const AVATAR_HALO_RADIUS_CLASS_MAP: Record<UiAvatarSize, string> = {
  xxs: "after:rounded-[calc(var(--radius-control-xs)+3px)]",
  xs: "after:rounded-[calc(var(--radius-control-xs)+3px)]",
  sm: "after:rounded-[calc(var(--radius-control-sm)+3px)]",
  md: "after:rounded-[calc(var(--radius-control-md)+3px)]",
  lg: "after:rounded-[calc(var(--radius-control-lg)+3px)]",
  xl: "after:rounded-[calc(var(--radius-control-lg)+3px)]",
};

const ROOM_AVATAR_SIZE_CLASS_MAP: Record<UiRoomAvatarSize, string> = {
  sm: "h-8 w-8 rounded-(--radius-control-sm)",
  md: "h-10 w-10 rounded-(--radius-control-md)",
  lg: "h-14 w-14 rounded-(--radius-control-lg)",
};

// 拼图文字是头像内部微几何：多成员只取一个完整字符，字号随可用格宽缩放。
const ROOM_MEMBER_TEXT_CLASS_MAP: Record<UiRoomAvatarSize, Record<1 | 2 | 3, string>> = {
  sm: { 1: "text-[12px]", 2: "text-[10px]", 3: "text-[6px]" },
  md: { 1: "text-[14px]", 2: "text-[12px]", 3: "text-[8px]" },
  lg: { 1: "text-[16px]", 2: "text-[14px]", 3: "text-[10px]" },
};

const ROOM_AVATAR_GRID_CLASS_MAP: Record<1 | 2 | 3, string> = {
  1: "grid-cols-1 grid-rows-1",
  2: "grid-cols-2 grid-rows-2",
  3: "grid-cols-3 grid-rows-3",
};

function roomAvatarGridSize(memberCount: number): 1 | 2 | 3 {
  if (memberCount <= 1) {
    return 1;
  }
  if (memberCount <= 4) {
    return 2;
  }
  return 3;
}

/** 图片身份变化时由 key 重建失败状态；同地址不循环重试，名称变化仍更新回退。 */
function AvatarContent({
  fallback,
  imageClassName,
  src,
}: {
  fallback: ReactNode;
  imageClassName?: string;
  src: string | null;
}) {
  const [failed, setFailed] = useState(false);
  return (
    <span className="flex h-full w-full min-w-0 items-center justify-center overflow-hidden [border-radius:inherit]">
      {src && !failed ? (
        <img alt="" className={cn("h-full w-full object-cover", imageClassName)} onError={() => setFailed(true)} src={src} />
      ) : fallback}
    </span>
  );
}

export function UiAgentAvatar({
  avatar,
  className,
  imageClassName,
  isWorking = false,
  name,
  size = "md",
  ...props
}: UiAgentAvatarProps) {
  const avatarSrc = getIconAvatarSrc(avatar);
  const roundedClassName = AVATAR_RADIUS_CLASS_MAP[size];

  return (
    <span
      aria-label={name}
      className={cn(
        "relative flex shrink-0 items-center justify-center border border-(--surface-avatar-border) bg-(--surface-avatar-background) font-semibold text-(--surface-avatar-foreground) shadow-(--surface-avatar-shadow)",
        AVATAR_SIZE_CLASS_MAP[size],
        roundedClassName,
        isWorking && cn(
          "after:pointer-events-none after:absolute after:inset-[-3px] after:border after:border-[color:var(--status-running-soft-border)] after:shadow-[0_0_0_3px_var(--status-running-soft-bg)]",
          AVATAR_HALO_RADIUS_CLASS_MAP[size],
        ),
        className,
      )}
      role="img"
      {...props}
    >
      <AvatarContent
        fallback={getInitials(name, "AG", size === "xxs" || size === "xs" || size === "sm" ? 1 : 2)}
        imageClassName={imageClassName}
        key={avatarSrc}
        src={avatarSrc}
      />
    </span>
  );
}

/** 中文注释：Room 头像最多取 9 个成员做九宫格，避免业务侧各自实现不同的拼图规则。 */
export function UiRoomAvatar({
  avatar,
  className,
  isWorking = false,
  maxMembers = 9,
  members,
  roomId,
  size = "md",
  title,
  ...props
}: UiRoomAvatarProps) {
  const visibleMembers = [...members]
    .sort((left, right) => left.id < right.id ? -1 : left.id > right.id ? 1 : 0)
    .slice(0, Math.max(0, Math.min(9, Math.floor(maxMembers))));
  const gridSize = roomAvatarGridSize(visibleMembers.length);
  const isMemberPair = visibleMembers.length === 2;
  const hasMembers = visibleMembers.length > 0;
  const roomAvatarSrc = hasMembers ? null : getIconAvatarSrc(
    getRoomAvatarIconId(roomId ?? title, title, avatar), "room",
  );

  return (
    <span
      aria-label={title}
      className={cn(
        "relative shrink-0 overflow-hidden border",
        hasMembers
          ? "border-[color:color-mix(in_srgb,var(--divider-subtle-color)_82%,transparent)] bg-[color:color-mix(in_srgb,var(--background)_78%,var(--surface-panel-background)_22%)]"
          : "flex items-center justify-center border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-muted) shadow-(--surface-avatar-shadow)",
        ROOM_AVATAR_SIZE_CLASS_MAP[size],
        hasMembers && (isMemberPair
          ? "block"
          : cn("grid gap-[2px] p-[2px]", ROOM_AVATAR_GRID_CLASS_MAP[gridSize])),
        isWorking && "ring-2 ring-[color:var(--status-running-soft-border)] ring-offset-1 ring-offset-(--background)",
        className,
      )}
      role="img"
      {...props}
    >
      {hasMembers ? visibleMembers.map((member, memberIndex) => (
        <span
          aria-hidden="true"
          className={cn(
            "min-h-0 min-w-0 overflow-hidden rounded-[5px] bg-(--surface-avatar-background) font-semibold text-(--surface-avatar-foreground) leading-none",
            ROOM_MEMBER_TEXT_CLASS_MAP[size][gridSize],
            isMemberPair &&
              "absolute h-[56%] w-[56%] border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)",
            isMemberPair && memberIndex === 0 && "left-[5%] top-[12%] z-[1]",
            isMemberPair && memberIndex === 1 && "right-[5%] bottom-[12%] z-[2]",
          )}
          key={member.id}
        >
          <AvatarContent
            fallback={getInitials(member.name, "A", visibleMembers.length > 1 ? 1 : 2)}
            key={getIconAvatarSrc(member.avatar)}
            src={getIconAvatarSrc(member.avatar)}
          />
        </span>
      )) : (
        <AvatarContent fallback={<Hash aria-hidden="true" className="h-4 w-4" />} key={roomAvatarSrc} src={roomAvatarSrc} />
      )}
    </span>
  );
}
