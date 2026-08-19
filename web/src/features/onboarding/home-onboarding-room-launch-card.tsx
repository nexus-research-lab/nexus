/**
 * [INPUT]: 已创建 Room 的草稿、启动路由和阶段切换命令。
 * [OUTPUT]: 从 Nexus DM 进入真实 Room 并自动发布评审任务的入口卡片。
 * [POS]: Room 创建完成与协作执行之间的跨页面交接点。
 */

import { ArrowRight, DoorOpen, Sparkles, UsersRound } from "lucide-react";
import { Link } from "react-router-dom";

import { getUiButtonClassName } from "@/shared/ui/button/button-styles";

import {
  buildHomeOnboardingRoomLaunchRoute,
  type HomeOnboardingRoomTaskDraft,
} from "./home-onboarding-room-task";

interface HomeOnboardingRoomLaunchCardProps {
  draft: HomeOnboardingRoomTaskDraft;
  onLaunch: () => void;
  resume: boolean;
}

export function HomeOnboardingRoomLaunchCard({
  draft,
  onLaunch,
  resume,
}: HomeOnboardingRoomLaunchCardProps) {
  return (
    <section className="nexus-onboarding-provider-card mx-auto mb-5 mt-1 w-full max-w-[760px] overflow-hidden rounded-[20px] border border-[color:color-mix(in_srgb,var(--primary)_30%,var(--surface-panel-border))] bg-(--surface-panel-background) shadow-[0_22px_64px_color-mix(in_srgb,var(--primary)_18%,transparent)]">
      <div className="bg-[radial-gradient(circle_at_86%_0%,color-mix(in_srgb,var(--primary)_22%,transparent),transparent_40%),linear-gradient(135deg,color-mix(in_srgb,var(--primary)_12%,transparent),transparent_65%)] px-5 py-5 sm:px-6 sm:py-6">
        <div className="flex items-start gap-3.5">
          <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-[18px] bg-[color-mix(in_srgb,var(--primary)_15%,transparent)] text-primary">
            <DoorOpen className="h-5 w-5" />
          </div>
          <div>
            <div className="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.14em] text-primary">
              <Sparkles className="h-3.5 w-3.5" />
              Room 已准备就绪
            </div>
            <h3 className="mt-1.5 text-[19px] font-semibold text-(--text-strong)">
              {draft.roomName}
            </h3>
            <p className="mt-1.5 text-[12px] leading-5 text-(--text-muted)">
              {resume
                ? "协作流程仍在这个 Room 中，返回后由 Room 的真实模型服务继续推进。"
                : "进入后 Nexus 主持人会发布固定评审任务；从顾问评审开始，所有协作接力与内容输出都由真实模型完成。"}
            </p>
          </div>
        </div>

        <div className="mt-4 flex flex-col gap-3 border-t border-(--divider-subtle-color) pt-4 sm:flex-row sm:items-center sm:justify-between">
          <span className="flex items-center gap-2 text-[12px] text-(--text-muted)">
            <UsersRound className="h-4 w-4 text-primary" />
            Nexus 主持人 + 3 位协作 Agent
          </span>
          <Link
            className={getUiButtonClassName({
              size: "sm",
              tone: "primary",
              variant: "solid",
            })}
            onClick={onLaunch}
            to={buildHomeOnboardingRoomLaunchRoute(draft, !resume)}
          >
            {resume ? "返回协作 Room" : "进入 Room 开始协作"}
            <ArrowRight className="h-4 w-4" />
          </Link>
        </div>
      </div>
    </section>
  );
}
