import { Crown } from "lucide-react";

import { cn, ROOM_ICON_ID_END, ROOM_ICON_ID_START } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiRoomAvatar } from "@/shared/ui/display/avatar";
import { IconPicker } from "@/shared/ui/icon-picker/icon-picker";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";

import type {
  RoomDialogFormState,
  RoomMemberAgentOption,
} from "./create-room-dialog-types";

interface RoomSettingsFormProps {
  avatarFallbackTitle: string;
  canSubmit: boolean;
  isCreating: boolean;
  onSubmit: () => void;
  selectedAgents: RoomMemberAgentOption[];
  setters: {
    setAvatar: (avatar: string) => void;
    setHostAgentId: (agentId: string) => void;
    setHostAutoReplyEnabled: (enabled: boolean) => void;
    setName: (name: string) => void;
    setPrivateMessagesEnabled: (enabled: boolean) => void;
  };
  state: RoomDialogFormState;
}

export function RoomSettingsForm({
  avatarFallbackTitle,
  canSubmit,
  isCreating,
  onSubmit,
  selectedAgents,
  setters,
  state,
}: RoomSettingsFormProps) {
  const { t } = useI18n();
  const hostOptions = [
    { label: "未设置", value: "" },
    ...selectedAgents.map((agent) => ({
      label: agent.name,
      value: agent.agent_id,
    })),
  ];
  return (
    <div className="flex min-h-0 w-60 shrink-0 flex-col gap-3">
      <p className="dialog-label">{t("room.settings_title")}</p>
      <div className="flex flex-col gap-3">
        <div className="flex items-center gap-3">
          <UiRoomAvatar
            avatar={state.avatar}
            className="h-11 w-11 rounded-[14px]"
            members={[]}
            roomId={state.name}
            title={state.name || avatarFallbackTitle}
          />
          <input
            aria-label={t("room.settings_title")}
            className="dialog-input min-w-0 flex-1 rounded-xl px-3 py-2 text-sm text-(--text-strong) placeholder:text-(--text-soft) focus-visible:outline-none"
            data-autofocus="true"
            maxLength={64}
            onChange={(event) => setters.setName(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && canSubmit) {
                onSubmit();
              }
            }}
            placeholder={t("room.name_required_placeholder")}
            required
            type="text"
            value={state.name}
          />
        </div>
        <IconPicker
          className="mt-3"
          disabled={isCreating}
          iconFamily="room"
          iconSize="sm"
          layout="row"
          maxIcons={ROOM_ICON_ID_END - ROOM_ICON_ID_START + 1}
          onSelect={setters.setAvatar}
          showClear={false}
          startIconId={ROOM_ICON_ID_START}
          value={state.avatar}
        />
      </div>
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <div className="flex shrink-0 items-center gap-1.5 text-[11px] font-semibold text-(--text-muted)">
            <Crown className="h-3.5 w-3.5 text-primary" />
            <span>群主</span>
          </div>
          <UiSelectMenu
            ariaLabel="选择 Room 群主"
            className="min-w-0 flex-1"
            disabled={selectedAgents.length === 0 || isCreating}
            onChange={setters.setHostAgentId}
            options={hostOptions}
            size="sm"
            surface="dialog"
            value={state.hostAgentId}
          />
        </div>
        <RoomSettingCheckbox
          checked={state.hostAutoReplyEnabled}
          className="mt-1.5"
          disabled={!state.hostAgentId || isCreating}
          label="未 @ 时由群主接管，可回答或委派"
          onChange={setters.setHostAutoReplyEnabled}
        />
        <RoomSettingCheckbox
          checked={state.privateMessagesEnabled}
          disabled={isCreating}
          label="允许成员私信协作"
          onChange={setters.setPrivateMessagesEnabled}
        />
      </div>
    </div>
  );
}

function RoomSettingCheckbox({
  checked,
  className,
  disabled,
  label,
  onChange,
}: {
  checked: boolean;
  className?: string;
  disabled: boolean;
  label: string;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label
      className={cn(
        "flex items-center gap-2 px-0.5 text-[11px] font-medium text-(--text-default)",
        className,
      )}
    >
      <input
        checked={checked}
        className="h-3.5 w-3.5 shrink-0 accent-[var(--primary)] disabled:cursor-not-allowed disabled:opacity-55"
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
        type="checkbox"
      />
      <span className="min-w-0 truncate">{label}</span>
    </label>
  );
}
