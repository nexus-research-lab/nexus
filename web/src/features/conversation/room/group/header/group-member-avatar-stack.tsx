import { UsersRound } from "lucide-react";

import type { Agent } from "@/types/agent/agent";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
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
    <UiButton
      aria-label={t("room.members")}
      className="workspace-surface-header-control-segment workspace-surface-header-member-control h-9 min-h-0 gap-1.5 px-2.5"
      data-tour-anchor={tourAnchor}
      onClick={onClick}
      size="md"
      title={t("room.members")}
      variant="ghost"
    >
      <UsersRound className="workspace-surface-header-member-icon hidden h-3.5 w-3.5" />
      <div className="workspace-surface-header-member-avatars flex items-center -space-x-1.5">
        {visibleMembers.map((member) => (
          <UiAgentAvatar
            avatar={member.avatar}
            className="ring-1 ring-[color:color-mix(in_srgb,var(--background)_76%,var(--surface-panel-background)_24%)]"
            key={member.agent_id}
            name={member.name}
            size="xs"
            title={member.name}
          />
        ))}
        {overflowCount > 0 ? (
          <span className="flex h-5.5 w-5.5 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[8px] font-semibold text-(--text-strong) shadow-(--surface-avatar-shadow)">
            +{overflowCount}
          </span>
        ) : null}
      </div>
      <span className="workspace-surface-header-member-label">{t("room.members")}</span>
    </UiButton>
  );
}
