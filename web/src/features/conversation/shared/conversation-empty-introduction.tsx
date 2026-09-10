/**
 * INPUT: 空会话身份、会话类型与建议文本选择动作。
 * OUTPUT: 居中的静态介绍与紧凑快捷建议，捕获建议发送失败并交给宿主诊断。
 * POS: DM/Room canonical timeline 为空时的前端展示，不创建消息或 runtime round。
 */
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

import { notifyDesktopDiagnostic } from "@/config/desktop-runtime";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { WorkspaceIconFrame } from "@/shared/ui/workspace/catalog/workspace-icon-frame";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface ConversationEmptyIntroductionProps {
  agentAvatar?: string | null;
  agentName?: string | null;
  isMain?: boolean;
  kind: "dm" | "room";
  onSelect: (prompt: string) => void | Promise<void>;
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
    { icon: FilePenLine, key: "conversation.empty_room_workspace" },
    { icon: Workflow, key: "conversation.empty_room_workgraph" },
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

  const selectSuggestion = async (prompt: string) => {
    try {
      await onSelect(prompt);
    } catch (error) {
      // 发送层负责可靠性提示和消息保留；快捷入口只收口事件 Promise。
      console.error("[ConversationEmptyIntroduction] 快捷建议发送失败", error);
      notifyDesktopDiagnostic("conversation.suggestion_failed", { kind }, error);
    }
  };

  return (
    <section
      aria-label={title}
      className="flex min-h-[22rem] flex-1 items-center justify-center px-3 py-10 sm:py-12"
      data-conversation-empty-introduction
    >
      <div className="w-full max-w-[46rem]">
        <div className="flex justify-center">
          {kind === "dm" ? (
            <UiAgentAvatar avatar={agentAvatar} name={name} size="lg" />
          ) : (
            <WorkspaceIconFrame size="lg">
              <MessagesSquare aria-hidden className="-translate-x-0.5 h-6 w-6" />
            </WorkspaceIconFrame>
          )}
        </div>
        <h2 className={`mx-auto mt-5 max-w-[32rem] text-balance text-center ${getUiTypographyClassName({
          role: "featureTitle",
          tone: "strong",
          weight: "medium",
        })}`}>
          {title}
        </h2>
        <div className="mt-8 grid gap-3 sm:grid-cols-2">
          {EMPTY_SUGGESTIONS[variant].map(({ icon: Icon, key }) => {
            const label = t(key);
            return (
              <UiButton
                className="group min-h-24 w-full flex-col items-start justify-between text-left"
                key={key}
                onClick={() => { void selectSuggestion(label); }}
                size="lg"
                variant="outline"
              >
                <Icon className="h-4 w-4 text-(--icon-muted) transition-colors group-hover:text-(--text-default)" />
                <span className={`mt-4 ${getUiTypographyClassName({
                  role: "supporting",
                  tone: "default",
                })}`}>
                  {label}
                </span>
              </UiButton>
            );
          })}
        </div>
      </div>
    </section>
  );
}
