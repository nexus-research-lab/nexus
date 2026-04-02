/**
 * Capabilities 面板内容
 *
 * 改为能力总览导航，不再展示展开式长列表。
 */

import {
  ArrowRight,
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
import { cn } from "@/lib/utils";
import { SkillInfo } from "@/types/skill";

interface CapabilitySummaryCardProps {
  title: string;
  count: number;
  icon: React.ReactNode;
  description: string;
  accent?: "default" | "active";
  selected?: boolean;
  on_click: () => void;
}

function CapabilitySummaryCard({
  title,
  count,
  icon,
  description,
  accent = "default",
  selected = false,
  on_click,
}: CapabilitySummaryCardProps) {
  return (
    <button
      className={cn(
        "group flex w-full items-center gap-3 rounded-[18px] border px-3 py-3 text-left transition-all duration-200",
        selected
          ? "border-sky-200/80 bg-white/80 shadow-[0_14px_30px_rgba(102,112,145,0.12)]"
          : accent === "active"
            ? "border-emerald-200/70 bg-white/72 shadow-[0_14px_30px_rgba(102,112,145,0.10)]"
            : "border-white/30 bg-white/42 hover:bg-white/56",
      )}
      onClick={on_click}
      type="button"
    >
      <div
        className={cn(
          "flex h-9 w-9 shrink-0 items-center justify-center rounded-[14px]",
          selected
            ? "bg-sky-50 text-sky-600"
            : accent === "active"
              ? "bg-emerald-50 text-emerald-600"
              : "bg-white/70 text-slate-500",
        )}
      >
        {icon}
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="text-[12px] font-semibold uppercase tracking-[0.08em] text-slate-600">
            {title}
          </span>
          <span className="rounded-full bg-white/75 px-1.5 py-0.5 text-[10px] font-semibold text-slate-500">
            {count}
          </span>
        </div>
        <p className="mt-1 line-clamp-1 text-[11px] text-slate-500">
          {description}
        </p>
      </div>

      <ArrowRight className="h-4 w-4 shrink-0 text-slate-400 transition-transform group-hover:translate-x-0.5 group-hover:text-slate-600" />
    </button>
  );
}

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
      .then((count) => {
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

  const skill_summary = useMemo(() => {
    if (skill_count === 0) {
      return "还没有安装能力";
    }
    return `已安装 ${skill_count} 个能力`;
  }, [skill_count]);

  return (
    <div className="flex flex-col gap-2">
      <CapabilitySummaryCard
        accent={skill_count > 0 ? "active" : "default"}
        count={skill_count}
        description={skill_summary}
        icon={<Puzzle className="h-4 w-4" />}
        on_click={() => navigate(AppRouteBuilders.skills())}
        selected={location.pathname.startsWith("/capability/skills")}
        title="Skills"
      />

      <CapabilitySummaryCard
        accent={connector_count > 0 ? "active" : "default"}
        count={connector_count}
        description={connector_count > 0 ? `已连接 ${connector_count} 个应用` : "连接外部服务和数据源"}
        icon={<Link2 className="h-4 w-4" />}
        on_click={() => navigate(AppRouteBuilders.connectors())}
        selected={location.pathname.startsWith("/capability/connectors")}
        title="Connectors"
      />

      <CapabilitySummaryCard
        count={0}
        description="自动运行周期性任务"
        icon={<Calendar className="h-4 w-4" />}
        on_click={() => navigate(AppRouteBuilders.scheduled_tasks())}
        selected={location.pathname.startsWith("/capability/scheduled-tasks")}
        title="Scheduled"
      />

      <CapabilitySummaryCard
        count={0}
        description="接入消息和通知通道"
        icon={<Radio className="h-4 w-4" />}
        on_click={() => navigate(AppRouteBuilders.channels())}
        selected={location.pathname.startsWith("/capability/channels")}
        title="Channels"
      />

      <CapabilitySummaryCard
        count={0}
        description="配置 Agent 之间的协作关系"
        icon={<Users2 className="h-4 w-4" />}
        on_click={() => navigate(AppRouteBuilders.pairings())}
        selected={location.pathname.startsWith("/capability/pairings")}
        title="Pairings"
      />
    </div>
  );
});
