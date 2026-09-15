// INPUT: Room 基础设置草稿、成员候选与字段更新/提交命令。
// OUTPUT: 使用共享表单与选择原语的 Room 名称、群主和协作设置。
// POS: Room 创建/管理弹窗的设置组合层；不拥有草稿状态或服务端提交。

import { Crown } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiCheckbox } from "@/shared/ui/form/checkbox";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";

import type {
  RoomDialogFormState,
  RoomMemberAgentOption,
} from "./create-room-dialog-types";
import { RoomIdentityFields } from "./room-dialog-layout";

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
    { label: t("room.host_unset"), value: "" },
    ...selectedAgents.map((agent) => ({
      label: agent.name,
      value: agent.agent_id,
    })),
  ];
  return (
    <div className="flex min-h-0 min-w-0 flex-col gap-4">
      <p className="dialog-label">{t("room.settings_title")}</p>
      <RoomIdentityFields avatar={state.avatar} name={state.name} disabled={isCreating} fallbackTitle={avatarFallbackTitle}
        onAvatarChange={setters.setAvatar} onNameChange={setters.setName} onSubmit={canSubmit ? onSubmit : undefined} />
      <div className="flex flex-col gap-2 border-t divider-subtle pt-3">
        <div className="flex items-center gap-2">
          <div className="flex shrink-0 items-center gap-1.5 text-xs font-semibold text-(--text-muted)">
            <Crown className="h-3.5 w-3.5 text-primary" />
            <span>{t("room.host_label")}</span>
          </div>
          <UiSelectMenu
            ariaLabel={t("room.host_select_label")}
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
          disabled={!state.hostAgentId || state.location === "online" || isCreating}
          label={t("room.host_auto_reply_label")}
          onChange={setters.setHostAutoReplyEnabled}
        />
        <RoomSettingCheckbox
          checked={state.privateMessagesEnabled}
          disabled={isCreating}
          label={t("room.private_messages_label")}
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
      <span className="min-w-0">{label}</span>
    </label>
  );
}
