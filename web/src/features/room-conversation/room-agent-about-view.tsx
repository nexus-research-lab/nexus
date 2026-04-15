"use client";

import { ReactNode } from "react";
import { Bot, Shield } from "lucide-react";

import { getIconAvatarSrc, getInitials } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/workspace-surface-view";
import { Agent } from "@/types/agent";
import { format_provider_label } from "@/types/provider";

interface RoomAgentAboutViewProps {
  agent: Agent;
  header_action?: ReactNode;
}

export function RoomAgentAboutView({ agent, header_action }: RoomAgentAboutViewProps) {
  const { t } = useI18n();
  const avatar_src = getIconAvatarSrc(agent.avatar);

  return (
    <WorkspaceSurfaceView
      action={header_action}
      body_class_name="px-4 py-5 sm:px-5 xl:px-6"
      eyebrow={t("room.about")}
      max_width_class_name="max-w-[820px]"
      show_eyebrow={false}
      title={agent.name}
    >
      <div className="rounded-[24px] border border-(--divider-subtle-color) p-5">
        <div className="flex items-center gap-3">
          <div className="flex h-11 w-11 items-center justify-center overflow-hidden rounded-[16px] border border-(--divider-subtle-color) bg-(--surface-avatar-background) text-(--icon-strong)">
            {avatar_src ? (
              <img
                alt={agent.name}
                className="h-full w-full object-cover"
                src={avatar_src}
              />
            ) : (
              <span className="text-[13px] font-bold text-(--text-strong)">
                {getInitials(agent.name, "AG")}
              </span>
            )}
          </div>
          <div>
            <p className="text-base font-semibold text-(--text-strong)">{agent.name}</p>
            <p className="text-[13px] text-(--text-muted)">{t("room.about_subtitle")}</p>
          </div>
        </div>

        <dl className="mt-5 divide-y divide-(--divider-subtle-color) border-t border-(--divider-subtle-color)">
          <div className="flex items-center justify-between gap-4 py-4">
            <dt className="text-[11px] font-semibold uppercase tracking-[0.14em] text-(--text-soft)">
              {t("room.about_provider")}
            </dt>
            <dd className="text-[13px] font-semibold text-(--text-strong)">
              {format_provider_label(agent.options.provider)}
            </dd>
          </div>
          <div className="flex items-center justify-between gap-4 py-4">
            <dt className="text-[11px] font-semibold uppercase tracking-[0.14em] text-(--text-soft)">
              {t("room.about_permission")}
            </dt>
            <dd className="inline-flex items-center gap-2 text-[13px] font-semibold text-(--text-strong)">
              <Shield className="h-4 w-4 text-(--icon-default)" />
              {agent.options.permission_mode || "default"}
            </dd>
          </div>
        </dl>
      </div>
    </WorkspaceSurfaceView>
  );
}
