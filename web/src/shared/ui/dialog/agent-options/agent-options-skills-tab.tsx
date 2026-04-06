"use client";

import { useCallback, useDeferredValue, useEffect, useMemo, useState } from "react";
import { Loader2, Lock, Search } from "lucide-react";

import { getAgentSkillsApi, installSkillApi, uninstallSkillApi } from "@/lib/skill-api";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import type { AgentSkillEntry } from "@/types/skill";

interface AgentOptionsSkillsTabProps {
  agent_id?: string;
  is_visible: boolean;
}

export function AgentOptionsSkillsTab({
  agent_id,
  is_visible,
}: AgentOptionsSkillsTabProps) {
  const [skills, setSkills] = useState<AgentSkillEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [toggling, setToggling] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const deferredSearchQuery = useDeferredValue(searchQuery);

  const loadSkills = useCallback(async () => {
    if (!agent_id) {
      setSkills([]);
      setErrorMessage(null);
      return;
    }

    try {
      setLoading(true);
      setErrorMessage(null);
      const data = await getAgentSkillsApi(agent_id);
      setSkills(data);
    } catch (error) {
      setErrorMessage(
        error instanceof Error ? error.message : "加载技能列表失败"
      );
    } finally {
      setLoading(false);
    }
  }, [agent_id]);

  useEffect(() => {
    if (!is_visible) {
      return;
    }
    void loadSkills();
  }, [is_visible, loadSkills]);

  const handleToggle = useCallback(
    async (skill: AgentSkillEntry) => {
      if (!agent_id || skill.locked || toggling) {
        return;
      }

      try {
        setToggling(skill.name);
        setErrorMessage(null);
        if (skill.installed) {
          await uninstallSkillApi(agent_id, skill.name);
        } else {
          await installSkillApi(agent_id, skill.name);
        }
        await loadSkills();
      } catch (error) {
        setErrorMessage(
          error instanceof Error ? error.message : "切换技能状态失败"
        );
      } finally {
        setToggling(null);
      }
    },
    [agent_id, loadSkills, toggling]
  );

  const installedCount = useMemo(
    () => skills.filter((skill) => skill.installed).length,
    [skills]
  );
  const installedSkills = useMemo(
    () => skills.filter((skill) => skill.installed),
    [skills]
  );
  const addableSkills = useMemo(
    () => skills.filter((skill) => !skill.installed && !skill.locked),
    [skills]
  );
  const filteredAddableSkills = useMemo(() => {
    const query = deferredSearchQuery.trim().toLowerCase();
    if (!query) {
      return addableSkills;
    }
    return addableSkills.filter((skill) => {
      const haystacks = [
        skill.name,
        skill.title,
        skill.description,
        skill.category_name,
        skill.tags.join(" "),
      ];
      return haystacks.some((value) => value.toLowerCase().includes(query));
    });
  }, [addableSkills, deferredSearchQuery]);

  const renderSkillCard = (
    skill: AgentSkillEntry,
    actionLabel: string,
    tone: "installed" | "add"
  ) => {
    const isBusy = toggling === skill.name;
    return (
      <div
        key={skill.name}
        className="dialog-card rounded-[22px] px-5 py-4 transition-all duration-200 hover:border-primary/20 hover:bg-white/80"
      >
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-semibold text-slate-900">
                {skill.title || skill.name}
              </span>
            {skill.locked ? (
                <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] font-semibold text-emerald-600">
                  <Lock className="h-3 w-3" />
                  系统内置
                </span>
              ) : null}
              {skill.scope === "main" ? (
                <span className="rounded-full bg-sky-50 px-2 py-0.5 text-[10px] font-semibold text-sky-600">
                  Main Only
                </span>
              ) : null}
            </div>
            {skill.description ? (
              <p className="mt-2 line-clamp-1 text-sm leading-6 text-slate-600">
                {skill.description}
              </p>
            ) : null}
          </div>

          {skill.locked ? (
            <span className="inline-flex h-9 items-center rounded-full bg-emerald-50 px-3 text-xs font-semibold text-emerald-600">
              已启用
            </span>
          ) : (
            <WorkspacePillButton
              disabled={isBusy}
              onClick={() => void handleToggle(skill)}
              size="sm"
              variant={tone === "installed" ? "tonal" : "primary"}
            >
              {isBusy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : actionLabel}
            </WorkspacePillButton>
          )}
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-6 animate-in slide-in-from-right-4 duration-300">
      <div className="flex items-center justify-between gap-4">
        <h3 className="text-xl font-semibold tracking-tight text-slate-900">
          {installedCount} skills
        </h3>
        <span className="text-sm text-slate-400">{skills.length} total</span>
      </div>

      {errorMessage ? (
        <div className="radius-shell-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-600">
          {errorMessage}
        </div>
      ) : null}

      {loading ? (
        <div className="surface-card radius-shell-md flex items-center justify-center py-12 text-slate-400">
          <Loader2 className="h-4 w-4 animate-spin" />
        </div>
      ) : null}

      {!agent_id ? (
        <div className="surface-card radius-shell-md px-5 py-6 text-sm text-slate-600">
          创建后可安装 skill。
        </div>
      ) : null}

      {agent_id && !loading ? (
        <>
          <div className="space-y-4">
            <div className="flex items-center justify-between gap-4">
              <h4 className="text-sm font-semibold text-slate-900">已安装</h4>
              <span className="text-xs text-slate-400">{installedSkills.length}</span>
            </div>

            {installedSkills.length === 0 ? (
              <div className="surface-card radius-shell-md px-5 py-6 text-sm text-slate-600">
                暂无已安装 skill。
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-3">
                {installedSkills.map((skill) => renderSkillCard(skill, "移除", "installed"))}
              </div>
            )}
          </div>

          <div className="space-y-4">
            <div className="flex items-center justify-between gap-4">
              <h4 className="text-sm font-semibold text-slate-900">添加技能</h4>
              <span className="text-xs text-slate-400">
                {filteredAddableSkills.length}/{addableSkills.length}
              </span>
            </div>

            <div className="surface-card radius-shell-md p-4">
              <div className="flex items-center gap-3 rounded-[18px] border border-slate-200/80 bg-white px-4 py-3">
                <Search className="h-4 w-4 text-slate-400" />
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(event) => setSearchQuery(event.target.value)}
                  placeholder="搜索 skill"
                  className="w-full bg-transparent text-sm text-slate-800 outline-none placeholder:text-slate-400"
                />
              </div>
            </div>

            {skills.length === 0 ? (
              <div className="surface-card radius-shell-md px-5 py-6 text-sm text-slate-600">
                当前没有可用 skill。
              </div>
            ) : null}

            {skills.length > 0 && filteredAddableSkills.length === 0 ? (
              <div className="surface-card radius-shell-md px-5 py-6 text-sm text-slate-600">
                {addableSkills.length === 0
                  ? "没有更多可添加 skill。"
                  : "没有匹配的 skill。"}
              </div>
            ) : null}

            {filteredAddableSkills.length > 0 ? (
              <div className="grid grid-cols-1 gap-3">
                {filteredAddableSkills.map((skill) => renderSkillCard(skill, "添加", "add"))}
              </div>
            ) : null}
          </div>
        </>
      ) : null}
    </div>
  );
}
