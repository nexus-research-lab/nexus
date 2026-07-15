"use client";

import type { ReactNode } from "react";

import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { cn } from "@/shared/ui/class-name";

export interface AgentMentionDirectory {
  avatars?: Readonly<Record<string, string | null>>;
  names?: Readonly<Record<string, string>>;
}

interface AgentMentionChipProps {
  agentId: string;
	children: ReactNode;
  directory?: AgentMentionDirectory;
  onOpenAgentContact?: (agentId: string) => void;
}

export function AgentMentionChip({
  agentId,
  children,
  directory,
  onOpenAgentContact,
}: AgentMentionChipProps) {
  const label = directory?.names?.[agentId] ?? String(children);
  const avatar = directory?.avatars?.[agentId] ?? null;
  const handleClick = () => onOpenAgentContact?.(agentId);
  const interactive = Boolean(onOpenAgentContact);
  return (
    <button
      aria-label={`打开 ${label} 的联络`}
      className={cn(
        "mx-0.5 inline-flex max-w-full items-center gap-1 rounded-full border px-1.5 py-0.5 align-middle text-[0.9em] font-medium leading-none transition-colors",
        "border-primary/20 bg-primary/8 text-primary",
        interactive && "cursor-pointer hover:border-primary/40 hover:bg-primary/14",
        !interactive && "cursor-default",
      )}
      disabled={!interactive}
      onClick={handleClick}
      type="button"
    >
      <UiAgentAvatar
        avatar={avatar}
        className="h-4 w-4 border-0 shadow-none"
        name={label}
        size="xs"
      />
      <span className="truncate">{children}</span>
    </button>
  );
}
