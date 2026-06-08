"use client";

import { type HTMLAttributes } from "react";
import { Hash } from "lucide-react";

import {
  get_icon_avatar_src,
  get_initials,
  get_room_avatar_icon_id,
} from "@/lib/utils";
import { cn } from "@/lib/utils";

type UiAvatarSize = "xs" | "sm" | "md" | "lg" | "xl";
type UiAvatarShape = "round" | "rounded";
type UiRoomAvatarSize = "sm" | "md" | "lg";

export interface UiAvatarMember {
  id: string;
  name: string;
  avatar?: string | null;
}

interface UiAgentAvatarProps extends HTMLAttributes<HTMLSpanElement> {
  avatar?: string | null;
  class_name?: string;
  image_class_name?: string;
  is_working?: boolean;
  name: string;
  shape?: UiAvatarShape;
  size?: UiAvatarSize;
}

interface UiRoomAvatarProps extends HTMLAttributes<HTMLSpanElement> {
  avatar?: string | null;
  class_name?: string;
  max_members?: number;
  members: UiAvatarMember[];
  room_id?: string | null;
  size?: UiRoomAvatarSize;
  title: string;
}

const AVATAR_SIZE_CLASS_MAP: Record<UiAvatarSize, string> = {
  xs: "h-5.5 w-5.5 text-[8px]",
  sm: "h-7 w-7 text-[10px]",
  md: "h-10 w-10 text-[12px]",
  lg: "h-14 w-14 text-[15px]",
  xl: "h-16 w-16 text-[17px]",
};

const ROOM_AVATAR_SIZE_CLASS_MAP: Record<UiRoomAvatarSize, string> = {
  sm: "h-8 w-8 rounded-[9px]",
  md: "h-10 w-10 rounded-[10px]",
  lg: "h-14 w-14 rounded-[16px]",
};

const ROOM_AVATAR_GRID_CLASS_MAP: Record<1 | 2 | 3, string> = {
  1: "grid-cols-1 grid-rows-1",
  2: "grid-cols-2 grid-rows-2",
  3: "grid-cols-3 grid-rows-3",
};

function room_avatar_grid_size(member_count: number): 1 | 2 | 3 {
  if (member_count <= 1) {
    return 1;
  }
  if (member_count <= 4) {
    return 2;
  }
  return 3;
}

export function UiAgentAvatar({
  avatar,
  class_name,
  className,
  image_class_name,
  is_working = false,
  name,
  shape = "round",
  size = "md",
  ...props
}: UiAgentAvatarProps) {
  const avatar_src = get_icon_avatar_src(avatar);
  const rounded_class_name = shape === "round" ? "rounded-full" : "rounded-[10px]";

  return (
    <span
      className={cn(
        "relative flex shrink-0 items-center justify-center border border-(--surface-avatar-border) bg-(--surface-avatar-background) font-semibold text-(--surface-avatar-foreground) shadow-(--surface-avatar-shadow)",
        AVATAR_SIZE_CLASS_MAP[size],
        rounded_class_name,
        is_working && "after:pointer-events-none after:absolute after:inset-[-3px] after:rounded-full after:border after:border-[color:color-mix(in_srgb,var(--primary)_48%,transparent)] after:shadow-[0_0_0_3px_color-mix(in_srgb,var(--primary)_8%,transparent)]",
        className,
        class_name,
      )}
      {...props}
    >
      {avatar_src ? (
        <img
          alt={name}
          className={cn("h-full w-full object-cover", rounded_class_name, image_class_name)}
          src={avatar_src}
        />
      ) : (
        get_initials(name, "AG", size === "xs" || size === "sm" ? 1 : 2)
      )}
    </span>
  );
}

/** 中文注释：Room 头像最多取 9 个成员做九宫格，避免业务侧各自实现不同的拼图规则。 */
export function UiRoomAvatar({
  avatar,
  class_name,
  className,
  max_members = 9,
  members,
  room_id,
  size = "md",
  title,
  ...props
}: UiRoomAvatarProps) {
  const visible_members = members.slice(0, max_members);
  const grid_size = room_avatar_grid_size(visible_members.length);

  if (visible_members.length === 0) {
    const room_avatar_id = get_room_avatar_icon_id(room_id ?? title, title, avatar);
    const room_avatar_src = get_icon_avatar_src(room_avatar_id, "room");

    return (
      <span
        className={cn(
          "flex shrink-0 items-center justify-center overflow-hidden border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-muted) shadow-(--surface-avatar-shadow)",
          ROOM_AVATAR_SIZE_CLASS_MAP[size],
          className,
          class_name,
        )}
        {...props}
      >
        {room_avatar_src ? (
          <img alt={title} className="h-full w-full object-cover" src={room_avatar_src} />
        ) : (
          <Hash className="h-4 w-4" />
        )}
      </span>
    );
  }

  return (
    <span
      className={cn(
        "grid shrink-0 gap-[2px] overflow-hidden border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_88%,white)] p-[2px] shadow-(--surface-avatar-shadow)",
        ROOM_AVATAR_SIZE_CLASS_MAP[size],
        ROOM_AVATAR_GRID_CLASS_MAP[grid_size],
        visible_members.length === 2 && "grid-rows-1",
        className,
        class_name,
      )}
      {...props}
    >
      {visible_members.map((member) => (
        <span className="min-h-0 min-w-0 overflow-hidden rounded-[5px]" key={member.id}>
          <UiAgentAvatar
            avatar={member.avatar}
            class_name="h-full w-full rounded-[5px] border-0 shadow-none"
            image_class_name="rounded-[5px]"
            name={member.name}
            shape="rounded"
            size="md"
          />
        </span>
      ))}
    </span>
  );
}
