/**
 * Skill 详情弹窗对话框
 *
 * 单栏布局（无侧边导航），复用 modal-dialog-surface CSS 系统。
 * 展示 Skill 名称、描述、标签、Agent 安装列表。
 */

"use client";

import { useCallback, useEffect, useState } from "react";
import { Bot, Check, Loader2, Lock, Puzzle, Shield, Tag, X } from "lucide-react";

import { getAgents } from "@/lib/agent-manage-api";
import { getAgentSkillsApi, getAvailableSkillsApi, installSkillApi, uninstallSkillApi } from "@/lib/skill-api";
import { Agent } from "@/types/agent";
import { AgentSkillEntry, SkillInfo } from "@/types/skill";

interface SkillDetailDialogProps {
  /** 要展示的 skill 名称 */
  skill_name: string;
  /** 是否打开 */
  is_open: boolean;
  /** 关闭回调 */
  on_close: () => void;
}

/** Skill 详情弹窗 — 单栏布局，modal-dialog-surface 风格 */
export function SkillDetailDialog({ skill_name, is_open, on_close }: SkillDetailDialogProps) {
  const [skill, set_skill] = useState<SkillInfo | null>(null);
  const [agents, set_agents] = useState<Agent[]>([]);
  const [agent_skills_map, set_agent_skills_map] = useState<Map<string, AgentSkillEntry[]>>(new Map());
  const [loading, set_loading] = useState(true);
  const [toggling, set_toggling] = useState<string | null>(null);

  // 加载 skill 详情和 agent 安装状态
  const load_data = useCallback(async () => {
    try {
      set_loading(true);
      const [skills_data, agents_data] = await Promise.all([
        getAvailableSkillsApi(),
        getAgents(),
      ]);
      set_agents(agents_data);

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
      console.error("[SkillDetailDialog] Failed to load:", err);
    } finally {
      set_loading(false);
    }
  }, [skill_name]);

  useEffect(() => {
    if (is_open) {
      void load_data();
    }
  }, [is_open, load_data]);

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
        console.error("[SkillDetailDialog] Toggle failed:", err);
      } finally {
        set_toggling(null);
      }
    },
    [load_data, skill, toggling],
  );

  if (!is_open) return null;

  const is_system = skill?.scope === "main";

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm animate-in fade-in duration-200"
      onClick={on_close}
    >
      <div
        className="modal-dialog-surface radius-shell-xl flex w-full max-w-2xl flex-col overflow-hidden animate-in zoom-in-95 duration-200"
        onClick={(e) => e.stopPropagation()}
        style={{ maxHeight: "80vh" }}
      >
        {/* 头部 */}
        <div className="flex items-center justify-between border-b modal-divider px-6 py-5">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl modal-card text-primary">
              <Puzzle className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <h2 className="truncate text-lg font-semibold tracking-tight text-slate-800">
                {loading ? "加载中..." : skill?.name ?? "技能未找到"}
              </h2>
              <p className="truncate text-xs text-slate-500">
                {loading ? "正在获取技能信息" : skill?.description ?? "暂无描述"}
              </p>
            </div>
          </div>
          <button
            aria-label="关闭对话框"
            className="modal-btn-secondary rounded-xl p-2 text-slate-400 transition-colors hover:text-slate-700 focus-visible:ring-2 focus-visible:ring-primary/50"
            onClick={on_close}
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* 内容区域 — 可滚动 */}
        <div className="soft-scrollbar flex-1 overflow-y-auto px-6 py-5">
          {loading ? (
            <div className="flex min-h-[200px] items-center justify-center">
              <Loader2 className="h-6 w-6 animate-spin text-slate-400" />
            </div>
          ) : !skill ? (
            /* 技能未找到 */
            <div className="flex min-h-[200px] flex-col items-center justify-center gap-3 text-center">
              <p className="text-[16px] font-bold text-slate-950/80">技能未找到</p>
              <p className="text-sm text-slate-700/60">
                找不到名为 &quot;{skill_name}&quot; 的技能。
              </p>
            </div>
          ) : (
            <>
              {/* 标签区域 */}
              {(skill.tags.length > 0 || is_system) && (
                <div className="mb-5 flex flex-wrap gap-2">
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

              {/* 描述 */}
              <section className="mb-5">
                <h3 className="mb-2 text-[13px] font-bold uppercase tracking-wider text-slate-400">
                  描述
                </h3>
                <p className="text-[14px] leading-6 text-slate-700/80">
                  {skill.description || "暂无描述"}
                </p>
              </section>

              {/* 分隔线 */}
              <div className="border-t modal-divider" />

              {/* Agent 安装列表 */}
              <section className="mt-5">
                {is_system ? (
                  <div className="modal-card flex items-center gap-3 rounded-2xl px-5 py-4">
                    <Lock className="h-5 w-5 shrink-0 text-amber-500" />
                    <span className="text-[14px] text-slate-700/78">
                      此技能为系统级技能，仅由主智能体使用，不支持手动安装或卸载。
                    </span>
                  </div>
                ) : (
                  <>
                    <h3 className="mb-3 text-[13px] font-bold uppercase tracking-wider text-slate-400">
                      已安装到 {installed_agent_ids.length} 个 Agent
                    </h3>
                    <div className="space-y-2.5">
                      {agents
                        .filter((a) => a.options.skills_enabled)
                        .map((agent) => {
                          const is_installed = installed_agent_ids.includes(agent.agent_id);
                          return (
                            <div
                              key={agent.agent_id}
                              className="modal-card flex items-center justify-between gap-3 rounded-[14px] px-4 py-3"
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
                                    : "modal-btn-secondary text-slate-600 hover:bg-sky-50 hover:text-sky-600")
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
                  </>
                )}
              </section>
            </>
          )}
        </div>

        {/* 底部按钮 */}
        <div className="flex items-center justify-end border-t modal-divider px-6 py-4">
          <button
            className="modal-btn-secondary rounded-xl px-5 py-2.5 text-sm font-medium text-slate-500 transition-colors hover:text-slate-800"
            onClick={on_close}
          >
            关闭
          </button>
        </div>
      </div>
    </div>
  );
}
