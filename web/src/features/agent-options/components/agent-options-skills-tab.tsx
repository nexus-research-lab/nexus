"use client";

import { useCallback, useDeferredValue, useEffect, useMemo, useState } from "react";
import { Loader2, Lock, Search } from "lucide-react";

import { get_agent_skills_api, install_skill_api, uninstall_skill_api } from "@/lib/skill-api";
import { get_dialog_action_class_name } from "@/shared/ui/dialog/dialog-styles";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { AgentSkillEntry } from "@/types/skill";

interface AgentOptionsSkillsTabProps {
  agent_id?: string;
  is_visible: boolean;
}

export function AgentOptionsSkillsTab({
  agent_id,
  is_visible,
}: AgentOptionsSkillsTabProps) {
  const { t } = useI18n();
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
      const data = await get_agent_skills_api(agent_id);
      setSkills(data);
    } catch (error) {
      setErrorMessage(
        error instanceof Error ? error.message : t("agent_options.skills.load_failed")
      );
    } finally {
      setLoading(false);
    }
  }, [agent_id, t]);

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
          await uninstall_skill_api(agent_id, skill.name);
        } else {
          await install_skill_api(agent_id, skill.name);
        }
        await loadSkills();
      } catch (error) {
        setErrorMessage(
          error instanceof Error ? error.message : t("agent_options.skills.toggle_failed")
        );
      } finally {
        setToggling(null);
      }
    },
    [agent_id, loadSkills, toggling, t]
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

  const render_skill_card = (
    skill: AgentSkillEntry,
    actionLabel: string,
    tone: "installed" | "add"
  ) => {
    const isBusy = toggling === skill.name;
    return (
      <div
        key={skill.name}
        className="flex items-start justify-between gap-3 rounded-[15px] border border-(--divider-subtle-color) bg-transparent px-3 py-2.5 transition-[background,border-color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)"
      >
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-[12.5px] font-semibold leading-[1.35] text-(--text-strong)">
              {skill.title || skill.name}
            </span>
            {skill.locked ? (
              <span className="inline-flex items-center gap-1 rounded-full border border-emerald-400/20 bg-emerald-500/8 px-2 py-0.5 text-[10px] font-semibold text-emerald-600">
                <Lock className="h-3 w-3" />
                {t("agent_options.skills.system_builtin")}
              </span>
            ) : null}
            {skill.scope === "main" ? (
              <span className="rounded-full border border-sky-400/20 bg-sky-500/8 px-2 py-0.5 text-[10px] font-semibold text-sky-600">
                {t("agent_options.skills.main_only")}
              </span>
            ) : null}
          </div>
          {skill.description ? (
            <p className="mt-0.5 line-clamp-2 text-[11.5px] leading-[1.5] text-(--text-muted)">
              {skill.description}
            </p>
          ) : null}
        </div>

        {skill.locked ? (
          <span className="inline-flex h-7 shrink-0 items-center rounded-full border border-emerald-400/20 bg-emerald-500/8 px-2.5 text-[11px] font-semibold text-emerald-600">
            {t("agent_options.skills.enabled")}
          </span>
        ) : (
          <button
            className={get_dialog_action_class_name(tone === "installed" ? "default" : "primary", "compact")}
            disabled={isBusy}
            onClick={() => void handleToggle(skill)}
            type="button"
          >
            {isBusy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : actionLabel}
          </button>
        )}
      </div>
    );
  };

  return (
    <div className="space-y-3.5 animate-in slide-in-from-right-4 duration-300">
      <div className="flex items-center justify-between gap-4">
        <h3 className="text-[16px] font-semibold tracking-tight text-(--text-strong)">
          {t("agent_options.skills.summary", { count: installedCount })}
        </h3>
        <span className="text-[12px] text-(--text-soft)">{t("agent_options.skills.total", { count: skills.length })}</span>
      </div>

      {errorMessage ? (
        <div className="rounded-[15px] border border-rose-400/20 bg-rose-500/10 px-3.5 py-2.5 text-[12px] text-rose-300">
          {errorMessage}
        </div>
      ) : null}

      {loading ? (
        <div className="flex items-center justify-center rounded-[15px] border border-(--divider-subtle-color) py-10 text-(--text-soft)">
          <Loader2 className="h-4 w-4 animate-spin" />
        </div>
      ) : null}

      {!agent_id ? (
        <div className="rounded-[15px] border border-(--divider-subtle-color) px-4 py-5 text-[12.5px] text-(--text-muted)">
          {t("agent_options.skills.create_first")}
        </div>
      ) : null}

      {agent_id && !loading ? (
        <>
          <div className="space-y-2.5">
            <div className="flex items-center justify-between gap-4">
              <h4 className="text-sm font-semibold text-(--text-strong)">{t("agent_options.skills.installed")}</h4>
              <span className="text-xs text-(--text-soft)">{installedSkills.length}</span>
            </div>

            {installedSkills.length === 0 ? (
              <div className="rounded-[15px] border border-(--divider-subtle-color) px-4 py-5 text-[12.5px] text-(--text-muted)">
                {t("agent_options.skills.empty_installed")}
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-1.5">
                {installedSkills.map((skill) => render_skill_card(skill, t("agent_options.skills.remove"), "installed"))}
              </div>
            )}
          </div>

          <div className="space-y-2.5">
            <div className="flex items-center justify-between gap-4">
              <h4 className="text-sm font-semibold text-(--text-strong)">{t("agent_options.skills.add")}</h4>
              <span className="text-xs text-(--text-soft)">
                {filteredAddableSkills.length}/{addableSkills.length}
              </span>
            </div>

            <div className="dialog-input flex items-center gap-2.5 rounded-[15px] px-3.5 py-2">
              <Search className="h-4 w-4 text-(--text-soft)" />
              <input
                type="text"
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder={t("agent_options.skills.search_placeholder")}
                className="w-full bg-transparent text-[13px] text-(--text-strong) outline-none placeholder:text-(--text-soft)"
              />
            </div>

            {skills.length === 0 ? (
              <div className="rounded-[15px] border border-(--divider-subtle-color) px-4 py-5 text-[12.5px] text-(--text-muted)">
                {t("agent_options.skills.empty_available")}
              </div>
            ) : null}

            {skills.length > 0 && filteredAddableSkills.length === 0 ? (
              <div className="rounded-[15px] border border-(--divider-subtle-color) px-4 py-5 text-[12.5px] text-(--text-muted)">
                {addableSkills.length === 0
                  ? t("agent_options.skills.empty_addable")
                  : t("agent_options.skills.empty_search")}
              </div>
            ) : null}

            {filteredAddableSkills.length > 0 ? (
              <div className="grid grid-cols-1 gap-1.5">
                {filteredAddableSkills.map((skill) => render_skill_card(skill, t("agent_options.skills.add_button"), "add"))}
              </div>
            ) : null}
          </div>
        </>
      ) : null}
    </div>
  );
}
