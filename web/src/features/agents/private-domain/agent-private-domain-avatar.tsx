import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { AgentPrivateParticipant } from "@/types/agent/private-domain";
import { cn } from "@/shared/ui/class-name";

export function PrivateParticipantAvatarStack({
  ownerAgentId: ownerAgentId,
  participants,
}: {
  ownerAgentId: string;
  participants: AgentPrivateParticipant[];
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
          <PrivateParticipantAvatar participant={participant} size={isGroup ? "stack" : "md"} />
        </span>
      ))}
      {overflowCount > 0 ? (
        <span className="absolute bottom-0 right-0 z-20 flex h-3.5 min-w-3.5 items-center justify-center rounded-full border border-(--surface-elevated-background) bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_68%,transparent)] px-0.5 text-[8px] font-bold leading-none text-(--text-soft)">
          +{overflowCount}
        </span>
      ) : null}
    </div>
  );
}

export function PrivateParticipantAvatar({
  participant,
  size,
}: {
  participant?: AgentPrivateParticipant;
  size: "sm" | "stack" | "md";
}) {
  const avatarSize = size === "md" ? "sm" : "xs";
  return (
    <UiAgentAvatar
      avatar={participant?.avatar}
      className={size === "sm" ? "h-5 w-5" : size === "stack" ? "h-6 w-6" : "h-8 w-8"}
      name={participant?.name || participant?.agent_id || "Agent"}
      size={avatarSize}
    />
  );
}
