// INPUT: Room 基础设置草稿、成员候选与字段更新/提交命令。
// OUTPUT: 使用共享表单与选择原语的 Room 名称、群主和协作设置。
// POS: Room 创建/管理弹窗的设置组合层；不拥有草稿状态或服务端提交。

import { Crown } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiCheckbox } from "@/shared/ui/form/checkbox";
import { UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";

import type {
  RoomDialogFormState,
  RoomMemberAgentOption,
} from "./create-room-dialog-types";
import { RoomAvatarPicker } from "./room-avatar-picker";

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
    <div className="flex min-h-0 min-w-0 flex-col gap-4">
      <p className="dialog-label">{t("room.settings_title")}</p>
      <div className="flex items-start gap-3">
        <RoomAvatarPicker
          avatar={state.avatar}
          disabled={isCreating}
          fallbackTitle={avatarFallbackTitle}
          name={state.name}
          onChange={setters.setAvatar}
        />
        <label className="min-w-0 flex-1 space-y-1.5">
          <span className="dialog-label">{t("room.name")}</span>
          <UiInput
            aria-label={t("room.name")}
            className="min-w-0"
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
            value={state.name}
          />
        </label>
      </div>
      <div className="flex flex-col gap-2 border-t divider-subtle pt-3">
        <div className="flex items-center gap-2">
          <div className="flex shrink-0 items-center gap-1.5 text-xs font-semibold text-(--text-muted)">
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
          label="未 @ 时由群主接管，可回答或协调"
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
        "flex items-center gap-2 px-0.5 text-xs font-medium text-(--text-default)",
        className,
      )}
    >
      <UiCheckbox
        checked={checked}
        checkboxSize="small"
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
      />
      <span className="min-w-0 truncate">{label}</span>
    </label>
  );
}
