/**
 * [INPUT]: Nexus Agent/Room 创建命令投影出的结构化资源产物与可选 Room 开场动作。
 * [OUTPUT]: 样式与跳转行为固定的 Agent 身份卡片或可启动真实协作的 Room 完成卡片。
 * [POS]: Nexus 真实资源创建成功后的统一可点击反馈视图。
 */

import {
  ArrowRight,
  ArrowUpRight,
  BadgeCheck,
  Bot,
  MessagesSquare,
  UsersRound,
} from "lucide-react";
import { Link } from "react-router-dom";

import {
  completeHomeOnboarding,
  isHomeOnboardingCompleted,
} from "@/features/onboarding/home-agent-onboarding";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/display/avatar";
import type { NexusResourceArtifactContent } from "@/types/conversation/message/content";

import { buildNexusResourceArtifactRoute } from "./nexus-resource-artifact-route";

interface NexusResourceArtifactCardProps {
  artifact: NexusResourceArtifactContent;
}

function RoomResourceArtifactCard({
  artifact,
}: NexusResourceArtifactCardProps) {
  const members = artifact.members ?? [];
  const handleOpen = () => {
    if (!isHomeOnboardingCompleted()) {
      completeHomeOnboarding();
    }
  };

  return (
    <Link
      aria-label={`进入 Room：${artifact.name}`}
      className="group my-3 block w-full max-w-[680px] overflow-hidden rounded-[22px] border border-[color:color-mix(in_srgb,var(--primary)_34%,var(--surface-panel-border))] bg-(--surface-panel-background) shadow-[0_20px_60px_color-mix(in_srgb,var(--primary)_16%,transparent)] transition-[border-color,box-shadow,transform] duration-300 hover:-translate-y-1 hover:border-[color:color-mix(in_srgb,var(--primary)_58%,var(--surface-panel-border))] hover:shadow-[0_28px_72px_color-mix(in_srgb,var(--primary)_23%,transparent)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/45"
      data-testid="nexus-room-resource-card"
      onClick={handleOpen}
      to={buildNexusResourceArtifactRoute(artifact)}
    >
      <div className="relative overflow-hidden px-5 py-5">
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_12%_0%,color-mix(in_srgb,var(--primary)_22%,transparent),transparent_42%),linear-gradient(135deg,color-mix(in_srgb,var(--primary)_10%,transparent),transparent_72%)]" />
        <div className="relative flex items-start gap-4">
          <UiRoomAvatar
            avatar={artifact.avatar}
            className="ring-4 ring-[color:color-mix(in_srgb,var(--primary)_10%,transparent)]"
            members={members}
            roomId={artifact.resource_id}
            size="lg"
            title={artifact.name}
          />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="inline-flex items-center gap-1.5 rounded-full bg-[color:color-mix(in_srgb,var(--primary)_13%,transparent)] px-2.5 py-1 text-[11px] font-semibold tracking-[0.08em] text-primary">
                <BadgeCheck className="h-3.5 w-3.5" />
                ROOM 已创建
              </span>
              {members.length > 0 ? (
                <span className="inline-flex items-center gap-1 text-[11px] text-(--text-muted)">
                  <UsersRound className="h-3.5 w-3.5" />
                  {members.length} 位 Agent
                </span>
              ) : null}
            </div>
            <h3 className="mt-2 truncate text-[19px] font-semibold text-(--text-strong)">
              {artifact.name}
            </h3>
            {artifact.description ? (
              <p className="mt-1.5 line-clamp-2 text-[12px] leading-5 text-(--text-muted)">
                {artifact.description}
              </p>
            ) : null}
            {members.length > 0 ? (
              <div className="mt-3 flex flex-wrap gap-1.5">
                {members.slice(0, 4).map((member) => (
                  <span
                    className="rounded-full border border-(--divider-subtle-color) bg-[color-mix(in_srgb,var(--surface-panel-background)_88%,transparent)] px-2.5 py-1 text-[11px] text-(--text-muted)"
                    key={member.id}
                  >
                    {member.name}
                  </span>
                ))}
              </div>
            ) : null}
          </div>
          <ArrowUpRight className="h-5 w-5 shrink-0 text-primary transition-transform duration-200 group-hover:-translate-y-0.5 group-hover:translate-x-0.5" />
        </div>
      </div>
      <div className="flex items-center justify-between gap-3 border-t border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--primary)_5%,transparent)] px-5 py-3.5">
        <span className="flex items-center gap-2 text-[12px] text-(--text-muted)">
          <MessagesSquare className="h-4 w-4 text-primary" />
          进入 Room 开始真实协作
        </span>
        <span className="inline-flex items-center gap-1.5 rounded-full bg-primary px-3.5 py-1.5 text-[12px] font-semibold text-primary-foreground transition-transform duration-200 group-hover:translate-x-0.5">
          进入 Room
          <ArrowRight className="h-3.5 w-3.5" />
        </span>
      </div>
    </Link>
  );
}

function AgentResourceArtifactCard({
  artifact,
}: NexusResourceArtifactCardProps) {
  return (
    <Link
      aria-label={`打开 Agent：${artifact.name}`}
      className="group my-3 block w-full max-w-[680px] overflow-hidden rounded-[20px] border border-[color:color-mix(in_srgb,var(--primary)_30%,var(--surface-panel-border))] bg-(--surface-panel-background) shadow-[0_18px_52px_color-mix(in_srgb,var(--primary)_14%,transparent)] transition-[border-color,box-shadow,transform] duration-300 hover:-translate-y-0.5 hover:border-[color:color-mix(in_srgb,var(--primary)_52%,var(--surface-panel-border))] hover:shadow-[0_24px_64px_color-mix(in_srgb,var(--primary)_20%,transparent)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
      to={buildNexusResourceArtifactRoute(artifact)}
    >
      <div className="flex items-start gap-4 bg-[linear-gradient(135deg,color-mix(in_srgb,var(--primary)_12%,transparent),transparent_68%)] px-5 py-5">
        <UiAgentAvatar
          avatar={artifact.avatar}
          name={artifact.name}
          shape="rounded"
          size="lg"
        />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.13em] text-primary">
            <BadgeCheck className="h-3.5 w-3.5" />
            Agent 已创建
          </div>
          <h3 className="mt-1.5 truncate text-[18px] font-semibold text-(--text-strong)">
            {artifact.name}
          </h3>
          {artifact.description ? (
            <p className="mt-1.5 line-clamp-2 text-[12px] leading-5 text-(--text-muted)">
              {artifact.description}
            </p>
          ) : null}
          {artifact.vibe_tags?.length ? (
            <div className="mt-3 flex flex-wrap gap-1.5">
              {artifact.vibe_tags.map((tag) => (
                <span
                  className="rounded-full bg-[color-mix(in_srgb,var(--primary)_9%,transparent)] px-2.5 py-1 text-[11px] font-medium text-primary"
                  key={tag}
                >
                  {tag}
                </span>
              ))}
            </div>
          ) : null}
        </div>
        <ArrowUpRight className="h-5 w-5 shrink-0 text-primary transition-transform duration-200 group-hover:-translate-y-0.5 group-hover:translate-x-0.5" />
      </div>
      <div className="flex items-center gap-2 border-t border-(--divider-subtle-color) px-5 py-3 text-[12px] text-(--text-muted)">
        <Bot className="h-4 w-4 text-primary" />
        查看并继续配置 Agent
      </div>
    </Link>
  );
}

export function NexusResourceArtifactCard(
  props: NexusResourceArtifactCardProps,
) {
  return props.artifact.resource_kind === "room" ? (
    <RoomResourceArtifactCard {...props} />
  ) : (
    <AgentResourceArtifactCard {...props} />
  );
}
