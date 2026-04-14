"use client";

import { useCallback, useDeferredValue, useEffect, useMemo, useState } from "react";
import { Loader2, Lock, Search } from "lucide-react";

import { getAgentSkillsApi, installSkillApi, uninstallSkillApi } from "@/lib/skill-api";
import { getDialogActionClassName } from "@/shared/ui/dialog/dialog-styles";
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
      const data = await getAgentSkillsApi(agent_id);
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
          await uninstallSkillApi(agent_id, skill.name);
        } else {
          await installSkillApi(agent_id, skill.name);
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

  const renderSkillCard = (
    skill: AgentSkillEntry,
    actionLabel: string,
    tone: "installed" | "add"
  ) => {
    const isBusy = toggling === skill.name;
    return (
      <div
        key={skill.name}
        className="dialog-card rounded-[20px] px-4 py-3.5 transition-all duration-(--motion-duration-normal) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)"
      >
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-semibold text-(--text-strong)">
                {skill.title || skill.name}
              </span>
              {skill.locked ? (
                <span className="inline-flex items-center gap-1 rounded-full border border-emerald-400/20 bg-emerald-500/10 px-2 py-0.5 text-[10px] font-semibold text-emerald-300">
                  <Lock className="h-3 w-3" />
                  {t("agent_options.skills.system_builtin")}
                </span>
              ) : null}
              {skill.scope === "main" ? (
                <span className="rounded-full border border-sky-400/20 bg-sky-500/10 px-2 py-0.5 text-[10px] font-semibold text-sky-300">
                  {t("agent_options.skills.main_only")}
                </span>
              ) : null}
            </div>
            {skill.description ? (
              <p className="mt-2 line-clamp-1 text-sm leading-6 text-(--text-muted)">
                {skill.description}
              </p>
            ) : null}
          </div>

          {skill.locked ? (
            <span className="inline-flex h-9 items-center rounded-full bg-emerald-50 px-3 text-xs font-semibold text-emerald-600">
              {t("agent_options.skills.enabled")}
            </span>
          ) : (
            <button
              className={getDialogActionClassName(tone === "installed" ? "default" : "primary")}
              disabled={isBusy}
              onClick={() => void handleToggle(skill)}
              type="button"
            >
              {isBusy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : actionLabel}
            </button>
          )}
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-5 animate-in slide-in-from-right-4 duration-300">
      <div className="flex items-center justify-between gap-4">
        <h3 className="text-[18px] font-semibold tracking-tight text-(--text-strong)">
          {t("agent_options.skills.summary", { count: installedCount })}
        </h3>
        <span className="text-sm text-(--text-soft)">{t("agent_options.skills.total", { count: skills.length })}</span>
      </div>

      {errorMessage ? (
        <div className="radius-shell-md border border-rose-400/20 bg-rose-500/10 px-4 py-3 text-sm text-rose-300">
          {errorMessage}
        </div>
      ) : null}

      {loading ? (
        <div className="surface-card radius-shell-md flex items-center justify-center py-12 text-(--text-soft)">
          <Loader2 className="h-4 w-4 animate-spin" />
        </div>
      ) : null}

      {!agent_id ? (
        <div className="surface-card radius-shell-md px-5 py-6 text-sm text-(--text-muted)">
          {t("agent_options.skills.create_first")}
        </div>
      ) : null}

      {agent_id && !loading ? (
        <>
          <div className="space-y-4">
            <div className="flex items-center justify-between gap-4">
              <h4 className="text-sm font-semibold text-(--text-strong)">{t("agent_options.skills.installed")}</h4>
              <span className="text-xs text-(--text-soft)">{installedSkills.length}</span>
            </div>

            {installedSkills.length === 0 ? (
              <div className="surface-card radius-shell-md px-5 py-6 text-sm text-(--text-muted)">
                {t("agent_options.skills.empty_installed")}
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-3">
                {installedSkills.map((skill) => renderSkillCard(skill, t("agent_options.skills.remove"), "installed"))}
              </div>
            )}
          </div>

          <div className="space-y-4">
            <div className="flex items-center justify-between gap-4">
              <h4 className="text-sm font-semibold text-(--text-strong)">{t("agent_options.skills.add")}</h4>
              <span className="text-xs text-(--text-soft)">
                {filteredAddableSkills.length}/{addableSkills.length}
              </span>
            </div>

            <div className="surface-card radius-shell-md p-4">
              <div className="dialog-input flex items-center gap-3 rounded-[18px] px-4 py-3">
                <Search className="h-4 w-4 text-(--text-soft)" />
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(event) => setSearchQuery(event.target.value)}
                  placeholder={t("agent_options.skills.search_placeholder")}
                  className="w-full bg-transparent text-sm text-(--text-strong) outline-none placeholder:text-(--text-soft)"
                />
              </div>
            </div>

            {skills.length === 0 ? (
              <div className="surface-card radius-shell-md px-5 py-6 text-sm text-(--text-muted)">
                {t("agent_options.skills.empty_available")}
              </div>
            ) : null}

            {skills.length > 0 && filteredAddableSkills.length === 0 ? (
              <div className="surface-card radius-shell-md px-5 py-6 text-sm text-(--text-muted)">
                {addableSkills.length === 0
                  ? t("agent_options.skills.empty_addable")
                  : t("agent_options.skills.empty_search")}
              </div>
            ) : null}

            {filteredAddableSkills.length > 0 ? (
              <div className="grid grid-cols-1 gap-3">
                {filteredAddableSkills.map((skill) => renderSkillCard(skill, t("agent_options.skills.add_button"), "add"))}
              </div>
            ) : null}
          </div>
        </>
      ) : null}
    </div>
  );
}
