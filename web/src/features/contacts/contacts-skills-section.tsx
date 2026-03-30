"use client";

import { useCallback, useEffect, useState } from "react";
import { Lock, Sparkles, Loader2 } from "lucide-react";

import { AgentSkillEntry } from "@/types/skill";
import { getAgentSkillsApi, installSkillApi, uninstallSkillApi } from "@/lib/skill-api";
import { WorkspaceInspectorSection } from "@/shared/ui/workspace-inspector-section";

interface ContactsSkillsSectionProps {
  agent_id: string;
}

export function ContactsSkillsSection({ agent_id }: ContactsSkillsSectionProps) {
  const [skills, setSkills] = useState<AgentSkillEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [toggling, setToggling] = useState<string | null>(null);

  const loadSkills = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getAgentSkillsApi(agent_id);
      setSkills(data);
    } catch (err) {
      console.error("[Skills] Failed to load:", err);
    } finally {
      setLoading(false);
    }
  }, [agent_id]);

  useEffect(() => {
    loadSkills();
  }, [loadSkills]);

  const handleToggle = async (skill: AgentSkillEntry) => {
    if (skill.locked || toggling) return;
    setToggling(skill.name);
    try {
      if (skill.installed) {
        await uninstallSkillApi(agent_id, skill.name);
      } else {
        await installSkillApi(agent_id, skill.name);
      }
      await loadSkills();
    } catch (err) {
      console.error("[Skills] Toggle failed:", err);
    } finally {
      setToggling(null);
    }
  };

  const installed_count = skills.filter((s) => s.installed).length;

  const action_badge = (
    <span className="workspace-chip rounded-full px-2 py-0.5 text-[10px] font-bold text-slate-700/68">
      {installed_count}/{skills.length}
    </span>
  );

  return (
    <WorkspaceInspectorSection icon={Sparkles} title="Skills" action={action_badge}>
      {loading ? (
        <div className="flex items-center justify-center py-6 text-slate-400">
          <Loader2 className="h-4 w-4 animate-spin" />
        </div>
      ) : skills.length === 0 ? (
        <div className="workspace-card rounded-[18px] px-4 py-4 text-[13px] leading-6 text-slate-700/60">
          暂无可用技能。
        </div>
      ) : (
        <div className="space-y-2">
          {skills.map((skill) => (
            <div
              key={skill.name}
              className="workspace-card rounded-[18px] px-4 py-3"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-[13px] font-semibold text-slate-950/88">
                      {skill.title || skill.name}
                    </span>
                    {skill.locked && (
                      <Lock className="h-3 w-3 flex-shrink-0 text-slate-400" />
                    )}
                  </div>
                  {skill.description && (
                    <p className="mt-1 line-clamp-2 text-[11px] leading-4 text-slate-600/72">
                      {skill.description}
                    </p>
                  )}
                </div>
                <div className="flex-shrink-0 pt-0.5">
                  {skill.locked ? (
                    <span className="inline-flex items-center rounded-full bg-emerald-50 px-2.5 py-1 text-[10px] font-semibold text-emerald-600">
                      已启用
                    </span>
                  ) : (
                    <button
                      onClick={() => handleToggle(skill)}
                      disabled={toggling === skill.name}
                      className={
                        "inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[10px] font-semibold transition-all duration-200 disabled:opacity-60 " +
                        (skill.installed
                          ? "bg-emerald-50 text-emerald-600 hover:bg-red-50 hover:text-red-500"
                          : "workspace-chip text-slate-600 hover:bg-sky-50 hover:text-sky-600")
                      }
                    >
                      {toggling === skill.name ? (
                        <Loader2 className="h-3 w-3 animate-spin" />
                      ) : skill.installed ? (
                        "已启用"
                      ) : (
                        "启用"
                      )}
                    </button>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </WorkspaceInspectorSection>
  );
}
