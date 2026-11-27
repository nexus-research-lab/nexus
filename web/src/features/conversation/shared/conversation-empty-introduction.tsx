/**
 * INPUT: 空会话身份、会话类型与建议文本选择动作。
 * OUTPUT: 居中的静态介绍与单行快捷建议，捕获建议发送失败并交给宿主诊断；主智能体只显示标题，起手建议由首页启动台提供。
 * POS: DM/Room canonical timeline 为空时的前端展示，不创建消息或 runtime round。
 */
"use client";

import {
  Clock3,
  FilePenLine,
  ListChecks,
  Plug,
  UsersRound,
  Workflow,
} from "lucide-react";

import { notifyDesktopDiagnostic } from "@/config/desktop-runtime";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
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
  main: [],
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
        {kind === "dm" ? (
          <div className="mb-5 flex justify-center">
            <UiAgentAvatar avatar={agentAvatar} name={name} size="lg" />
          </div>
        ) : null}
        <h2 className={`mx-auto max-w-[32rem] text-balance text-center ${getUiTypographyClassName({
          role: "featureTitle",
          tone: "strong",
          weight: "medium",
        })}`}>
          {title}
        </h2>
        {EMPTY_SUGGESTIONS[variant].length > 0 ? (
        <div className="mt-8 grid gap-2 sm:grid-cols-2">
          {EMPTY_SUGGESTIONS[variant].map(({ icon: Icon, key }) => {
            const label = t(key);
            return (
              <UiButton
                className="group w-full justify-start gap-3 text-left"
                key={key}
                onClick={() => { void selectSuggestion(label); }}
                size="lg"
                variant="outline"
              >
                <Icon className="h-4 w-4 shrink-0 text-(--icon-muted) transition-colors group-hover:text-(--text-default)" />
                <span className={`min-w-0 truncate ${getUiTypographyClassName({
                  role: "supporting",
                  tone: "default",
                })}`}>
                  {label}
                </span>
              </UiButton>
            );
          })}
        </div>
        ) : null}
      </div>
    </section>
  );
}
