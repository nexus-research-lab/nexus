/**
 * AgentOptions Skills Tab
 *
 * 技能系统开关 + 加载来源 + 当前 Agent 启用技能列表
 */

"use client";

import { useEffect, useMemo, useState } from "react";
import { Check, Loader2, Puzzle, Sparkles } from "lucide-react";

import { getAgentSkillsApi, getAvailableSkillsApi } from "@/lib/skill-api";
import { cn } from "@/lib/utils";
import { AgentSkillEntry, SkillInfo } from "@/types/skill";

type SettingSource = "user" | "project" | "local";

interface SourceItem {
  key: SettingSource;
  label: string;
  description: string;
}

const SOURCE_ITEMS: SourceItem[] = [
  { key: "user", label: "用户设置", description: "读取全局技能和权限设置。" },
  { key: "project", label: "项目设置", description: "从 workspace 读取。" },
];

interface SkillToggleItem {
  name: string;
  title: string;
  description: string;
  version?: string;
  enabled: boolean;
  locked: boolean;
}

interface AgentOptionsSkillsTabProps {
  agentId?: string;
  enabledSkillNames: string[];
  skillsEnabled: boolean;
  onEnabledSkillNamesChange: (skillNames: string[]) => void;
  onSkillsEnabledChange: (enabled: boolean) => void;
  settingSources: SettingSource[];
  onToggleSettingSource: (source: SettingSource) => void;
}

function buildCreateModeSkills(
  global_skills: SkillInfo[],
  enabled_names: string[],
): SkillToggleItem[] {
  const enabled_set = new Set(enabled_names);
  // 市场页 getAvailableSkillsApi 返回的 installed 在 resource_pool_mode 下即为 pool 可用性
  return global_skills
    .filter((skill) => !skill.locked && skill.installed && skill.global_enabled)
    .map((skill) => ({
      name: skill.name,
      title: skill.title || skill.name,
      description: skill.description,
      version: skill.version,
      enabled: enabled_set.has(skill.name),
      locked: skill.locked,
    }));
}

function buildEditModeSkills(agent_skills: AgentSkillEntry[]): SkillToggleItem[] {
  // 后端已按资源池可用性过滤，前端直接展示全部返回项
  return agent_skills
    .filter((skill) => !skill.locked)
    .map((skill) => ({
      name: skill.name,
      title: skill.title || skill.name,
      description: skill.description,
      version: skill.version,
      enabled: skill.installed,
      locked: skill.locked,
    }));
}

export function AgentOptionsSkillsTab({
  agentId,
  enabledSkillNames,
  skillsEnabled,
  onEnabledSkillNamesChange,
  onSkillsEnabledChange,
  settingSources,
  onToggleSettingSource,
}: AgentOptionsSkillsTabProps) {
  const [loadingSkills, setLoadingSkills] = useState(false);
  const [availableSkills, setAvailableSkills] = useState<SkillToggleItem[]>([]);

  useEffect(() => {
    let cancelled = false;
    const load_skills = async () => {
      try {
        setLoadingSkills(true);
        if (agentId) {
          const agent_skills = await getAgentSkillsApi(agentId);
          if (!cancelled) {
            setAvailableSkills(buildEditModeSkills(agent_skills));
            onEnabledSkillNamesChange(
              agent_skills
                .filter((skill) => !skill.locked && skill.installed)
                .map((skill) => skill.name),
            );
          }
          return;
        }
        const global_skills = await getAvailableSkillsApi();
        if (!cancelled) {
          setAvailableSkills(buildCreateModeSkills(global_skills, enabledSkillNames));
        }
      } catch {
        if (!cancelled) {
          setAvailableSkills([]);
        }
      } finally {
        if (!cancelled) {
          setLoadingSkills(false);
        }
      }
    };

    void load_skills();
    return () => {
      cancelled = true;
    };
  }, [agentId, onEnabledSkillNamesChange]);

  const enabled_count = useMemo(
    () => availableSkills.filter((skill) => enabledSkillNames.includes(skill.name)).length,
    [availableSkills, enabledSkillNames],
  );

  const toggle_skill = (skill_name: string) => {
    onEnabledSkillNamesChange(
      enabledSkillNames.includes(skill_name)
        ? enabledSkillNames.filter((item) => item !== skill_name)
        : enabledSkillNames.concat(skill_name),
    );
  };

  return (
    <div className="space-y-8 animate-in slide-in-from-right-4 duration-300">
      <div className="space-y-4">
        <h3 className="text-[11px] font-semibold text-slate-600">
          Agent Skills
        </h3>

        <div className="modal-card radius-shell-md flex items-center justify-between p-4 transition-all hover:border-primary/20">
          <div className="flex-1">
            <label className="flex items-center gap-2 text-sm font-medium leading-none">
              启用技能系统
              {skillsEnabled && (
                <span className="rounded bg-green-500/10 px-1.5 py-0.5 text-[10px] font-medium text-green-600">
                  已启用
                </span>
              )}
            </label>
            <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
              技能先全局安装到 Nexus 技能库，再按 Agent 维度启用/停用。
            </p>
          </div>
          <label className="relative ml-4 inline-flex cursor-pointer items-center">
            <input
              type="checkbox"
              checked={skillsEnabled}
              onChange={(e) => onSkillsEnabledChange(e.target.checked)}
              className="peer sr-only"
            />
            <div className="h-6 w-11 rounded-full bg-muted peer peer-checked:bg-primary peer-focus:ring-2 peer-focus:ring-primary/20 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:after:translate-x-full peer-checked:after:border-white" />
          </label>
        </div>
      </div>

      <div className="space-y-4">
        <h3 className="text-[11px] font-semibold text-slate-600">
          设置加载来源
        </h3>

        <div className="radius-shell-md flex gap-3 border border-orange-500/20 bg-orange-500/10 p-4">
          <div className="mt-0.5 text-orange-600">
            <Sparkles className="h-4 w-4" />
          </div>
          <div>
            <p className="text-sm font-medium text-orange-700">
              来源同时影响技能与权限规则
            </p>
            <p className="mt-1 text-xs leading-relaxed text-orange-600/90">
              Nexus 会从这些来源读取配置。项目/本地设置里的权限规则只有在对应来源启用后，后续会话才会自动生效。
            </p>
          </div>
        </div>

        <div className="grid grid-cols-1 gap-3">
          {SOURCE_ITEMS.map((item) => {
            const isEnabled = settingSources.includes(item.key);
            return (
              <div
                key={item.key}
                className={cn(
                  "radius-shell-md flex items-center justify-between p-4 transition-all duration-200",
                  isEnabled
                    ? "modal-card-active bg-primary/5 ring-1 ring-primary/20"
                    : "modal-card hover:border-primary/20",
                )}
              >
                <div className="mr-4 flex-1">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    {item.label}
                    {isEnabled && (
                      <span className="rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary">
                        已启用
                      </span>
                    )}
                  </div>
                  <div className="mt-1 text-xs text-muted-foreground">
                    {item.description}
                  </div>
                </div>
                <label className="relative inline-flex cursor-pointer items-center">
                  <input
                    type="checkbox"
                    checked={isEnabled}
                    onChange={() => onToggleSettingSource(item.key)}
                    className="peer sr-only"
                  />
                  <div className="h-6 w-11 rounded-full bg-slate-200/80 peer peer-checked:bg-primary peer-focus:ring-2 peer-focus:ring-primary/20 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:border after:border-white/60 after:bg-white after:shadow-sm after:transition-all after:content-[''] peer-checked:after:translate-x-full peer-checked:after:border-white" />
                </label>
              </div>
            );
          })}
        </div>
      </div>

      <div className="space-y-4">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-[11px] font-semibold text-slate-600">
            当前 Agent 启用技能
          </h3>
          <span className="rounded-full bg-white/70 px-2 py-0.5 text-[11px] font-semibold text-slate-500">
            {enabled_count} / {availableSkills.length}
          </span>
        </div>

        {!skillsEnabled ? (
          <div className="modal-card radius-shell-md px-4 py-4 text-sm text-muted-foreground">
            先启用技能系统，下面的 Agent 技能开关才会生效。
          </div>
        ) : loadingSkills ? (
          <div className="modal-card radius-shell-md flex items-center justify-center px-4 py-6 text-slate-400">
            <Loader2 className="h-4 w-4 animate-spin" />
          </div>
        ) : availableSkills.length === 0 ? (
          <div className="modal-card radius-shell-md px-4 py-4 text-sm text-muted-foreground">
            还没有可启用的全局技能。请先去 Skills Marketplace 导入或安装技能。
          </div>
        ) : (
          <div className="space-y-3">
            {availableSkills.map((skill) => {
              const isEnabled = enabledSkillNames.includes(skill.name);
              return (
                <div
                  key={skill.name}
                  className={cn(
                    "radius-shell-md flex items-center justify-between gap-4 p-4 transition-all duration-200",
                    isEnabled
                      ? "modal-card-active bg-primary/5 ring-1 ring-primary/20"
                      : "modal-card hover:border-primary/20",
                  )}
                >
                  <div className="min-w-0 flex flex-1 items-start gap-3">
                    <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-emerald-50 text-emerald-600">
                      <Puzzle className="h-5 w-5" />
                    </div>
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="truncate text-sm font-semibold text-slate-950/90">
                          {skill.title}
                        </p>
                        {skill.version ? (
                          <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[10px] font-medium text-slate-500">
                            {skill.version}
                          </span>
                        ) : null}
                        {isEnabled ? (
                          <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] font-semibold text-emerald-600">
                            <Check className="h-3 w-3" />
                            已启用
                          </span>
                        ) : null}
                      </div>
                      <p className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">
                        {skill.description || "暂无描述"}
                      </p>
                    </div>
                  </div>
                  <label className="relative inline-flex cursor-pointer items-center">
                    <input
                      type="checkbox"
                      checked={isEnabled}
                      onChange={() => toggle_skill(skill.name)}
                      className="peer sr-only"
                    />
                    <div className="h-6 w-11 rounded-full bg-slate-200/80 peer peer-checked:bg-primary peer-focus:ring-2 peer-focus:ring-primary/20 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:border after:border-white/60 after:bg-white after:shadow-sm after:transition-all after:content-[''] peer-checked:after:translate-x-full peer-checked:after:border-white" />
                  </label>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
