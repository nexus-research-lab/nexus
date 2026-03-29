"use client";

import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { ArrowLeft, Bot, Check, Loader2, Lock, Puzzle, Shield, Tag } from "lucide-react";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getAgents } from "@/lib/agent-manage-api";
import { getAgentSkillsApi, getAvailableSkillsApi, installSkillApi, uninstallSkillApi } from "@/lib/skill-api";
import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";
import { Agent } from "@/types/agent";
import { AgentSkillEntry, SkillInfo } from "@/types/skill";

interface SkillsDetailPageProps {
  /** 路由参数中的 skill_name */
  skill_name: string;
}

/** Skill 详情页内容 — 独立页面，在 /skills/:skill_name 路由下渲染 */
export function SkillsDetailPage({ skill_name }: SkillsDetailPageProps) {
  const navigate = useNavigate();
  const [skill, set_skill] = useState<SkillInfo | null>(null);
  const [agents, set_agents] = useState<Agent[]>([]);
  const [agent_skills_map, set_agent_skills_map] = useState<Map<string, AgentSkillEntry[]>>(new Map());
  const [loading, set_loading] = useState(true);
  const [toggling, set_toggling] = useState<string | null>(null);

  const load_data = useCallback(async () => {
    try {
      set_loading(true);
      const [skills_data, agents_data] = await Promise.all([
        getAvailableSkillsApi(),
        getAgents(),
      ]);
      set_agents(agents_data);

      // 找到当前 skill
      const found = skills_data.find((s) => s.name === skill_name) ?? null;
      set_skill(found);

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
      console.error("[SkillDetail] Failed to load:", err);
    } finally {
      set_loading(false);
    }
  }, [skill_name]);

  useEffect(() => {
    void load_data();
  }, [load_data]);

  // 计算哪些 agent 安装了此 skill
  const installed_agent_ids: string[] = [];
  for (const [agent_id, entries] of agent_skills_map) {
    for (const entry of entries) {
      if (entry.name === skill_name && entry.installed && !entry.locked) {
        installed_agent_ids.push(agent_id);
      }
    }
  }

  // 安装/卸载操作
  const handle_toggle = useCallback(
    async (agent_id: string, currently_installed: boolean) => {
      if (!skill || toggling) return;
      set_toggling(agent_id);
      try {
        if (currently_installed) {
          await uninstallSkillApi(agent_id, skill.name);
        } else {
          await installSkillApi(agent_id, skill.name);
        }
        await load_data();
      } catch (err) {
        console.error("[SkillDetail] Toggle failed:", err);
      } finally {
        set_toggling(null);
      }
    },
    [load_data, skill, toggling],
  );

  if (loading) {
    return (
      <div className="flex min-h-[400px] items-center justify-center text-sm text-slate-500/60">
        加载中...
      </div>
    );
  }

  if (!skill) {
    return (
      <div className="flex min-h-[400px] flex-col items-center justify-center gap-4 text-center">
        <p className="text-[18px] font-bold text-slate-950/80">技能未找到</p>
        <p className="text-sm text-slate-700/60">找不到名为 &quot;{skill_name}&quot; 的技能。</p>
        <WorkspacePillButton onClick={() => navigate(AppRouteBuilders.skills())}>
          返回技能列表
        </WorkspacePillButton>
      </div>
    );
  }

  const is_system = skill.scope === "main";

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      {/* 详情头部 */}
      <div className="border-b workspace-divider px-5 py-5 xl:px-6">
        <div className="flex items-center gap-3">
          <button
            className="flex h-8 w-8 items-center justify-center rounded-full border border-slate-200/80 bg-white/72 text-slate-700 shadow-sm transition-all hover:bg-slate-100"
            onClick={() => navigate(AppRouteBuilders.skills())}
            type="button"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
          <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-[14px] bg-slate-100/80">
            <Puzzle className="h-5 w-5 text-slate-900/82" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-[20px] font-black tracking-[-0.04em] text-slate-950/92">
              {skill.name}
            </p>
            <p className="mt-1 text-[13px] text-slate-700/68">
              {skill.description || "暂无描述"}
            </p>
          </div>
        </div>

        {/* 标签 */}
        {skill.tags.length > 0 && (
          <div className="mt-4 flex flex-wrap gap-2">
            {is_system && (
              <span className="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2.5 py-1 text-[11px] font-semibold text-amber-700">
                <Shield className="h-3 w-3" />
                系统级
              </span>
            )}
            {skill.tags.map((tag) => (
              <span
                key={tag}
                className="inline-flex items-center gap-1 rounded-full bg-slate-100/80 px-2.5 py-1 text-[11px] font-semibold text-slate-600"
              >
                <Tag className="h-3 w-3" />
                {tag}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* 内容区域 */}
      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-5 py-5 xl:px-6">
        {/* 安装信息 */}
        {is_system ? (
          <section className="workspace-card rounded-[22px] px-5 py-5">
            <div className="flex items-center gap-3 text-[14px] text-slate-700/78">
              <Lock className="h-5 w-5 text-amber-500" />
              <span>此技能为系统级技能，仅由主智能体使用，不支持手动安装或卸载。</span>
            </div>
          </section>
        ) : (
          <section>
            <h3 className="text-[15px] font-bold text-slate-950/88">
              已安装到 {installed_agent_ids.length} 个 Agent
            </h3>
            <div className="mt-4 space-y-3">
              {agents
                .filter((a) => a.options.skills_enabled)
                .map((agent) => {
                  const is_installed = installed_agent_ids.includes(agent.agent_id);
                  return (
                    <div
                      key={agent.agent_id}
                      className="workspace-card flex items-center justify-between gap-3 rounded-[18px] px-4 py-3"
                    >
                      <div className="flex min-w-0 items-center gap-3">
                        <Bot className="h-4 w-4 shrink-0 text-slate-600" />
                        <p className="truncate text-[13px] font-semibold text-slate-950/88">
                          {agent.name}
                        </p>
                        {is_installed && (
                          <Check className="h-3.5 w-3.5 shrink-0 text-emerald-500" />
                        )}
                      </div>
                      <button
                        className={
                          "inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[10px] font-semibold transition-all duration-200 disabled:opacity-60 " +
                          (is_installed
                            ? "bg-emerald-50 text-emerald-600 hover:bg-red-50 hover:text-red-500"
                            : "workspace-chip text-slate-600 hover:bg-sky-50 hover:text-sky-600")
                        }
                        disabled={toggling === agent.agent_id}
                        onClick={() => handle_toggle(agent.agent_id, is_installed)}
                        type="button"
                      >
                        {toggling === agent.agent_id ? (
                          <Loader2 className="h-3 w-3 animate-spin" />
                        ) : is_installed ? (
                          "卸载"
                        ) : (
                          "安装"
                        )}
                      </button>
                    </div>
                  );
                })}
            </div>
          </section>
        )}
      </div>
    </div>
  );
}
