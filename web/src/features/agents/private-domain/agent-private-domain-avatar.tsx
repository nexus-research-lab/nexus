// INPUT: Exact participants/owner identity, avatar density and localized display names.
// OUTPUT: Participant avatars with readable names and a bounded peer stack with overflow count.
// POS: Private-domain identity geometry; the 14px counter is part of the 40px avatar stack, not ordinary metadata or an action.

import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { getAgentDisplayName } from "@/lib/agent-display-name";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { AgentPrivateParticipant } from "@/types/agent/private-domain";
import { cn } from "@/shared/ui/class-name";

export function PrivateParticipantAvatarStack({
  ownerAgentId: ownerAgentId,
  participants,
  t,
}: {
  ownerAgentId: string;
  participants: AgentPrivateParticipant[];
  t: I18nContextValue["t"];
}) {
  const peers = participants.filter((participant) => participant.agent_id !== ownerAgentId);
  const stackParticipants = peers.length ? peers : participants;
  const isGroup = stackParticipants.length > 1;
  const visible = stackParticipants.slice(0, isGroup ? 2 : 1);
  const overflowCount = Math.max(stackParticipants.length - visible.length, 0);
  return (
    <div className="relative flex h-9 w-10 shrink-0 items-center justify-start">
      {visible.map((participant, index) => (
        <span
          className={cn(index > 0 && "-ml-2")}
          key={participant.agent_id}
          style={{ zIndex: 10 - index }}
        >
          <PrivateParticipantAvatar name={getAgentDisplayName(participant.name, t)} participant={participant} size={isGroup ? "stack" : "md"} />
        </span>
      ))}
      {overflowCount > 0 ? (
        <span className="absolute bottom-0 right-0 z-20 flex h-3.5 min-w-3.5 items-center justify-center rounded-full border border-(--surface-elevated-background) bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_68%,transparent)] px-0.5 text-[8px] font-semibold leading-none text-(--text-soft)">
          +{overflowCount}
        </span>
      ) : null}
    </div>
  );
}

export function PrivateParticipantAvatar({
  name,
  participant,
  size,
}: {
  name: string;
  participant?: AgentPrivateParticipant;
  size: "sm" | "stack" | "md";
}) {
  const avatarSize = size === "md" ? "sm" : "xs";
  return (
    <UiAgentAvatar
      avatar={participant?.avatar}
      className={size === "sm" ? "h-5 w-5" : size === "stack" ? "h-6 w-6" : "h-8 w-8"}
      name={name}
      size={avatarSize}
    />
  );
}
