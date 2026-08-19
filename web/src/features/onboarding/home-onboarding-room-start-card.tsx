/**
 * [INPUT]: 已完成 Agent 任务的草稿与进入 Room 任务的命令。
 * [OUTPUT]: 引导用户开始第二个里程碑的对话卡片。
 * [POS]: Agent 身份卡之后、Room 需求收集之前的衔接入口。
 */

import { ArrowRight, MessagesSquare, UsersRound } from "lucide-react";

import { getUiButtonClassName } from "@/shared/ui/button/button-styles";

import type { HomeOnboardingAgentTaskDraft } from "./home-onboarding-agent-task";

interface HomeOnboardingRoomStartCardProps {
  agentDraft: HomeOnboardingAgentTaskDraft;
  onStart: () => void;
}

export function HomeOnboardingRoomStartCard({
  agentDraft,
  onStart,
}: HomeOnboardingRoomStartCardProps) {
  return (
    <section className="nexus-onboarding-provider-card mx-auto mb-5 mt-1 w-full max-w-[760px] overflow-hidden rounded-[20px] border border-(--surface-panel-border) bg-(--surface-panel-background) shadow-[0_18px_52px_color-mix(in_srgb,var(--primary)_14%,transparent)]">
      <div className="flex flex-col gap-4 bg-[linear-gradient(135deg,color-mix(in_srgb,var(--primary)_13%,transparent),transparent_64%)] px-5 py-5 sm:flex-row sm:items-center sm:justify-between sm:px-6">
        <div className="flex items-start gap-3.5">
          <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-[color-mix(in_srgb,var(--primary)_14%,transparent)] text-primary shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--primary)_22%,transparent)]">
            <UsersRound className="h-5 w-5" />
          </div>
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-primary">
              任务 2 / 2
            </p>
            <h3 className="mt-1 text-[16px] font-semibold text-(--text-strong)">
              创建产品评审 Room
            </h3>
            <p className="mt-1.5 max-w-[470px] text-[12px] leading-5 text-(--text-muted)">
              让「{agentDraft.name || "产品经理 Agent"}」与用户研究、技术评审两个角色完成一次真实协作。
            </p>
          </div>
        </div>
        <button
          className={getUiButtonClassName({
            size: "sm",
            tone: "primary",
            variant: "solid",
          })}
          onClick={onStart}
          type="button"
        >
          <MessagesSquare className="h-4 w-4" />
          开始任务 2
          <ArrowRight className="h-4 w-4" />
        </button>
      </div>
    </section>
  );
}
