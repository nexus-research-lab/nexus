"use client";

import { ArrowRight, Bot, MessageSquareText } from "lucide-react";

import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace-status-badge";

interface ContactsAgentCardProps {
  description: string;
  is_selected: boolean;
  model_label: string;
  name: string;
  status_tone: "active" | "running" | "idle";
  status_label: string;
  on_open_profile: () => void;
  on_open_room: () => void;
}

export function ContactsAgentCard({
  description,
  is_selected,
  model_label,
  name,
  status_tone,
  status_label,
  on_open_profile,
  on_open_room,
}: ContactsAgentCardProps) {
  return (
    <article
      className={`rounded-[26px] border px-6 py-5 transition-all ${
        is_selected
          ? "workspace-card-strong border-sky-300/36 shadow-[0_18px_34px_rgba(102,112,145,0.14)]"
          : "workspace-card border-white/24 hover:border-white/30 hover:bg-white/34"
      }`}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex min-w-0 items-center gap-4">
          <div className="workspace-chip flex h-18 w-18 shrink-0 items-center justify-center rounded-[20px] text-slate-900/82">
            <Bot className="h-7 w-7 text-slate-900/88" />
          </div>
          <div className="min-w-0">
            <p className="truncate text-[28px] font-bold tracking-[-0.04em] text-slate-950/92">{name}</p>
            <p className="mt-1 text-[12px] font-semibold uppercase tracking-[0.18em] text-sky-600/90">
              {model_label}
            </p>
          </div>
        </div>

        <WorkspaceStatusBadge label={status_label} tone={status_tone} />
      </div>

      <p className="mt-5 min-h-[72px] text-[15px] leading-7 text-slate-700/78">
        {description}
      </p>

      <div className="mt-6 flex items-center gap-3">
        <WorkspacePillButton onClick={on_open_profile}>
          View Details
        </WorkspacePillButton>
        <WorkspacePillButton onClick={on_open_room} variant="strong">
          <MessageSquareText className="h-4 w-4" />
          Message
          <ArrowRight className="h-4 w-4" />
        </WorkspacePillButton>
      </div>
    </article>
  );
}
