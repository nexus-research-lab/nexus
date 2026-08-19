/**
 * [INPUT]: 产品想法、产品经理 Agent、创建状态与确认命令。
 * [OUTPUT]: 可核对协作成员、执行步骤和最终产出的 Room 方案卡片。
 * [POS]: 产品经理 Room 任务的事务确认边界。
 */

import {
  Bot,
  CheckCircle2,
  LoaderCircle,
  RefreshCcw,
  Route,
  UsersRound,
} from "lucide-react";

import { getUiButtonClassName } from "@/shared/ui/button/button-styles";

import type { HomeOnboardingRoomTaskDraft } from "./home-onboarding-room-task";

interface HomeOnboardingRoomPlanCardProps {
  draft: HomeOnboardingRoomTaskDraft;
  isCreating: boolean;
  onConfirm: () => void;
  onRestart: () => void;
}

export function HomeOnboardingRoomPlanCard({
  draft,
  isCreating,
  onConfirm,
  onRestart,
}: HomeOnboardingRoomPlanCardProps) {
  const members = [
    [draft.productManagerAgentName, "产品经理 · 收敛最终方案"],
    [draft.researcherAgentName, "用户研究 · 校验用户问题"],
    [draft.technicalReviewerAgentName, "技术评审 · 评估实现风险"],
  ];

  return (
    <section className="nexus-onboarding-provider-card mx-auto mb-5 mt-1 w-full max-w-[760px] overflow-hidden rounded-[20px] border border-(--surface-panel-border) bg-(--surface-panel-background) shadow-[0_18px_52px_color-mix(in_srgb,var(--primary)_14%,transparent)]">
      <div className="border-b border-(--divider-subtle-color) bg-[linear-gradient(135deg,color-mix(in_srgb,var(--primary)_14%,transparent),transparent_60%)] px-5 py-5 sm:px-6">
        <div className="flex items-start gap-3.5">
          <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-[color-mix(in_srgb,var(--primary)_14%,transparent)] text-primary">
            <UsersRound className="h-5 w-5" />
          </div>
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-primary">
              Room 协作方案
            </p>
            <h3 className="mt-1 text-[18px] font-semibold text-(--text-strong)">
              {draft.roomName}
            </h3>
            <p className="mt-1 text-[12px] leading-5 text-(--text-muted)">
              评审主题：{draft.productIdea}
            </p>
          </div>
        </div>
      </div>

      <div className="space-y-5 px-5 py-5 sm:px-6">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
            协作成员
          </p>
          <div className="mt-2 grid gap-2 sm:grid-cols-3">
            {members.map(([name, responsibility]) => (
              <div
                className="rounded-2xl border border-(--divider-subtle-color) bg-[color-mix(in_srgb,var(--background)_58%,transparent)] p-3"
                key={name}
              >
                <div className="flex items-center gap-2 text-[12px] font-semibold text-(--text-strong)">
                  <Bot className="h-3.5 w-3.5 text-primary" />
                  {name}
                </div>
                <p className="mt-1.5 text-[11px] leading-5 text-(--text-muted)">
                  {responsibility}
                </p>
              </div>
            ))}
          </div>
        </div>

        <div className="rounded-2xl border border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color))] bg-[color-mix(in_srgb,var(--primary)_6%,transparent)] p-4">
          <div className="flex items-center gap-2 text-[12px] font-semibold text-(--text-strong)">
            <Route className="h-4 w-4 text-primary" />
            两阶段真实协作
          </div>
          <ol className="mt-2.5 space-y-2 text-[12px] leading-5 text-(--text-muted)">
            <li>1. 用户研究与技术评审并行输出各自结论</li>
            <li>2. 产品经理读取两方观点，收敛需求范围、风险和验收标准</li>
          </ol>
        </div>

        <div className="grid gap-2 text-[12px] text-(--text-muted) sm:grid-cols-2">
          <span className="flex items-center gap-2">
            <CheckCircle2 className="h-4 w-4 text-primary" />
            自动补齐两位协作 Agent
          </span>
          <span className="flex items-center gap-2">
            <CheckCircle2 className="h-4 w-4 text-primary" />
            使用已配置的默认对话模型
          </span>
        </div>

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            className={getUiButtonClassName({
              size: "sm",
              tone: "default",
              variant: "ghost",
            })}
            disabled={isCreating}
            onClick={onRestart}
            type="button"
          >
            <RefreshCcw className="h-4 w-4" />
            重新描述想法
          </button>
          <button
            className={getUiButtonClassName({
              size: "sm",
              tone: "primary",
              variant: "solid",
            })}
            disabled={isCreating}
            onClick={onConfirm}
            type="button"
          >
            {isCreating ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <UsersRound className="h-4 w-4" />
            )}
            {isCreating ? "正在创建 Room…" : "确认方案并创建 Room"}
          </button>
        </div>
      </div>
    </section>
  );
}
