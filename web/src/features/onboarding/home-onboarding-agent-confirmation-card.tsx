/**
 * [INPUT]: 已收集的 Agent 草案、创建状态与确认命令。
 * [OUTPUT]: 创建前可核对并确认的 Agent 配置摘要卡片。
 * [POS]: 专属 Agent 创建任务的事务确认边界。
 */

import { Bot, Check, LoaderCircle, RefreshCcw, ShieldCheck } from "lucide-react";

import { getUiButtonClassName } from "@/shared/ui/button/button-styles";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";

import type {
  HomeOnboardingAgentTaskDraft,
} from "./home-onboarding-agent-task";

interface HomeOnboardingAgentConfirmationCardProps {
  draft: HomeOnboardingAgentTaskDraft;
  isCreating: boolean;
  onConfirm: () => void;
  onRestart: () => void;
}

export function HomeOnboardingAgentConfirmationCard({
  draft,
  isCreating,
  onConfirm,
  onRestart,
}: HomeOnboardingAgentConfirmationCardProps) {
  return (
    <section
      aria-labelledby="home-onboarding-agent-confirmation-title"
      className="nexus-onboarding-provider-card mx-auto mb-5 mt-1 w-full max-w-[760px] overflow-hidden rounded-[20px] border border-(--surface-panel-border) bg-(--surface-panel-background) shadow-[0_18px_52px_color-mix(in_srgb,var(--primary)_14%,transparent)]"
    >
      <div className="border-b border-(--divider-subtle-color) bg-[linear-gradient(135deg,color-mix(in_srgb,var(--primary)_14%,transparent),transparent_60%)] px-5 py-5 sm:px-6">
        <div className="flex items-center gap-3.5">
          <UiAgentAvatar name={draft.name} shape="rounded" size="lg" />
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-primary">
              <Bot className="h-3.5 w-3.5" />
              Agent 草案
            </div>
            <h3
              className="mt-1 truncate text-[18px] font-semibold text-(--text-strong)"
              id="home-onboarding-agent-confirmation-title"
            >
              {draft.name}
            </h3>
            <p className="mt-1 text-[12px] text-(--text-muted)">
              面向{draft.role || "你的角色"}的专属智能体
            </p>
          </div>
        </div>
      </div>

      <div className="space-y-4 px-5 py-5 sm:px-6">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
            核心职责
          </p>
          <p className="mt-1.5 text-[13px] leading-6 text-(--text-default)">
            {draft.description}
          </p>
        </div>

        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
            工作风格
          </p>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {draft.vibeTags.map((tag) => (
              <span
                className="rounded-full border border-[color:color-mix(in_srgb,var(--primary)_20%,var(--divider-subtle-color))] bg-[color-mix(in_srgb,var(--primary)_8%,transparent)] px-2.5 py-1 text-[11px] font-medium text-primary"
                key={tag}
              >
                {tag}
              </span>
            ))}
          </div>
        </div>

        <div className="grid gap-2 rounded-2xl border border-(--divider-subtle-color) bg-[color-mix(in_srgb,var(--background)_55%,transparent)] p-3.5 text-[12px] text-(--text-muted) sm:grid-cols-2">
          <span className="flex items-center gap-2">
            <Check className="h-3.5 w-3.5 text-primary" />
            使用当前默认对话模型
          </span>
          <span className="flex items-center gap-2">
            <ShieldCheck className="h-3.5 w-3.5 text-primary" />
            权限沿用 Nexus 安全默认值
          </span>
        </div>

        <div className="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
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
            重新填写
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
              <Bot className="h-4 w-4" />
            )}
            {isCreating ? "正在创建…" : "确认创建智能体"}
          </button>
        </div>
      </div>
    </section>
  );
}
