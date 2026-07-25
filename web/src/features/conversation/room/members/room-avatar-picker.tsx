"use client";

import { ChevronDown } from "lucide-react";

import {
  ROOM_ICON_ID_END,
  ROOM_ICON_ID_START,
} from "@/lib/avatar";
import { cn } from "@/shared/ui/class-name";
import { UiRoomAvatar } from "@/shared/ui/display/avatar";
import { IconPickerPopover } from "@/shared/ui/icon-picker/icon-picker-popover";
import { useI18n } from "@/shared/i18n/i18n-context";

interface RoomAvatarPickerProps {
  avatar: string;
  disabled: boolean;
  fallbackTitle: string;
  name: string;
  onChange: (avatar: string) => void;
}

export function RoomAvatarPicker({
  avatar,
  disabled,
  fallbackTitle,
  name,
  onChange,
}: RoomAvatarPickerProps) {
  const { t } = useI18n();

  return (
    <IconPickerPopover
      ariaLabel={t("room.choose_avatar")}
      disabled={disabled}
      iconFamily="room"
      maxIcons={ROOM_ICON_ID_END - ROOM_ICON_ID_START + 1}
      onSelect={onChange}
      renderTrigger={(isOpen) => (
        <>
          <UiRoomAvatar
            avatar={avatar}
            className="transition-[border-color] duration-(--motion-duration-fast) group-hover:border-(--surface-interactive-hover-border)"
            members={[]}
            roomId={name}
            size="lg"
            title={name || fallbackTitle}
          />
          <span className="inline-flex h-7 items-center gap-1 radius-control-sm px-2 text-compact font-medium text-(--text-muted) transition-[background,color] group-hover:bg-(--surface-interactive-hover-background) group-hover:text-(--text-strong)">
            {t("room.change_avatar")}
            <ChevronDown
              className={cn(
                "h-3 w-3 transition-transform duration-(--motion-duration-fast)",
                isOpen && "rotate-180",
              )}
            />
          </span>
        </>
      )}
      startIconId={ROOM_ICON_ID_START}
      triggerClassName="group relative flex shrink-0 flex-col items-center gap-1 rounded-[12px] text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] focus-visible:ring-offset-2 focus-visible:ring-offset-(--background) disabled:cursor-not-allowed disabled:opacity-55"
      value={avatar}
    />
  );
}
