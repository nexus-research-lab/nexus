"use client";

import { type HTMLAttributes } from "react";
import { Hash } from "lucide-react";

import {
  getIconAvatarSrc,
  getInitials,
  getRoomAvatarIconId,
} from "@/lib/avatar";
import { cn } from "@/shared/ui/class-name";

type UiAvatarSize = "xs" | "sm" | "md" | "lg" | "xl";
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
  xs: "h-5.5 w-5.5 text-[8px]",
  sm: "h-7 w-7 text-2xs",
  md: "h-10 w-10 text-compact",
  lg: "h-14 w-14 text-base",
  xl: "h-16 w-16 text-md",
};

const AVATAR_RADIUS_CLASS_MAP: Record<UiAvatarSize, string> = {
  xs: "rounded-[6px]",
  sm: "rounded-[8px]",
  md: "rounded-[10px]",
  lg: "rounded-[12px]",
  xl: "rounded-[12px]",
};

const AVATAR_HALO_RADIUS_CLASS_MAP: Record<UiAvatarSize, string> = {
  xs: "after:rounded-[9px]",
  sm: "after:rounded-[11px]",
  md: "after:rounded-[13px]",
  lg: "after:rounded-[15px]",
  xl: "after:rounded-[15px]",
};

const ROOM_AVATAR_SIZE_CLASS_MAP: Record<UiRoomAvatarSize, string> = {
  sm: "h-8 w-8 radius-control-sm",
  md: "h-10 w-10 rounded-[10px]",
  lg: "h-14 w-14 surface-radius-md",
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

export function UiAgentAvatar({
  avatar,
  className,
  imageClassName: imageClassName,
  isWorking: isWorking = false,
  name,
  size = "md",
  ...props
}: UiAgentAvatarProps) {
  const avatarSrc = getIconAvatarSrc(avatar);
  const roundedClassName = AVATAR_RADIUS_CLASS_MAP[size];

  return (
    <span
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
      {...props}
    >
      {avatarSrc ? (
        <img
          alt={name}
          className={cn("h-full w-full object-cover", roundedClassName, imageClassName)}
          src={avatarSrc}
        />
      ) : (
        getInitials(name, "AG", size === "xs" || size === "sm" ? 1 : 2)
      )}
    </span>
  );
}

/** 中文注释：Room 头像最多取 9 个成员做九宫格，避免业务侧各自实现不同的拼图规则。 */
export function UiRoomAvatar({
  avatar,
  className,
  isWorking: isWorking = false,
  maxMembers: maxMembers = 9,
  members,
  roomId: roomId,
  size = "md",
  title,
  ...props
}: UiRoomAvatarProps) {
  const visibleMembers = members.slice(0, maxMembers);
  const gridSize = roomAvatarGridSize(visibleMembers.length);
  const isMemberPair = visibleMembers.length === 2;

  if (visibleMembers.length === 0) {
    const roomAvatarId = getRoomAvatarIconId(roomId ?? title, title, avatar);
    const roomAvatarSrc = getIconAvatarSrc(roomAvatarId, "room");

    return (
      <span
        className={cn(
          "relative flex shrink-0 items-center justify-center overflow-hidden border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-muted) shadow-(--surface-avatar-shadow)",
          ROOM_AVATAR_SIZE_CLASS_MAP[size],
          isWorking && "ring-2 ring-[color:var(--status-running-soft-border)] ring-offset-1 ring-offset-(--background)",
          className,
        )}
        {...props}
      >
        {roomAvatarSrc ? (
          <img alt={title} className="h-full w-full object-cover" src={roomAvatarSrc} />
        ) : (
          <Hash className="h-4 w-4" />
        )}
      </span>
    );
  }

  return (
    <span
      className={cn(
        "relative shrink-0 overflow-hidden border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_82%,transparent)] bg-[color:color-mix(in_srgb,var(--background)_78%,var(--surface-panel-background)_22%)]",
        ROOM_AVATAR_SIZE_CLASS_MAP[size],
        isMemberPair
          ? "block"
          : cn("grid gap-[2px] p-[2px]", ROOM_AVATAR_GRID_CLASS_MAP[gridSize]),
        isWorking && "ring-2 ring-[color:var(--status-running-soft-border)] ring-offset-1 ring-offset-(--background)",
        className,
      )}
      {...props}
    >
      {visibleMembers.map((member, memberIndex) => (
        <span
          className={cn(
            "min-h-0 min-w-0 overflow-hidden rounded-[5px]",
            isMemberPair &&
              "absolute h-[56%] w-[56%] border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)",
            isMemberPair && memberIndex === 0 && "left-[5%] top-[12%] z-[1]",
            isMemberPair && memberIndex === 1 && "right-[5%] bottom-[12%] z-[2]",
          )}
          key={member.id}
        >
          <UiAgentAvatar
            avatar={member.avatar}
            className="h-full w-full !rounded-[5px] border-0 shadow-none"
            imageClassName="!rounded-[5px]"
            name={member.name}
            size="md"
          />
        </span>
      ))}
    </span>
  );
}
