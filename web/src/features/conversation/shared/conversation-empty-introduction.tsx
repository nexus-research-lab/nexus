"use client";

import {
  Bot,
  Clock3,
  FilePenLine,
  ListChecks,
  MessagesSquare,
  Plug,
  SlidersHorizontal,
  UsersRound,
  Wrench,
  Workflow,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";

interface ConversationEmptyIntroductionProps {
  agentAvatar?: string | null;
  agentName?: string | null;
  isMain?: boolean;
  kind: "dm" | "room";
  onSelect: (prompt: string) => void;
}

const EMPTY_SUGGESTIONS = {
  main: [
    { icon: Bot, key: "conversation.empty_main_agents" },
    { icon: UsersRound, key: "conversation.empty_main_rooms" },
    { icon: SlidersHorizontal, key: "conversation.empty_main_providers" },
    { icon: Wrench, key: "conversation.empty_main_troubleshoot" },
  ],
  agent: [
    { icon: FilePenLine, key: "conversation.empty_agent_workspace" },
    { icon: Plug, key: "conversation.empty_agent_capabilities" },
    { icon: Workflow, key: "conversation.empty_agent_workgraph" },
    { icon: Clock3, key: "conversation.empty_agent_automation" },
  ],
  room: [
    { icon: UsersRound, key: "conversation.empty_room_collaborate" },
    { icon: ListChecks, key: "conversation.empty_room_delegate" },
    { icon: Workflow, key: "conversation.empty_room_workgraph" },
    { icon: FilePenLine, key: "conversation.empty_room_workspace" },
  ],
} as const;

export function ConversationEmptyIntroduction({
  agentAvatar,
  agentName,
  isMain = false,
  kind,
  onSelect,
}: ConversationEmptyIntroductionProps) {
  const { t } = useI18n();
  const name = agentName?.trim() || "Nexus";
  const variant = kind === "room" ? "room" : isMain ? "main" : "agent";
  const title = variant === "room"
    ? t("conversation.empty_room_title")
    : variant === "main"
    ? t("conversation.empty_main_title")
    : t("conversation.empty_dm_title", { name });

  return (
    <section
      aria-label={title}
      className="flex min-h-[clamp(22rem,58vh,42rem)] items-center justify-center px-3 py-12"
      data-conversation-empty-introduction
    >
      <div className="w-full max-w-[46rem]">
        <div className="flex justify-center">
          {kind === "dm" ? (
            <UiAgentAvatar avatar={agentAvatar} name={name} size="lg" />
          ) : (
            <span className="flex h-14 w-14 items-center justify-center rounded-[12px] border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-muted) shadow-(--surface-avatar-shadow)">
              <MessagesSquare className="h-6 w-6" />
            </span>
          )}
        </div>
        <h2 className="mt-6 text-center text-xl font-medium text-(--text-strong)">
          {title}
        </h2>
        <div className="mt-8 grid gap-3 sm:grid-cols-2">
          {EMPTY_SUGGESTIONS[variant].map(({ icon: Icon, key }) => {
            const label = t(key);
            return (
              <button
                className="group flex min-h-24 flex-col items-start justify-between rounded-xl border border-(--divider-subtle-color) bg-transparent p-4 text-left transition-colors hover:border-(--surface-control-border) hover:bg-(--surface-control-background) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--primary)"
                key={key}
                type="button"
                onClick={() => onSelect(label)}
              >
                <Icon className="h-4 w-4 text-(--icon-muted) transition-colors group-hover:text-(--text-default)" />
                <span className="mt-4 text-sm leading-5 text-(--text-default)">
                  {label}
                </span>
              </button>
            );
          })}
        </div>
      </div>
    </section>
  );
}
