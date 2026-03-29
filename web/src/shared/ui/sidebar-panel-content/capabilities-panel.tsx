/**
 * Capabilities 面板内容
 *
 * 5 个可折叠分区：Skills、Connectors、Scheduled Tasks、Channels、Pairings。
 * Skills 分区调用 getAvailableSkillsApi() 获取数据。
 * 其余分区暂无数据，显示占位文案。
 * 每个分区标题右侧有 [→] 按钮，点击导航到对应全量页面。
 */

import {
  ArrowRight,
  Calendar,
  ChevronDown,
  ChevronRight,
  Link2,
  Puzzle,
  Radio,
  Users2,
} from "lucide-react";
import { memo, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getAvailableSkillsApi } from "@/lib/skill-api";
import { cn } from "@/lib/utils";
import { useSidebarStore } from "@/store/sidebar";
import { SkillInfo } from "@/types/skill";

// ==================== 可折叠 Section ====================

interface CapSectionProps {
  section_id: string;
  title: string;
  count: number;
  icon: React.ReactNode;
  /** 点击 [→] 导航到全量页面 */
  on_navigate?: () => void;
  children: React.ReactNode;
}

function CapSection({
  section_id,
  title,
  count,
  icon,
  on_navigate,
  children,
}: CapSectionProps) {
  const is_collapsed = useSidebarStore(
    (s) => s.collapsed_sections[section_id] ?? false,
  );
  const toggle = useSidebarStore((s) => s.toggle_section);

  return (
    <section className="border-b border-white/10 pb-1">
      <div className="flex items-center gap-1 px-2 py-2">
        <button
          className="flex flex-1 items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.08em] text-slate-500 transition-colors hover:text-slate-700"
          onClick={() => toggle(section_id)}
          type="button"
        >
          {is_collapsed ? (
            <ChevronRight className="h-3 w-3" />
          ) : (
            <ChevronDown className="h-3 w-3" />
          )}
          <span className="flex items-center gap-1.5">
            {icon}
            {title}
          </span>
          <span className="text-[10px] text-slate-400">{count}</span>
        </button>

        {/* 导航到全量页面按钮 */}
        {on_navigate ? (
          <button
            className="flex h-5 w-5 items-center justify-center rounded text-slate-400 transition-colors hover:bg-white/40 hover:text-slate-600"
            onClick={on_navigate}
            title={`查看全部${title}`}
            type="button"
          >
            <ArrowRight className="h-3 w-3" />
          </button>
        ) : null}
      </div>

      {!is_collapsed ? (
        <div className="flex flex-col gap-0.5 pb-1">{children}</div>
      ) : null}
    </section>
  );
}

// ==================== 空占位 ====================

function EmptyPlaceholder({ text }: { text: string }) {
  return (
    <p className="px-2 py-2 text-[11px] text-slate-400">{text}</p>
  );
}

// ==================== 主组件 ====================

export const CapabilitiesPanelContent = memo(function CapabilitiesPanelContent() {
  const navigate = useNavigate();
  const [skills, set_skills] = useState<SkillInfo[]>([]);

  // 加载 Skills 数据
  useEffect(() => {
    let cancelled = false;
    void getAvailableSkillsApi()
      .then((data) => {
        if (!cancelled) set_skills(data);
      })
      .catch(() => {
        // 静默处理错误
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="flex flex-col gap-1">
      {/* Skills 分区 */}
      <CapSection
        count={skills.length}
        icon={<Puzzle className="h-3 w-3" />}
        on_navigate={() => navigate(AppRouteBuilders.skills())}
        section_id="cap-skills"
        title="Skills"
      >
        {skills.length > 0 ? (
          skills.slice(0, 10).map((skill) => (
            <button
              key={skill.name}
              className={cn(
                "flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-[12px]",
                "text-slate-600 transition-all duration-150 hover:bg-white/30 hover:text-slate-800",
              )}
              onClick={() =>
                navigate(AppRouteBuilders.skills())
              }
              type="button"
            >
              <Puzzle className="h-3.5 w-3.5 shrink-0 text-slate-400" />
              <span className="min-w-0 flex-1 truncate">{skill.name}</span>
              <span className="shrink-0 text-[10px] text-slate-400">
                {skill.scope}
              </span>
            </button>
          ))
        ) : (
          <EmptyPlaceholder text="暂无技能" />
        )}
        {/* 超过 10 个时显示"查看更多" */}
        {skills.length > 10 ? (
          <button
            className="px-2 py-1 text-[11px] text-slate-500 hover:text-slate-700"
            onClick={() => navigate(AppRouteBuilders.skills())}
            type="button"
          >
            查看全部 {skills.length} 个技能 →
          </button>
        ) : null}
      </CapSection>

      {/* Connectors 分区 */}
      <CapSection
        count={0}
        icon={<Link2 className="h-3 w-3" />}
        section_id="cap-connectors"
        title="Connectors"
      >
        <EmptyPlaceholder text="暂无连接器" />
      </CapSection>

      {/* Scheduled Tasks 分区 */}
      <CapSection
        count={0}
        icon={<Calendar className="h-3 w-3" />}
        section_id="cap-scheduled"
        title="Scheduled"
      >
        <EmptyPlaceholder text="暂无定时任务" />
      </CapSection>

      {/* Channels 分区 */}
      <CapSection
        count={0}
        icon={<Radio className="h-3 w-3" />}
        section_id="cap-channels"
        title="Channels"
      >
        <EmptyPlaceholder text="暂无频道" />
      </CapSection>

      {/* Pairings 分区 */}
      <CapSection
        count={0}
        icon={<Users2 className="h-3 w-3" />}
        section_id="cap-pairings"
        title="Pairings"
      >
        <EmptyPlaceholder text="暂无配对" />
      </CapSection>
    </div>
  );
});
