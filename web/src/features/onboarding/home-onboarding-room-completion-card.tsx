/**
 * [INPUT]: 已完成 Room 任务的草稿与结束新手引导命令。
 * [OUTPUT]: Nexus DM 中可跳转 Room 的第二个里程碑完成卡片。
 * [POS]: 产品经理角色新手任务的最终交付与引导收口。
 */

import {
  ArrowUpRight,
  BadgeCheck,
  CheckCircle2,
  PartyPopper,
  UsersRound,
} from "lucide-react";
import { Link } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";

import type { HomeOnboardingRoomTaskDraft } from "./home-onboarding-room-task";

interface HomeOnboardingRoomCompletionCardProps {
  draft: HomeOnboardingRoomTaskDraft;
  onFinish: () => void;
}

export function HomeOnboardingRoomCompletionCard({
  draft,
  onFinish,
}: HomeOnboardingRoomCompletionCardProps) {
  return (
    <section className="nexus-onboarding-provider-card mx-auto mb-5 mt-1 w-full max-w-[760px] overflow-hidden rounded-[22px] border border-[color:color-mix(in_srgb,var(--primary)_34%,var(--surface-panel-border))] bg-(--surface-panel-background) shadow-[0_24px_72px_color-mix(in_srgb,var(--primary)_20%,transparent)]">
      <div className="bg-[radial-gradient(circle_at_84%_0%,color-mix(in_srgb,var(--primary)_25%,transparent),transparent_42%),linear-gradient(135deg,color-mix(in_srgb,var(--primary)_14%,transparent),transparent_66%)] px-5 py-5 sm:px-6 sm:py-6">
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-start gap-3.5">
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-[18px] bg-[color-mix(in_srgb,var(--primary)_16%,transparent)] text-primary ring-4 ring-[color:color-mix(in_srgb,var(--primary)_8%,transparent)]">
              <UsersRound className="h-6 w-6" />
            </div>
            <div>
              <div className="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.14em] text-primary">
                <BadgeCheck className="h-3.5 w-3.5" />
                任务 2 / 2 已完成
              </div>
              <h3 className="mt-1.5 text-[20px] font-semibold text-(--text-strong)">
                {draft.roomName}
              </h3>
              <p className="mt-1 text-[12px] text-(--text-muted)">
                跨角色产品评审 · 真实 Room 协作
              </p>
            </div>
          </div>
          <PartyPopper className="h-6 w-6 shrink-0 text-primary" />
        </div>
      </div>

      <div className="space-y-4 px-5 py-5 sm:px-6">
        <p className="text-[13px] leading-6 text-(--text-default)">
          围绕「{draft.productIdea}」，三位 Agent 已完成用户价值、技术可行性与产品范围的评审，并由 {draft.productManagerAgentName} 收敛最终结论。
        </p>

        <div className="grid gap-2 text-[12px] text-(--text-muted) sm:grid-cols-2">
          <span className="flex items-center gap-2">
            <CheckCircle2 className="h-4 w-4 text-primary" />
            已创建真实协作 Room
          </span>
          <span className="flex items-center gap-2">
            <CheckCircle2 className="h-4 w-4 text-primary" />
            评审过程与结论已保留
          </span>
        </div>

        <div className="flex flex-col gap-2 border-t border-(--divider-subtle-color) pt-4 sm:flex-row sm:items-center sm:justify-between">
          <Link
            className={getUiButtonClassName({
              size: "sm",
              tone: "default",
              variant: "ghost",
            })}
            to={AppRouteBuilders.roomConversation(
              draft.roomId,
              draft.conversationId,
            )}
          >
            查看协作 Room
            <ArrowUpRight className="h-4 w-4" />
          </Link>
          <button
            className={getUiButtonClassName({
              size: "sm",
              tone: "primary",
              variant: "solid",
            })}
            onClick={onFinish}
            type="button"
          >
            <PartyPopper className="h-4 w-4" />
            完成新手旅程
          </button>
        </div>
      </div>
    </section>
  );
}
