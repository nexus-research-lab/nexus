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
import { Fragment, memo, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { resolveAgentId } from "@/config/options";
import { getConnectedCountApi } from "@/lib/connector-api";
import { listScheduledTasksApi } from "@/lib/scheduled-task-api";
import { getAvailableSkillsApi } from "@/lib/skill-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { SidebarListItem } from "@/shared/ui/sidebar/collapsible-section";
import { SIDEBAR_CAPABILITY_ITEM_IDS, useSidebarStore } from "@/store/sidebar";
import { SkillInfo } from "@/types/skill";

const SCHEDULED_TASKS_MUTATED_EVENT = "nexus:scheduled-tasks-mutated";

export const CapabilitiesPanelContent = memo(function CapabilitiesPanelContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const active_panel_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const set_active_panel_item = useSidebarStore((s) => s.set_active_panel_item);
  const agent_id = resolveAgentId();
  const [skills, set_skills] = useState<SkillInfo[]>([]);
  const [connector_count, set_connector_count] = useState(0);
  const [scheduled_task_enabled_count, set_scheduled_task_enabled_count] = useState(0);

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
      .catch(() => { });
    const refresh_scheduled_task_count = async () => {
      try {
        const tasks = await listScheduledTasksApi({ agent_id });
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

  return (
    <Fragment>
      <SidebarListItem
        icon={<Puzzle className="h-4 w-4" />}
        is_active={active_panel_item_id === SIDEBAR_CAPABILITY_ITEM_IDS.skills}
        label={t("capability.skills")}
        meta={String(skill_count)}
        on_click={() => {
          set_active_panel_item(SIDEBAR_CAPABILITY_ITEM_IDS.skills);
          navigate(AppRouteBuilders.skills());
        }}
      />

      <SidebarListItem
        icon={<Link2 className="h-4 w-4" />}
        is_active={active_panel_item_id === SIDEBAR_CAPABILITY_ITEM_IDS.connectors}
        label={t("capability.connectors")}
        meta={String(connector_count)}
        on_click={() => {
          set_active_panel_item(SIDEBAR_CAPABILITY_ITEM_IDS.connectors);
          navigate(AppRouteBuilders.connectors());
        }}
      />

      <SidebarListItem
        icon={<Calendar className="h-4 w-4" />}
        is_active={active_panel_item_id === SIDEBAR_CAPABILITY_ITEM_IDS.scheduled_tasks}
        label={t("capability.scheduled")}
        meta={String(scheduled_task_enabled_count)}
        on_click={() => {
          set_active_panel_item(SIDEBAR_CAPABILITY_ITEM_IDS.scheduled_tasks);
          navigate(AppRouteBuilders.scheduled_tasks());
        }}
      />

      <SidebarListItem
        icon={<Radio className="h-4 w-4" />}
        is_active={active_panel_item_id === SIDEBAR_CAPABILITY_ITEM_IDS.channels}
        label={t("capability.channels")}
        meta={String(channel_count)}
        on_click={() => {
          set_active_panel_item(SIDEBAR_CAPABILITY_ITEM_IDS.channels);
          navigate(AppRouteBuilders.channels());
        }}
      />

      <SidebarListItem
        icon={<Users2 className="h-4 w-4" />}
        is_active={active_panel_item_id === SIDEBAR_CAPABILITY_ITEM_IDS.pairings}
        label={t("capability.pairings")}
        meta={String(pairing_count)}
        on_click={() => {
          set_active_panel_item(SIDEBAR_CAPABILITY_ITEM_IDS.pairings);
          navigate(AppRouteBuilders.pairings());
        }}
      />
    </Fragment>
  );
});
