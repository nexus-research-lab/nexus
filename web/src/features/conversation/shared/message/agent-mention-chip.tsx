/**
 * INPUT: Agent mention 身份、目录、联系人动作与可选 handoff_id。
 * OUTPUT: 单一可点击 mention chip 及其原位交接阶段。
 * POS: Markdown mention 的共享视觉边界，不创建 Agent execution 卡片。
 */
"use client";

import type { ReactNode } from "react";

import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  type AgentHandoffPhase,
  useAgentHandoffStatus,
} from "./agent-handoff-status-context";

export interface AgentMentionDirectory {
  avatars?: Readonly<Record<string, string | null>>;
  names?: Readonly<Record<string, string>>;
}

interface AgentMentionChipProps {
  agentId: string;
  children: ReactNode;
  directory?: AgentMentionDirectory;
  handoffId?: string;
  onOpenAgentContact?: (agentId: string) => void;
}

const HANDOFF_STATUS_LABEL = {
  active: "room.agent_handoff_active",
  preparing: "room.agent_handoff_preparing",
  queued: "room.agent_handoff_queued",
} as const satisfies Record<AgentHandoffPhase, string>;

export function AgentMentionChip({
  agentId,
  children,
  directory,
  handoffId,
  onOpenAgentContact,
}: AgentMentionChipProps) {
  const { t } = useI18n();
  const label = directory?.names?.[agentId] ?? String(children);
  const avatar = directory?.avatars?.[agentId] ?? null;
  const handoffStatus = useAgentHandoffStatus(handoffId);
  const handoffLabel = handoffStatus
    ? t(HANDOFF_STATUS_LABEL[handoffStatus])
    : null;
  const handleClick = () => onOpenAgentContact?.(agentId);
  const interactive = Boolean(onOpenAgentContact);
  return (
    <button
      aria-label={[
        t("room.agent_contact_open", { name: label }),
        handoffLabel,
      ].filter(Boolean).join("，")}
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
      {handoffStatus && handoffLabel ? (
        <span
          aria-live="polite"
          className="ml-0.5 inline-flex shrink-0 items-center gap-1 border-l border-current/15 pl-1.5 text-[0.78em] font-normal opacity-75"
          role="status"
        >
          <span
            aria-hidden="true"
            className={cn(
              "h-1.5 w-1.5 rounded-full bg-current",
              handoffStatus === "preparing" && "animate-pulse",
            )}
          />
          {handoffLabel}
        </span>
      ) : null}
    </button>
  );
}
