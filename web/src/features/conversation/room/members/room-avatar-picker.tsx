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
            className="h-[72px] w-[72px] rounded-[16px] transition-[border-color,box-shadow] duration-(--motion-duration-fast) group-hover:border-[color:color-mix(in_srgb,var(--primary)_35%,var(--surface-avatar-border))] group-hover:shadow-[0_8px_20px_color-mix(in_srgb,var(--shadow-color)_12%,transparent)]"
            members={[]}
            roomId={name}
            title={name || fallbackTitle}
          />
          <span className="inline-flex min-h-7 items-center gap-1 rounded-full border border-[color:color-mix(in_srgb,var(--primary)_18%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] px-2.5 text-[11.5px] font-semibold text-(--primary) transition-[background,border-color] group-hover:border-[color:color-mix(in_srgb,var(--primary)_32%,var(--divider-subtle-color))] group-hover:bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)]">
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
      triggerClassName="group relative flex shrink-0 flex-col items-center gap-1.5 rounded-[18px] text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_30%,transparent)] focus-visible:ring-offset-2 focus-visible:ring-offset-(--background) disabled:cursor-not-allowed disabled:opacity-55"
      value={avatar}
    />
  );
}
