// INPUT: 群设置、成员目录、成员分类切换与 Agent 暂停控制的视图和输入命令。
// OUTPUT: 本地与在线 Room 共用的双栏、身份表单、固定高度成员目录与行内控制原语。
// POS: Room 管理视图组合；不持有成员权限、草稿或提交事务。
import { Pause, Play } from "lucide-react";
import type { ReactNode } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { UiInput, UiSearchInput } from "@/shared/ui/form/form-control";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { RoomAvatarPicker } from "./room-avatar-picker";

export function RoomDialogColumns({ settings, children }: { settings: ReactNode; children: ReactNode }) {
  return <div className="grid min-h-0 grid-cols-[minmax(280px,0.9fr)_minmax(0,1.1fr)] gap-5 max-md:grid-cols-1">
    <div className="flex min-w-0 flex-col gap-5">{settings}</div>
    <div className="min-w-0 border-l divider-subtle pl-5 max-md:border-l-0 max-md:border-t max-md:pl-0 max-md:pt-4">{children}</div>
  </div>;
}

export function RoomIdentityFields({ avatar, name, disabled, fallbackTitle, onAvatarChange, onNameChange, onSubmit }: {
  avatar: string; name: string; disabled: boolean; fallbackTitle: string;
  onAvatarChange: (value: string) => void; onNameChange: (value: string) => void; onSubmit?: () => void;
}) {
  const { t } = useI18n();
  return <div className="flex items-start gap-3">
    <RoomAvatarPicker avatar={avatar} disabled={disabled} fallbackTitle={fallbackTitle} name={name} onChange={onAvatarChange} />
    <label className="min-w-0 flex-1 space-y-1.5">
      <span className="dialog-label">{t("room.name")}</span>
      <UiInput aria-label={t("room.name")} className="min-w-0" data-autofocus="true" disabled={disabled} maxLength={64}
        onChange={(event) => onNameChange(event.target.value)}
        onKeyDown={(event) => { if (event.key === "Enter" && !event.nativeEvent.isComposing && event.keyCode !== 229) onSubmit?.(); }}
        placeholder={t("room.name_required_placeholder")} required value={name} />
    </label>
  </div>;
}

export function RoomMemberDirectory({ header, query, onQueryChange, searchLabel, disabled, children }: {
  header: ReactNode; query: string; onQueryChange: (value: string) => void;
  searchLabel: string; disabled?: boolean; children: ReactNode;
}) {
  return <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
    <div className="flex h-7 shrink-0 items-center">{header}</div>
    <UiSearchInput aria-label={searchLabel} controlSize="md" disabled={disabled} onChange={onQueryChange} placeholder={searchLabel} value={query} variant="dialog" />
    <div className="flex h-[min(42vh,320px)] min-h-0 shrink-0 flex-col overflow-hidden max-md:h-[240px]">
      <div className="soft-scrollbar flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto" data-room-member-selection-list="true">{children}</div>
    </div>
  </div>;
}

export function RoomMemberTypeTabs({
  agentCount,
  disabled = false,
  onChange,
  userCount,
  value,
}: {
  agentCount: number;
  disabled?: boolean;
  onChange: (value: "users" | "agents") => void;
  userCount: number;
  value: "users" | "agents";
}) {
  const { t } = useI18n();
  return (
    <UiSegmentedControl
      className="w-full"
      density="compact"
      disabled={disabled}
      onChange={onChange}
      options={[
        { label: t("room.people_count", { count: userCount }), value: "users" },
        { label: t("room.agents_count", { count: agentCount }), value: "agents" },
      ]}
      stretch
      title={t("room.member_type")}
      value={value}
    />
  );
}

export function RoomAgentParticipationButton({
  disabled = false,
  name,
  onToggle,
  paused,
}: {
  disabled?: boolean;
  name: string;
  onToggle: () => void;
  paused: boolean;
}) {
  const { t } = useI18n();
  const actionLabel = t(paused ? "room.resume_member" : "room.pause_member", { name });
  const ParticipationIcon = paused ? Play : Pause;
  return (
    <UiChoiceButton
      active={paused}
      aria-label={actionLabel}
      choiceSize="xs"
      disabled={disabled}
      onClick={(event) => {
        event.stopPropagation();
        onToggle();
      }}
      title={actionLabel}
      tone="neutral"
    >
      <ParticipationIcon className="h-3 w-3" />
      <span className="max-sm:hidden">{t(paused ? "room.resume_participation" : "room.pause_participation")}</span>
    </UiChoiceButton>
  );
}

export function RoomDirectoryEmptyState({ memberType }: { memberType: "users" | "agents" }) {
  const { t } = useI18n();
  return (
    <p className={cn("px-3 py-8 text-center", getUiTypographyClassName({ role: "supporting", tone: "muted" }))}>
      {t(memberType === "users" ? "room.people_empty" : "room.agents_empty")}
    </p>
  );
}
