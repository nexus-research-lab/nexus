import { UiAgentAvatar } from "@/shared/ui/avatar";
import { AgentPrivateParticipant } from "@/types/agent/private-domain";
import { cn } from "@/lib/utils";

export function PrivateParticipantAvatarStack({
  owner_agent_id,
  participants,
}: {
  owner_agent_id: string;
  participants: AgentPrivateParticipant[];
}) {
  const peers = participants.filter((participant) => participant.agent_id !== owner_agent_id);
  const stack_participants = peers.length ? peers : participants;
  const is_group = stack_participants.length > 1;
  const visible = stack_participants.slice(0, is_group ? 2 : 1);
  const overflow_count = Math.max(stack_participants.length - visible.length, 0);
  return (
    <div className="relative flex h-9 w-10 shrink-0 items-center justify-start">
      {visible.map((participant, index) => (
        <span
          className={cn(index > 0 && "-ml-2")}
          key={participant.agent_id}
          style={{ zIndex: 10 - index }}
        >
          <PrivateParticipantAvatar participant={participant} size={is_group ? "stack" : "md"} />
        </span>
      ))}
      {overflow_count > 0 ? (
        <span className="absolute bottom-0 right-0 z-20 flex h-3.5 min-w-3.5 items-center justify-center rounded-full border border-(--surface-elevated-background) bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_68%,transparent)] px-0.5 text-[8px] font-bold leading-none text-(--text-soft)">
          +{overflow_count}
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
  const avatar_size = size === "md" ? "sm" : "xs";
  return (
    <UiAgentAvatar
      avatar={participant?.avatar}
      class_name={size === "sm" ? "h-5 w-5" : size === "stack" ? "h-6 w-6" : "h-8 w-8"}
      name={participant?.name || participant?.agent_id || "Agent"}
      size={avatar_size}
    />
  );
}
