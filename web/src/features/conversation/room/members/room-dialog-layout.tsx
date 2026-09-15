// INPUT: 群设置、成员目录与名称头像字段的视图和输入命令。
// OUTPUT: 本地与在线 Room 共用的双栏、身份表单和固定高度成员目录。
// POS: Room 管理视图组合；不持有成员权限、草稿或提交事务。
import type { ReactNode } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiInput, UiSearchInput } from "@/shared/ui/form/form-control";
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
