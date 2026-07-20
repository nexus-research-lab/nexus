import type { Agent } from "@/types/agent/agent";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";

interface GroupMemberAvatarStackProps {
  members: Agent[];
  onClick: () => void;
  tourAnchor?: string;
}

export function GroupMemberAvatarStack({
  members,
  onClick,
  tourAnchor,
}: GroupMemberAvatarStackProps) {
  const { t } = useI18n();
  const visibleMembers = members.slice(0, 4);
  const overflowCount = Math.max(0, members.length - visibleMembers.length);

  return (
    <button
      className="flex h-7 items-center gap-1.5 rounded-full border border-(--divider-subtle-color) bg-(--surface-panel-background) px-2 text-[10.5px] font-medium text-(--text-default) transition-[border-color,background,color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border) hover:text-(--text-strong)"
      data-tour-anchor={tourAnchor}
      onClick={onClick}
      type="button"
    >
      <div className="flex items-center -space-x-1.5">
        {visibleMembers.map((member) => (
          <UiAgentAvatar
            avatar={member.avatar}
            className="ring-1 ring-(--background)"
            key={member.agent_id}
            name={member.name}
            size="xs"
            title={member.name}
          />
        ))}
        {overflowCount > 0 ? (
          <span className="flex h-5.5 w-5.5 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[8px] font-bold text-(--text-strong) shadow-(--surface-avatar-shadow)">
            +{overflowCount}
          </span>
        ) : null}
      </div>
      <span className="hidden sm:inline">{t("room.members")}</span>
    </button>
  );
}
