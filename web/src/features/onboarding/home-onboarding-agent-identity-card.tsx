/**
 * [INPUT]: 已真实创建的 Agent ID 与引导草案。
 * [OUTPUT]: 可跳转至 Nexus Agent 身份页的完成里程碑卡片。
 * [POS]: 专属 Agent 创建任务的完成反馈和后续入口。
 */

import { ArrowUpRight, BadgeCheck, Bot, Users } from "lucide-react";
import { Link } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";

import {
  getHomeOnboardingAgentTaskProfile,
  type HomeOnboardingAgentTaskDraft,
} from "./home-onboarding-agent-task";

interface HomeOnboardingAgentIdentityCardProps {
  draft: HomeOnboardingAgentTaskDraft;
}

export function HomeOnboardingAgentIdentityCard({
  draft,
}: HomeOnboardingAgentIdentityCardProps) {
  const profile = getHomeOnboardingAgentTaskProfile(draft.role);

  return (
    <Link
      aria-label={`查看 ${draft.name} 的 Agent 身份页`}
      className="nexus-onboarding-provider-card group mx-auto mb-5 mt-1 block w-full max-w-[760px] overflow-hidden rounded-[22px] border border-[color:color-mix(in_srgb,var(--primary)_30%,var(--surface-panel-border))] bg-(--surface-panel-background) shadow-[0_22px_64px_color-mix(in_srgb,var(--primary)_18%,transparent)] transition-[border-color,box-shadow,transform] duration-300 hover:-translate-y-1 hover:border-[color:color-mix(in_srgb,var(--primary)_52%,var(--surface-panel-border))] hover:shadow-[0_28px_74px_color-mix(in_srgb,var(--primary)_24%,transparent)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
      to={AppRouteBuilders.contactAgent(draft.createdAgentId)}
    >
      <div className="bg-[radial-gradient(circle_at_82%_0%,color-mix(in_srgb,var(--primary)_22%,transparent),transparent_38%),linear-gradient(135deg,color-mix(in_srgb,var(--primary)_13%,transparent),transparent_64%)] px-5 py-5 sm:px-6 sm:py-6">
        <div className="flex items-start justify-between gap-4">
          <div className="flex min-w-0 items-center gap-4">
            <UiAgentAvatar
              className="ring-4 ring-[color:color-mix(in_srgb,var(--primary)_10%,transparent)]"
              name={draft.name}
              shape="rounded"
              size="xl"
            />
            <div className="min-w-0">
              <div className="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.14em] text-primary">
                <BadgeCheck className="h-3.5 w-3.5" />
                任务 1 / 2 已完成
              </div>
              <h3 className="mt-1.5 truncate text-[20px] font-semibold text-(--text-strong)">
                {draft.name}
              </h3>
              <p className="mt-1 text-[12px] text-(--text-muted)">
                {profile.agentType} · 已加入 Nexus 联系人
              </p>
            </div>
          </div>
          <ArrowUpRight className="h-5 w-5 shrink-0 text-primary transition-transform duration-200 group-hover:-translate-y-0.5 group-hover:translate-x-0.5" />
        </div>
      </div>

      <div className="px-5 py-5 sm:px-6">
        <p className="text-[13px] leading-6 text-(--text-default)">
          {draft.description}
        </p>
        <div className="mt-3 flex flex-wrap gap-1.5">
          {draft.vibeTags.map((tag) => (
            <span
              className="rounded-full bg-[color-mix(in_srgb,var(--primary)_9%,transparent)] px-2.5 py-1 text-[11px] font-medium text-primary"
              key={tag}
            >
              {tag}
            </span>
          ))}
        </div>

        <div className="mt-5 flex flex-col gap-2 border-t border-(--divider-subtle-color) pt-4 text-[12px] text-(--text-muted) sm:flex-row sm:items-center sm:justify-between">
          <span className="flex items-center gap-2">
            <Bot className="h-4 w-4 text-primary" />
            点击卡片查看并继续配置 Agent
          </span>
          <span className="flex items-center gap-2">
            <Users className="h-4 w-4 text-primary" />
            下一步：创建{profile.roomName}
          </span>
        </div>
      </div>
    </Link>
  );
}
