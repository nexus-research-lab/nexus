/**
 * INPUT: Agent mention/宿主 handoff reply 身份、目录与可选联系人动作。
 * OUTPUT: 可点击 mention chip 的原位阶段，以及不可点击的 reply 身份 chip。
 * POS: Agent @ 身份的共享视觉边界，reply 不创建 mention、wake 或 execution 卡片。
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
  responded: "room.agent_handoff_responded",
} as const satisfies Record<AgentHandoffPhase, string>;

export function AgentMentionChip({
  agentId,
  children,
  directory,
  handoffId,
  onOpenAgentContact,
}: AgentMentionChipProps) {
  const { t } = useI18n();
  const identity = resolveAgentIdentity(agentId, directory, String(children));
  const handoffStatus = useAgentHandoffStatus(handoffId);
  const handoffLabel = handoffStatus
    ? t(HANDOFF_STATUS_LABEL[handoffStatus])
    : null;
  const handleClick = () => onOpenAgentContact?.(agentId);
  const interactive = Boolean(onOpenAgentContact);
  return (
    <button
      aria-label={[
        t("room.agent_contact_open", { name: identity.label }),
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
        avatar={identity.avatar}
        className="h-4 w-4 border-0 shadow-none"
        name={identity.label}
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

/**
 * 宿主回执只复用 mention 的身份与视觉，不复用其点击、URI 或 handoff 动作。
 */
export function AgentHandoffReplyChip({
  agentId,
  directory,
}: {
  agentId: string;
  directory?: AgentMentionDirectory;
}) {
  const { t } = useI18n();
  const identity = resolveAgentIdentity(
    agentId,
    directory,
    t("message.assistant_fallback"),
  );
  const name = identity.label.replace(/^@+/, "");
  const label = t("room.agent_handoff_reply", { name });
  return (
    <span
      aria-label={label}
      className="inline-flex shrink-0 items-center gap-1 rounded-full border border-primary/20 bg-primary/8 px-1.5 py-0.5 text-[10px] font-medium leading-none text-primary"
      data-handoff-reply="true"
      title={label}
    >
      <UiAgentAvatar
        avatar={identity.avatar}
        className="h-3.5 w-3.5 border-0 shadow-none"
        name={identity.label}
        size="xs"
      />
      <span>{label}</span>
    </span>
  );
}

function resolveAgentIdentity(
  agentId: string,
  directory: AgentMentionDirectory | undefined,
  fallbackLabel: string,
) {
  return {
    avatar: directory?.avatars?.[agentId] ?? null,
    label: directory?.names?.[agentId]?.trim() || fallbackLabel,
  };
}
