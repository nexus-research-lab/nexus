// INPUT: Room 草稿头像、名称、禁用状态与头像修改命令。
// OUTPUT: 公共 56px 身份头像及唯一中性选择入口。
// POS: Room 表单头像适配；材质、选择和焦点归公共 Avatar/Picker。
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
            aria-hidden="true"
            avatar={avatar}
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
