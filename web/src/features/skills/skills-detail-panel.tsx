"use client";

import { useCallback, useEffect, useState } from "react";
import { Bot, Loader2, Lock, Puzzle, Shield, Sparkles, Tag } from "lucide-react";

import { getAgents } from "@/lib/agent-manage-api";
import { installSkillApi, uninstallSkillApi } from "@/lib/skill-api";
import { WorkspaceInspectorSection } from "@/shared/ui/workspace-inspector-section";
import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";
import { Agent } from "@/types/agent";
import { SkillInfo } from "@/types/skill";

interface SkillsDetailPanelProps {
  skill: SkillInfo | null;
  /** 哪些 agent 已安装此 skill */
  installed_agent_ids: string[];
  on_change: () => void;
}

export function SkillsDetailPanel({
  skill,
  installed_agent_ids,
  on_change,
}: SkillsDetailPanelProps) {
  const [agents, set_agents] = useState<Agent[]>([]);
  const [toggling, set_toggling] = useState<string | null>(null);

  useEffect(() => {
    void getAgents().then(set_agents);
  }, []);

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
        on_change();
      } catch (err) {
        console.error("[SkillDetail] Toggle failed:", err);
      } finally {
        set_toggling(null);
      }
    },
    [on_change, skill, toggling],
  );

  if (!skill) {
    return (
      <aside className="home-glass-panel-subtle radius-shell-xl hidden w-[340px] shrink-0 overflow-y-auto xl:block">
        <div className="flex h-full items-center justify-center px-6 text-center text-sm text-slate-500/60">
          选择一个技能查看详情
        </div>
      </aside>
    );
  }

  const is_system = skill.scope === "main";

  return (
    <aside className="home-glass-panel-subtle radius-shell-xl hidden w-[340px] shrink-0 overflow-y-auto xl:block">
      {/* Header */}
      <div className="border-b workspace-divider px-5 py-5">
        <div className="flex items-center gap-3">
          <div className="workspace-chip flex h-14 w-14 shrink-0 items-center justify-center rounded-[18px]">
            <Puzzle className="h-6 w-6 text-slate-900/82" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-[20px] font-black tracking-[-0.04em] text-slate-950/92">
              {skill.name}
            </p>
            <p className="mt-1 text-[12px] font-semibold uppercase tracking-[0.12em] text-sky-600/80">
              {is_system ? "SYSTEM SKILL" : "USER SKILL"}
            </p>
          </div>
        </div>
      </div>

      {/* Description */}
      <WorkspaceInspectorSection icon={Sparkles} title="Description">
        <div className="workspace-card rounded-[18px] px-4 py-3">
          <p className="text-[13px] leading-6 text-slate-700/78">
            {skill.description || "暂无描述"}
          </p>
        </div>
      </WorkspaceInspectorSection>

      {/* Meta */}
      <WorkspaceInspectorSection icon={Tag} title="Meta">
        <div className="space-y-2">
          <div className="workspace-card rounded-[18px] px-4 py-3">
            <div className="flex items-center gap-3 text-[13px] text-slate-700/78">
              <Shield className="h-4 w-4 text-amber-500" />
              <span>范围: {is_system ? "仅主智能体" : "所有智能体"}</span>
            </div>
          </div>
          {skill.tags.length > 0 && (
            <div className="workspace-card rounded-[18px] px-4 py-3">
              <div className="flex flex-wrap gap-1.5">
                {skill.tags.map((tag) => (
                  <span
                    key={tag}
                    className="inline-flex rounded-full bg-slate-100/80 px-2.5 py-1 text-[11px] font-semibold text-slate-600"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      </WorkspaceInspectorSection>

      {/* Agent installations */}
      {!is_system && (
        <WorkspaceInspectorSection
          icon={Bot}
          title="已安装到"
          action={
            <span className="workspace-chip rounded-full px-2 py-0.5 text-[10px] font-bold text-slate-700/68">
              {installed_agent_ids.length}
            </span>
          }
        >
          <div className="space-y-2">
            {agents
              .filter((a) => a.options.skills_enabled)
              .map((agent) => {
                const is_installed = installed_agent_ids.includes(agent.agent_id);
                return (
                  <div
                    key={agent.agent_id}
                    className="workspace-card flex items-center justify-between gap-3 rounded-[18px] px-4 py-3"
                  >
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-[13px] font-semibold text-slate-950/88">
                        {agent.name}
                      </p>
                    </div>
                    <button
                      onClick={() => handle_toggle(agent.agent_id, is_installed)}
                      disabled={toggling === agent.agent_id}
                      className={
                        "inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[10px] font-semibold transition-all duration-200 disabled:opacity-60 " +
                        (is_installed
                          ? "bg-emerald-50 text-emerald-600 hover:bg-red-50 hover:text-red-500"
                          : "workspace-chip text-slate-600 hover:bg-sky-50 hover:text-sky-600")
                      }
                    >
                      {toggling === agent.agent_id ? (
                        <Loader2 className="h-3 w-3 animate-spin" />
                      ) : is_installed ? (
                        "已启用"
                      ) : (
                        "启用"
                      )}
                    </button>
                  </div>
                );
              })}
          </div>
        </WorkspaceInspectorSection>
      )}

      {is_system && (
        <WorkspaceInspectorSection icon={Lock} title="访问控制">
          <div className="workspace-card rounded-[18px] px-4 py-3">
            <p className="text-[13px] leading-6 text-slate-700/78">
              此技能为系统级技能，仅由主智能体使用，不支持手动安装或卸载。
            </p>
          </div>
        </WorkspaceInspectorSection>
      )}
    </aside>
  );
}
