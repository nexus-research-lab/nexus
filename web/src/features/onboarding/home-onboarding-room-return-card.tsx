/**
 * [INPUT]: 已完成两阶段协作的 Room 草稿、原 Nexus DM 路径与返回命令。
 * [OUTPUT]: Room 消息流末尾的协作完成卡片。
 * [POS]: 真实 Room 执行结束后返回 Nexus Agent 的唯一交互出口。
 */

import { ArrowLeft, BadgeCheck, FileCheck2, UsersRound } from "lucide-react";
import { Link } from "react-router-dom";

import { getUiButtonClassName } from "@/shared/ui/button/button-styles";

import type { HomeOnboardingRoomTaskDraft } from "./home-onboarding-room-task";

interface HomeOnboardingRoomReturnCardProps {
  draft: HomeOnboardingRoomTaskDraft;
  onReturn: () => void;
  returnPath: string;
}

export function HomeOnboardingRoomReturnCard({
  draft,
  onReturn,
  returnPath,
}: HomeOnboardingRoomReturnCardProps) {
  return (
    <section className="nexus-onboarding-provider-card mx-auto mb-5 mt-4 w-full max-w-[760px] overflow-hidden rounded-[22px] border border-[color:color-mix(in_srgb,var(--primary)_34%,var(--surface-panel-border))] bg-(--surface-panel-background) shadow-[0_24px_72px_color-mix(in_srgb,var(--primary)_20%,transparent)]">
      <div className="bg-[radial-gradient(circle_at_82%_0%,color-mix(in_srgb,var(--primary)_24%,transparent),transparent_40%),linear-gradient(135deg,color-mix(in_srgb,var(--primary)_13%,transparent),transparent_65%)] px-5 py-5 sm:px-6 sm:py-6">
        <div className="flex items-start gap-3.5">
          <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-[color-mix(in_srgb,var(--primary)_16%,transparent)] text-primary ring-4 ring-[color:color-mix(in_srgb,var(--primary)_8%,transparent)]">
            <BadgeCheck className="h-6 w-6" />
          </div>
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-primary">
              协作执行完成
            </p>
            <h3 className="mt-1.5 text-[19px] font-semibold text-(--text-strong)">
              {draft.roomName}已产出评审结论
            </h3>
            <p className="mt-1.5 text-[12px] leading-5 text-(--text-muted)">
              用户研究、技术评审和产品经理的结果已经保存在当前 Room 中。
            </p>
          </div>
        </div>

        <div className="mt-4 grid gap-2 rounded-2xl border border-(--divider-subtle-color) bg-[color-mix(in_srgb,var(--background)_56%,transparent)] p-3.5 text-[12px] text-(--text-muted) sm:grid-cols-2">
          <span className="flex items-center gap-2">
            <UsersRound className="h-4 w-4 text-primary" />
            3 个角色完成协作
          </span>
          <span className="flex items-center gap-2">
            <FileCheck2 className="h-4 w-4 text-primary" />
            最终评审结论已生成
          </span>
        </div>

        <div className="mt-4 flex justify-end">
          <Link
            className={getUiButtonClassName({
              size: "sm",
              tone: "primary",
              variant: "solid",
            })}
            onClick={onReturn}
            to={returnPath}
          >
            <ArrowLeft className="h-4 w-4" />
            返回 Nexus Agent 查看完成卡片
          </Link>
        </div>
      </div>
    </section>
  );
}
