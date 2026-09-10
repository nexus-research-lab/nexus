// INPUT: 当前 Room 成员、目录加载/禁用状态和显式打开动作。
// OUTPUT: 36px 成员入口，四枚公共小头像、可增长的额外人数徽标与唯一具名按钮。
// POS: Room Header 成员摘要；头像与 Badge 持有外形，外层 Header 持有响应式收缩。

import { UsersRound } from "lucide-react";

import type { Agent } from "@/types/agent/agent";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiBadge } from "@/shared/ui/display/badge";
import { getAgentDisplayName } from "@/lib/agent-display-name";

interface GroupMemberAvatarStackProps {
  disabled?: boolean;
  isLoading?: boolean;
  members: Agent[];
  onClick: () => void;
  tourAnchor?: string;
}

export function GroupMemberAvatarStack({
  disabled = false,
  isLoading = false,
  members,
  onClick,
  tourAnchor,
}: GroupMemberAvatarStackProps) {
  const { t } = useI18n();
  const visibleMembers = members.slice(0, 4);
  const overflowCount = Math.max(0, members.length - visibleMembers.length);

  return (
    <UiButton
      aria-busy={isLoading || undefined}
      aria-label={t("room.members_with_count", { count: members.length })}
      className="workspace-surface-header-control-segment workspace-surface-header-member-control h-9 min-h-0 gap-1.5 px-2.5"
      data-tour-anchor={tourAnchor}
      disabled={disabled || isLoading}
      onClick={onClick}
      size="md"
      title={t("room.members_with_count", { count: members.length })}
      variant="ghost"
    >
      <UsersRound aria-hidden="true" className="workspace-surface-header-member-icon hidden h-3.5 w-3.5" />
      {visibleMembers.length > 0 ? (
        <div aria-hidden="true" className="workspace-surface-header-member-avatars flex items-center gap-1.5">
          <div className="flex items-center -space-x-1.5">
            {visibleMembers.map((member) => (
              <UiAgentAvatar
                avatar={member.avatar}
                key={member.agent_id}
                name={getAgentDisplayName(member.name, t)}
                size="xs"
              />
            ))}
          </div>
          {overflowCount > 0 ? (
            <UiBadge shape="pill" size="sm">
              +{overflowCount}
            </UiBadge>
          ) : null}
        </div>
      ) : null}
      <span className="workspace-surface-header-member-label">{t("room.members")}</span>
    </UiButton>
  );
}
