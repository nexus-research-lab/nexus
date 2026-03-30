"use client";

import { Puzzle, Upload } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { getAgentSkillsApi, getAvailableSkillsApi } from "@/lib/skill-api";
import { getAgents } from "@/lib/agent-manage-api";
import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";
import { WorkspaceSearchInput } from "@/shared/ui/workspace-search-input";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace-surface-header";
import { Agent } from "@/types/agent";
import { SkillInfo, AgentSkillEntry } from "@/types/skill";

import { SkillDetailDialog } from "./skill-detail-dialog";
import { SkillsCard } from "./skills-card";

/** Skills 全宽卡片网格 — 风格 */
export function SkillsDirectory() {
  const [skills, set_skills] = useState<SkillInfo[]>([]);
  const [agents, set_agents] = useState<Agent[]>([]);
  const [agent_skills_map, set_agent_skills_map] = useState<Map<string, AgentSkillEntry[]>>(new Map());
  const [search_query, set_search_query] = useState("");
  const [loading, set_loading] = useState(true);
  // 弹窗状态：选中的 skill 名称
  const [selected_skill, set_selected_skill] = useState<string | null>(null);

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
      const skill = skills.find((s) => s.name === name);
      if (skill?.scope === "main") return true;
      if (name === "memory-manager") return true;
      return (skill_install_map.get(name)?.length ?? 0) > 0;
    },
    [skill_install_map, skills],
  );

  // 判断是否系统级
  const is_skill_locked = useCallback(
    (name: string) => name === "memory-manager" || skills.find((s) => s.name === name)?.scope === "main",
    [skills],
  );

  // 搜索过滤
  const filtered_skills = useMemo(() => {
    return skills.filter((skill) => {
      if (!search_query) return true;
      const q = search_query.toLowerCase();
      return (
        skill.name.toLowerCase().includes(q) ||
        skill.description.toLowerCase().includes(q) ||
        skill.tags.some((t) => t.toLowerCase().includes(q))
      );
    });
  }, [search_query, skills]);

  // Header 右侧：搜索框 + Upload 按钮
  const header_trailing = (
    <>
      <WorkspaceSearchInput
        class_name="hidden sm:inline-flex"
        input_class_name="w-[200px]"
        on_change={set_search_query}
        placeholder="搜索技能..."
        value={search_query}
      />
      <WorkspacePillButton disabled size="sm">
        <Upload className="h-3.5 w-3.5" />
        Upload
      </WorkspacePillButton>
    </>
  );

  // 抑制 agents 未使用警告（后续详情页会用到）
  void agents;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <WorkspaceSurfaceHeader
        badge="SKILLS"
        leading={<Puzzle className="h-4 w-4 text-slate-800/72" />}
        subtitle="浏览和安装技能，扩展 Agent 的专业能力"
        title="🧩 技能"
        trailing={header_trailing}
      />

      {/* 卡片网格区域 */}
      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-5 py-5 xl:px-6">
        {loading ? (
          <div className="flex min-h-[320px] items-center justify-center text-sm text-slate-500/60">
            加载中...
          </div>
        ) : filtered_skills.length ? (
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-3">
            {filtered_skills.map((skill) => (
              <SkillsCard
                key={skill.name}
                description={skill.description}
                installed={is_skill_installed(skill.name)}
                locked={is_skill_locked(skill.name)}
                name={skill.name}
                on_select={() => set_selected_skill(skill.name)}
                tags={skill.tags}
              />
            ))}
          </div>
        ) : (
          <div className="workspace-card flex min-h-[320px] items-center justify-center rounded-[28px] px-8 text-center">
            <div>
              <p className="text-[22px] font-bold tracking-[-0.04em] text-slate-950/90">
                没有符合条件的技能
              </p>
              <p className="mt-3 text-sm leading-7 text-slate-700/60">
                换一个搜索条件，或者在 skills/ 目录下添加新的 SKILL.md。
              </p>
            </div>
          </div>
        )}
      </div>

      {/* Skill 详情弹窗 */}
      {selected_skill && (
        <SkillDetailDialog
          is_open={!!selected_skill}
          on_close={() => set_selected_skill(null)}
          skill_name={selected_skill}
        />
      )}
    </div>
  );
}
