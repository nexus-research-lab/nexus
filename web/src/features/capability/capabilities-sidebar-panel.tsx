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
  type LucideIcon,
  Puzzle,
  Radio,
  Users2,
} from "lucide-react";
import { Fragment, memo, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { resolve_agent_id } from "@/config/options";
import { get_connected_count_api } from "@/lib/api/connector-api";
import { list_scheduled_tasks_api } from "@/lib/api/scheduled-task-api";
import { get_available_skills_api } from "@/lib/api/skill-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { SidebarListItem } from "@/shared/ui/sidebar/collapsible-section";
import { SIDEBAR_CAPABILITY_ITEM_IDS, useSidebarStore } from "@/store/sidebar";
import { SkillInfo } from "@/types/capability/skill";

const SCHEDULED_TASKS_MUTATED_EVENT = "nexus:scheduled-tasks-mutated";

interface CapabilitySidebarItem {
  id: string;
  icon: LucideIcon;
  label: string;
  meta: string;
  path: string;
}

export const CapabilitiesPanelContent = memo(function CapabilitiesPanelContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const active_panel_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const set_active_panel_item = useSidebarStore((s) => s.set_active_panel_item);
  const agent_id = resolve_agent_id();
  const [skills, set_skills] = useState<SkillInfo[]>([]);
  const [connector_count, set_connector_count] = useState(0);
  const [scheduled_task_enabled_count, set_scheduled_task_enabled_count] = useState(0);

  useEffect(() => {
    let cancelled = false;
    void get_available_skills_api()
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
    void get_connected_count_api()
      .then((count: number) => {
        if (!cancelled) {
          set_connector_count(count);
        }
      })
      .catch(() => { });
    const refresh_scheduled_task_count = async () => {
      try {
        const tasks = await list_scheduled_tasks_api({ agent_id });
        if (!cancelled) {
          set_scheduled_task_enabled_count(tasks.filter((task) => task.enabled).length);
        }
      } catch {
        if (!cancelled) {
          set_scheduled_task_enabled_count(0);
        }
      }
    };
    void refresh_scheduled_task_count();

    const handle_scheduled_tasks_mutated = (event: Event) => {
      const custom_event = event as CustomEvent<{ agent_id?: string }>;
      if (custom_event.detail?.agent_id && custom_event.detail.agent_id !== agent_id) {
        return;
      }
      void refresh_scheduled_task_count();
    };
    window.addEventListener(SCHEDULED_TASKS_MUTATED_EVENT, handle_scheduled_tasks_mutated);

    return () => {
      cancelled = true;
      window.removeEventListener(SCHEDULED_TASKS_MUTATED_EVENT, handle_scheduled_tasks_mutated);
    };
  }, [agent_id]);

  const skill_count = useMemo(() => skills.length, [skills]);

  const channel_count = 0;
  const pairing_count = 0;
  const capability_items = useMemo<CapabilitySidebarItem[]>(() => [
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.skills,
      icon: Puzzle,
      label: t("capability.skills"),
      meta: String(skill_count),
      path: AppRouteBuilders.skills(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.connectors,
      icon: Link2,
      label: t("capability.connectors"),
      meta: String(connector_count),
      path: AppRouteBuilders.connectors(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.scheduled_tasks,
      icon: Calendar,
      label: t("capability.scheduled"),
      meta: String(scheduled_task_enabled_count),
      path: AppRouteBuilders.scheduled_tasks(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.channels,
      icon: Radio,
      label: t("capability.channels"),
      meta: String(channel_count),
      path: AppRouteBuilders.channels(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.pairings,
      icon: Users2,
      label: t("capability.pairings"),
      meta: String(pairing_count),
      path: AppRouteBuilders.pairings(),
    },
  ], [
    channel_count,
    connector_count,
    pairing_count,
    scheduled_task_enabled_count,
    skill_count,
    t,
  ]);

  return (
    <Fragment>
      {capability_items.map((item) => {
        const Icon = item.icon;
        return (
          <SidebarListItem
            icon={<Icon className="h-4 w-4" />}
            is_active={active_panel_item_id === item.id}
            key={item.id}
            label={item.label}
            meta={item.meta}
            on_click={() => {
              set_active_panel_item(item.id);
              navigate(item.path);
            }}
          />
        );
      })}
    </Fragment>
  );
});
