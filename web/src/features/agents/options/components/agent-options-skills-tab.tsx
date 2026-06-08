"use client";

import { useCallback, useDeferredValue, useEffect, useMemo, useState } from "react";
import { Loader2, Lock, RefreshCw } from "lucide-react";

import { get_agent_skills_api, install_skill_api, uninstall_skill_api } from "@/lib/api/skill-api";
import { UiBadge } from "@/shared/ui/badge";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { UiSearchInput } from "@/shared/ui/form-control";
import { UiStateBlock } from "@/shared/ui/state-block";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { AgentSkillEntry } from "@/types/capability/skill";

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
  const [pendingRemoveSkill, setPendingRemoveSkill] = useState<AgentSkillEntry | null>(null);
  const deferredSearchQuery = useDeferredValue(searchQuery);

  const loadSkills = useCallback(async (silent = false) => {
    if (!agent_id) {
      setSkills([]);
      setErrorMessage(null);
      return;
    }

    try {
      if (!silent) {
        setLoading(true);
      }
      setErrorMessage(null);
      const data = await get_agent_skills_api(agent_id);
      setSkills(data);
    } catch (error) {
      setErrorMessage(
        error instanceof Error ? error.message : t("agent_options.skills.load_failed")
      );
    } finally {
      if (!silent) {
        setLoading(false);
      }
    }
  }, [agent_id, t]);

  useEffect(() => {
    if (!is_visible) {
      return;
    }
    void loadSkills(false);

    const refresh_silently = () => {
      if (document.hidden) {
        return;
      }
      void loadSkills(true);
    };
    const handle_visibility_change = () => {
      if (!document.hidden) {
        refresh_silently();
      }
    };
    const interval_id = window.setInterval(refresh_silently, 5000);

    window.addEventListener("focus", refresh_silently);
    document.addEventListener("visibilitychange", handle_visibility_change);
    return () => {
      window.clearInterval(interval_id);
      window.removeEventListener("focus", refresh_silently);
      document.removeEventListener("visibilitychange", handle_visibility_change);
    };
  }, [is_visible, loadSkills]);

  const handleRefresh = useCallback(() => {
    void loadSkills(false);
  }, [loadSkills]);

  const runSkillToggle = useCallback(
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

  const handleSkillAction = useCallback(
    (skill: AgentSkillEntry) => {
      if (skill.installed) {
        setPendingRemoveSkill(skill);
        return;
      }
      void runSkillToggle(skill);
    },
    [runSkillToggle]
  );

  const handleConfirmRemove = useCallback(() => {
    if (!pendingRemoveSkill) {
      return;
    }
    const skill = pendingRemoveSkill;
    setPendingRemoveSkill(null);
    void runSkillToggle(skill);
  }, [pendingRemoveSkill, runSkillToggle]);

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
    const isWorkspaceLocal = skill.source_type === "workspace";
    const isSystemManaged = skill.source_type === "system";
    return (
      <div
        key={skill.name}
        className="flex h-[92px] items-start justify-between gap-3 rounded-[12px] border border-(--divider-subtle-color) bg-transparent px-3 py-2.5 transition-[background,border-color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)"
      >
        <div className="min-w-0 flex-1 overflow-hidden">
          <div className="flex min-w-0 items-center gap-1.5">
            <span className="min-w-0 truncate text-[12.5px] font-semibold leading-[1.35] text-(--text-strong)">
              {skill.title || skill.name}
            </span>
            {isSystemManaged ? (
              <UiBadge class_name="shrink-0" icon={<Lock className="h-3 w-3" />} size="xs" tone="success">
                {t("agent_options.skills.system_builtin")}
              </UiBadge>
            ) : null}
            {isWorkspaceLocal ? (
              <UiBadge class_name="shrink-0" size="xs" tone="warning">
                {t("agent_options.skills.agent_workspace_only")}
              </UiBadge>
            ) : null}
            {skill.scope === "main" ? (
              <UiBadge class_name="shrink-0" size="xs" tone="info">
                {t("agent_options.skills.main_only")}
              </UiBadge>
            ) : null}
          </div>
          {skill.description ? (
            <p className="mt-0.5 line-clamp-2 text-[11.5px] leading-normal text-(--text-muted)">
              {skill.description}
            </p>
          ) : null}
        </div>

        {skill.locked ? (
          <UiBadge class_name="mt-auto mb-auto shrink-0" size="xs" tone="success">
            {t("agent_options.skills.enabled")}
          </UiBadge>
        ) : (
          <UiButton
            class_name="mt-auto mb-auto shrink-0"
            disabled={!!toggling}
            onClick={() => handleSkillAction(skill)}
            size="sm"
            tone={tone === "installed" ? "default" : "primary"}
            type="button"
            variant="surface"
          >
            {isBusy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : actionLabel}
          </UiButton>
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
        <div className="flex items-center gap-2">
          <span className="text-[12px] text-(--text-soft)">{t("agent_options.skills.total", { count: skills.length })}</span>
          <UiIconButton
            aria-label={t("capability.refresh")}
            disabled={!agent_id || loading}
            onClick={handleRefresh}
            size="sm"
            title={t("capability.refresh")}
            tone="default"
            variant="ghost"
          >
            <RefreshCw className={loading ? "h-3.5 w-3.5 animate-spin" : "h-3.5 w-3.5"} />
          </UiIconButton>
        </div>
      </div>

      {errorMessage ? (
        <UiStateBlock description={errorMessage} size="sm" title="加载失败" tone="danger" variant="inset" />
      ) : null}

      {loading ? (
        <UiStateBlock
          class_name="py-10"
          icon={<Loader2 className="h-4 w-4 animate-spin" />}
          size="sm"
          variant="inset"
        />
      ) : null}

      {!agent_id ? (
        <UiStateBlock
          description={t("agent_options.skills.create_first")}
          size="sm"
          variant="inset"
        />
      ) : null}

      {agent_id && !loading ? (
        <>
          <div className="space-y-2.5">
            <div className="flex items-center justify-between gap-4">
              <h4 className="text-sm font-semibold text-(--text-strong)">{t("agent_options.skills.installed")}</h4>
              <span className="text-xs text-(--text-soft)">{installedSkills.length}</span>
            </div>

            {installedSkills.length === 0 ? (
              <UiStateBlock
                description={t("agent_options.skills.empty_installed")}
                size="sm"
                variant="inset"
              />
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

            <UiSearchInput
              control_size="md"
              on_change={setSearchQuery}
              placeholder={t("agent_options.skills.search_placeholder")}
              value={searchQuery}
              variant="dialog"
            />

            {skills.length === 0 ? (
              <UiStateBlock
                description={t("agent_options.skills.empty_available")}
                size="sm"
                variant="inset"
              />
            ) : null}

            {skills.length > 0 && filteredAddableSkills.length === 0 ? (
              <UiStateBlock
                description={
                  addableSkills.length === 0
                    ? t("agent_options.skills.empty_addable")
                    : t("agent_options.skills.empty_search")
                }
                size="sm"
                variant="inset"
              />
            ) : null}

            {filteredAddableSkills.length > 0 ? (
              <div className="grid grid-cols-1 gap-1.5">
                {filteredAddableSkills.map((skill) => render_skill_card(skill, t("agent_options.skills.add_button"), "add"))}
              </div>
            ) : null}
          </div>
        </>
      ) : null}

      <ConfirmDialog
        confirm_text={t("agent_options.skills.remove_confirm_action")}
        is_open={!!pendingRemoveSkill}
        message={t("agent_options.skills.remove_confirm_message", {
          name: pendingRemoveSkill?.title || pendingRemoveSkill?.name || "",
        })}
        on_cancel={() => setPendingRemoveSkill(null)}
        on_confirm={handleConfirmRemove}
        title={t("agent_options.skills.remove_confirm_title")}
        variant="danger"
      />
    </div>
  );
}
