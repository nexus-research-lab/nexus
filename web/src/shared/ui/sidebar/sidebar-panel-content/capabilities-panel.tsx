/**
 * Capabilities 面板内容
 *
 * 能力分区内容。
 *
 * 这里使用和 Rooms / DMs 一致的侧栏列表形式，
 * 避免能力区仍然保持独立卡片样式。
 */

import {
  Calendar,
  Link2,
  Puzzle,
  Radio,
  Users2,
} from "lucide-react";
import { memo, useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getConnectedCountApi } from "@/lib/connector-api";
import { getAvailableSkillsApi } from "@/lib/skill-api";
import { SidebarListItem } from "@/shared/ui/sidebar/collapsible-section";
import { SkillInfo } from "@/types/skill";

export const CapabilitiesPanelContent = memo(function CapabilitiesPanelContent() {
  const navigate = useNavigate();
  const location = useLocation();
  const [skills, set_skills] = useState<SkillInfo[]>([]);
  const [connector_count, set_connector_count] = useState(0);

  useEffect(() => {
    let cancelled = false;
    void getAvailableSkillsApi()
      .then((data) => {
        if (!cancelled) {
          set_skills(data.filter((skill) => skill.installed));
        }
      })
      .catch(() => {
        if (!cancelled) {
          set_skills([]);
        }
      });
    void getConnectedCountApi()
      .then((count: number) => {
        if (!cancelled) {
          set_connector_count(count);
        }
      })
      .catch(() => {});

    return () => {
      cancelled = true;
    };
  }, []);

  const skill_count = useMemo(() => skills.length, [skills]);

  const scheduled_task_count = 0;
  const channel_count = 0;
  const pairing_count = 0;

  return (
    <div className="flex flex-col gap-0.5 pb-1">
      <SidebarListItem
        icon={<Puzzle className="h-3.5 w-3.5" />}
        is_active={location.pathname.startsWith("/capability/skills")}
        label="skills"
        meta={String(skill_count)}
        on_click={() => navigate(AppRouteBuilders.skills())}
      />

      <SidebarListItem
        icon={<Link2 className="h-3.5 w-3.5" />}
        is_active={location.pathname.startsWith("/capability/connectors")}
        label="connectors"
        meta={String(connector_count)}
        on_click={() => navigate(AppRouteBuilders.connectors())}
      />

      <SidebarListItem
        icon={<Calendar className="h-3.5 w-3.5" />}
        is_active={location.pathname.startsWith("/capability/scheduled-tasks")}
        label="scheduled"
        meta={String(scheduled_task_count)}
        on_click={() => navigate(AppRouteBuilders.scheduled_tasks())}
      />

      <SidebarListItem
        icon={<Radio className="h-3.5 w-3.5" />}
        is_active={location.pathname.startsWith("/capability/channels")}
        label="channels"
        meta={String(channel_count)}
        on_click={() => navigate(AppRouteBuilders.channels())}
      />

      <SidebarListItem
        icon={<Users2 className="h-3.5 w-3.5" />}
        is_active={location.pathname.startsWith("/capability/pairings")}
        label="pairings"
        meta={String(pairing_count)}
        on_click={() => navigate(AppRouteBuilders.pairings())}
      />
    </div>
  );
});
