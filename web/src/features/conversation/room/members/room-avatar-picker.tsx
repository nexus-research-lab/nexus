"use client";

import {
  ROOM_ICON_ID_END,
  ROOM_ICON_ID_START,
} from "@/lib/avatar";
import { UiRoomAvatar } from "@/shared/ui/display/avatar";
import {
  IconPickerPopover,
  IconPickerTriggerLabel,
} from "@/shared/ui/icon-picker/icon-picker-popover";
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
          <IconPickerTriggerLabel isOpen={isOpen}>
            {t("room.change_avatar")}
          </IconPickerTriggerLabel>
        </>
      )}
      startIconId={ROOM_ICON_ID_START}
      triggerGap="compact"
      value={avatar}
    />
  );
}
