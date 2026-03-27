"use client";

import { Puzzle } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getAgentSkillsApi } from "@/lib/skill-api";
import { getAgents } from "@/lib/agent-manage-api";
import { WorkspaceCanvasShell } from "@/shared/ui/workspace-canvas-shell";
import { WorkspaceSearchInput } from "@/shared/ui/workspace-search-input";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace-surface-header";
import { Agent } from "@/types/agent";
import { SkillInfo, AgentSkillEntry } from "@/types/skill";
import { getAvailableSkillsApi } from "@/lib/skill-api";

import { SkillsCard } from "./skills-card";
import { SkillsDetailPanel } from "./skills-detail-panel";
import { SkillsFilterKey, SkillsFilterSidebar } from "./skills-filter-sidebar";

interface SkillsDirectoryProps {
  selected_skill_name?: string;
}

export function SkillsDirectory({ selected_skill_name }: SkillsDirectoryProps) {
  const navigate = useNavigate();
  const [skills, set_skills] = useState<SkillInfo[]>([]);
  const [agents, set_agents] = useState<Agent[]>([]);
  const [agent_skills_map, set_agent_skills_map] = useState<Map<string, AgentSkillEntry[]>>(new Map());
  const [active_filter, set_active_filter] = useState<SkillsFilterKey>("all");
  const [search_query, set_search_query] = useState("");
  const [loading, set_loading] = useState(true);

  const load_data = useCallback(async () => {
    try {
      set_loading(true);
      const [skills_data, agents_data] = await Promise.all([
        getAvailableSkillsApi(),
        getAgents(),
      ]);
      set_skills(skills_data);
      set_agents(agents_data);

      // 加载每个 agent 的 skill 安装状态
      const skills_enabled_agents = agents_data.filter((a: Agent) => a.options.skills_enabled);
      const entries = await Promise.all(
        skills_enabled_agents.map(async (agent: Agent) => {
          try {
            const agent_skills = await getAgentSkillsApi(agent.agent_id);
            return [agent.agent_id, agent_skills] as [string, AgentSkillEntry[]];
          } catch {
            return [agent.agent_id, [] as AgentSkillEntry[]] as [string, AgentSkillEntry[]];
          }
        }),
      );
      set_agent_skills_map(new Map(entries));
    } catch (err) {
      console.error("[Skills] Failed to load:", err);
    } finally {
      set_loading(false);
    }
  }, []);

  useEffect(() => {
    void load_data();
  }, [load_data]);

  // 计算每个 skill 被安装到了哪些 agent
  const skill_install_map = useMemo(() => {
    const map = new Map<string, string[]>();
    for (const [agent_id, entries] of agent_skills_map) {
      for (const entry of entries) {
        if (entry.installed && !entry.locked) {
          const list = map.get(entry.name) ?? [];
          list.push(agent_id);
          map.set(entry.name, list);
        }
      }
    }
    return map;
  }, [agent_skills_map]);

  // 判断一个 skill 是否 installed（至少一个 agent 安装了）
  const is_skill_installed = useCallback(
    (name: string) => {
      // 系统 skill（scope=main 或 base）视为 installed
      const skill = skills.find((s) => s.name === name);
      if (skill?.scope === "main") return true;
      if (name === "memory-manager") return true;
      return (skill_install_map.get(name)?.length ?? 0) > 0;
    },
    [skill_install_map, skills],
  );

  const is_skill_locked = useCallback(
    (name: string) => name === "memory-manager" || skills.find((s) => s.name === name)?.scope === "main",
    [skills],
  );

  // 过滤
  const filtered_skills = useMemo(() => {
    return skills.filter((skill) => {
      const installed = is_skill_installed(skill.name);
      const locked = is_skill_locked(skill.name);

      if (active_filter === "installed" && !installed) return false;
      if (active_filter === "available" && (installed || locked)) return false;
      if (active_filter === "system" && !locked) return false;

      if (search_query) {
        const q = search_query.toLowerCase();
        return (
          skill.name.toLowerCase().includes(q) ||
          skill.description.toLowerCase().includes(q) ||
          skill.tags.some((t) => t.toLowerCase().includes(q))
        );
      }
      return true;
    });
  }, [active_filter, is_skill_installed, is_skill_locked, search_query, skills]);

  const selected_skill = skills.find((s) => s.name === selected_skill_name) ?? skills[0] ?? null;

  const filter_sections = useMemo(() => {
    const count = (filter: SkillsFilterKey) => {
      if (filter === "all") return skills.length;
      if (filter === "installed") return skills.filter((s) => is_skill_installed(s.name)).length;
      if (filter === "available") return skills.filter((s) => !is_skill_installed(s.name) && !is_skill_locked(s.name)).length;
      if (filter === "system") return skills.filter((s) => is_skill_locked(s.name)).length;
      return 0;
    };

    return [
      {
        title: "Browse",
        items: [
          { key: "all" as const, label: "全部技能", count: count("all") },
          { key: "installed" as const, label: "已启用", count: count("installed"), dot_class_name: "bg-emerald-300" },
          { key: "available" as const, label: "可安装", count: count("available"), dot_class_name: "bg-sky-300" },
        ],
      },
      {
        title: "Category",
        items: [
          { key: "system" as const, label: "系统级", count: count("system"), dot_class_name: "bg-amber-300" },
        ],
      },
    ];
  }, [is_skill_installed, is_skill_locked, skills]);

  const header_trailing = (
    <WorkspaceSearchInput
      class_name="hidden xl:inline-flex"
      input_class_name="w-[220px]"
      on_change={set_search_query}
      placeholder="搜索技能名称或描述"
      value={search_query}
    />
  );

  return (
    <div className="flex min-h-0 min-w-0 flex-1 gap-2 lg:gap-2.5 xl:gap-3">
      <SkillsFilterSidebar
        active_filter={active_filter}
        on_change_filter={set_active_filter}
        sections={filter_sections}
        total_count={skills.length}
      />

      <WorkspaceCanvasShell is_joined_with_inspector>
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <WorkspaceSurfaceHeader
            badge="SKILLS"
            leading={<Puzzle className="h-4 w-4 text-slate-800/72" />}
            subtitle={
              <span className="truncate">
                {filtered_skills.length} / {skills.length} 个技能 · 为你的智能体装备能力
              </span>
            }
            title="技能中心"
            trailing={header_trailing}
          />

          <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-5 py-5 xl:px-6">
            {loading ? (
              <div className="flex min-h-[320px] items-center justify-center text-sm text-slate-500/60">
                加载中...
              </div>
            ) : filtered_skills.length ? (
              <div className="grid gap-5 xl:grid-cols-2 2xl:grid-cols-3">
                {filtered_skills.map((skill) => (
                  <SkillsCard
                    key={skill.name}
                    name={skill.name}
                    description={skill.description}
                    installed={is_skill_installed(skill.name)}
                    locked={is_skill_locked(skill.name)}
                    is_selected={selected_skill?.name === skill.name}
                    tags={skill.tags}
                    on_select={() => navigate(AppRouteBuilders.skill_detail(skill.name))}
                  />
                ))}
              </div>
            ) : (
              <div className="workspace-card flex min-h-[420px] items-center justify-center rounded-[28px] px-8 text-center">
                <div>
                  <p className="text-[22px] font-bold tracking-[-0.04em] text-slate-950/90">
                    没有符合条件的技能
                  </p>
                  <p className="mt-3 text-sm leading-7 text-slate-700/60">
                    换一个筛选条件，或者在 skills/ 目录下添加新的 SKILL.md。
                  </p>
                </div>
              </div>
            )}
          </div>
        </div>
      </WorkspaceCanvasShell>

      <SkillsDetailPanel
        skill={selected_skill}
        installed_agent_ids={skill_install_map.get(selected_skill?.name ?? "") ?? []}
        on_change={() => void load_data()}
      />
    </div>
  );
}
