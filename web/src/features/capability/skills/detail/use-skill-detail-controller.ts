import { useCallback, useEffect, useRef, useState } from "react";

import {
  getSkillAgentsApi,
  getSkillDetailApi,
  setAgentSkillEnabledApi,
} from "@/lib/api/capability/skill-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  SkillAgentBinding,
  SkillDetail,
  SkillInfo,
} from "@/types/capability/skill";

import {
  buildSkillAgentBindingsReadFailure,
  buildSkillAgentToggleFailure,
  buildSkillAgentToggleFollowupFailure,
  type SkillAgentBindingsReadFailure,
  type SkillAgentToggleFailure,
  type SkillDetailSnapshot,
} from "./skill-detail-model";

type SkillDetailAction = "delete" | "update" | "toggle";

interface UseSkillDetailControllerOptions {
  deleteSkill: (skill: SkillInfo) => Promise<boolean>;
  onAgentBindingChanged: () => Promise<void> | void;
  onDeleted: () => Promise<void> | void;
  skillName: string;
  updateSkill: (skillName: string) => Promise<boolean>;
}

export function useSkillDetailController({
  deleteSkill,
  onAgentBindingChanged,
  onDeleted,
  skillName,
  updateSkill,
}: UseSkillDetailControllerOptions) {
  const { t } = useI18n();
  const translationRef = useRef(t);
  translationRef.current = t;
  const expectedToggleRef = useRef(new Map<string, boolean>());
  const actionRef = useRef<SkillDetailAction | null>(null);
  const [snapshot, setSnapshot] = useState<SkillDetailSnapshot>({
    skill: null,
    status: "loading",
  });
  const [activeAction, setActiveAction] = useState<SkillDetailAction | null>(null);
  const [agentBindings, setAgentBindings] = useState<SkillAgentBinding[]>([]);
  const [agentsLoading, setAgentsLoading] = useState(true);
  const [busyAgentId, setBusyAgentId] = useState<string | null>(null);
  const [bindingsFailure, setBindingsFailure] = useState<
    SkillAgentBindingsReadFailure | null
  >(null);
  const [toggleFailures, setToggleFailures] = useState<Record<
    string,
    SkillAgentToggleFailure
  >>({});
  const requestGenerationRef = useRef(0);

  const loadBindings = useCallback(async (
    generation: number,
    targetSkillName: string,
  ) => {
    setAgentsLoading(true);
    setBindingsFailure(null);
    try {
      const bindings = await getSkillAgentsApi(targetSkillName);
      if (generation !== requestGenerationRef.current) return;
      setAgentBindings(bindings);
      setToggleFailures((current) => Object.fromEntries(Object.entries(current).flatMap(([agentId, failure]) => {
        const observed = bindings.find((binding) => binding.agent_id === agentId);
        if (!failure.blocksRepeat || failure.effect === "committed" || (observed && observed.enabled === expectedToggleRef.current.get(agentId))) return [];
        return [[agentId, { ...failure, canStartNewIntent: failure.effect === "unknown" }]];
      })));
    } catch (error) {
      if (generation !== requestGenerationRef.current) return;
      setBindingsFailure(buildSkillAgentBindingsReadFailure(error, translationRef.current));
    } finally {
      if (generation === requestGenerationRef.current) {
        setAgentsLoading(false);
      }
    }
  }, []);

  const loadDetail = useCallback(async () => {
    const generation = ++requestGenerationRef.current;
    setSnapshot({ skill: null, status: "loading" });
    setAgentBindings([]);
    setAgentsLoading(true);
    setBindingsFailure(null);
    let skill: SkillDetail;
    try {
      skill = await getSkillDetailApi(skillName);
      if (generation !== requestGenerationRef.current) return;
      setSnapshot({ skill, status: "ready" });
      if (skill.scope === "room") {
        setAgentsLoading(false);
        return;
      }
    } catch {
      if (generation !== requestGenerationRef.current) return;
      setSnapshot({ skill: null, status: "error" });
      setAgentsLoading(false);
      return;
    }
    await loadBindings(generation, skillName);
  }, [loadBindings, skillName]);

  const retryBindings = useCallback(async () => {
    if (actionRef.current || snapshot.status !== "ready" || snapshot.skill.scope === "room") {
      return;
    }
    const generation = ++requestGenerationRef.current;
    await loadBindings(generation, snapshot.skill.name);
  }, [loadBindings, snapshot]);

  useEffect(() => {
    void loadDetail();
    return () => {
      requestGenerationRef.current += 1;
    };
  }, [loadDetail]);

  useEffect(() => {
    setToggleFailures({});
  }, [skillName]);

  const handleUpdate = useCallback(async () => {
    if (snapshot.status !== "ready" || actionRef.current) return;
    actionRef.current = "update";
    setActiveAction("update");
    try {
      const succeeded = await updateSkill(snapshot.skill.name);
      if (succeeded) await loadDetail();
    } finally {
      actionRef.current = null;
      setActiveAction(null);
    }
  }, [loadDetail, snapshot, updateSkill]);

  const handleDelete = useCallback(async () => {
    if (
      snapshot.status !== "ready" ||
      !snapshot.skill.deletable ||
      actionRef.current
    ) return;
    actionRef.current = "delete";
    setActiveAction("delete");
    try {
      const succeeded = await deleteSkill(snapshot.skill);
      if (succeeded) await Promise.resolve(onDeleted());
    } finally {
      actionRef.current = null;
      setActiveAction(null);
    }
  }, [deleteSkill, onDeleted, snapshot]);

  const handleAgentToggle = useCallback(async (
    binding: SkillAgentBinding,
  ) => {
    if (snapshot.status !== "ready" || actionRef.current || snapshot.skill.locked) {
      return;
    }
    const existingFailure = toggleFailures[binding.agent_id];
    if (existingFailure?.blocksRepeat) {
      return;
    }
    actionRef.current = "toggle";
    expectedToggleRef.current.set(binding.agent_id, !binding.enabled);
    setActiveAction("toggle");
    setBusyAgentId(binding.agent_id);
    setToggleFailures((current) => {
      const next = { ...current };
      delete next[binding.agent_id];
      return next;
    });
    try {
      const updated = await setAgentSkillEnabledApi(
        binding.agent_id,
        snapshot.skill.name,
        !binding.enabled,
        "global_library",
      );
      setAgentBindings((current) => current.map((item) => (
        item.agent_id === binding.agent_id
          ? { ...item, enabled: updated.enabled_for_agent }
          : item
      )));
    } catch (error) {
      setToggleFailures((current) => ({
        ...current,
        [binding.agent_id]: buildSkillAgentToggleFailure(
          error,
          binding.agent_id,
          t,
        ),
      }));
      setBusyAgentId(null);
      actionRef.current = null;
      setActiveAction(null);
      return;
    }
    try {
      await Promise.resolve(onAgentBindingChanged());
    } catch (error) {
      setToggleFailures((current) => ({
        ...current,
        [binding.agent_id]: buildSkillAgentToggleFollowupFailure(
          error,
          binding.agent_id,
          t,
        ),
      }));
    } finally {
      setBusyAgentId(null);
      actionRef.current = null;
      setActiveAction(null);
    }
  }, [onAgentBindingChanged, snapshot, t, toggleFailures]);

  return {
    activeAction,
    agentBindings,
    agentsLoading,
    bindingsFailure,
    busyAgentId,
    deleteSkill: handleDelete,
    toggleAgent: handleAgentToggle,
    retry: loadDetail,
    retryBindings,
    startNewToggleIntent: (agentId: string) => {
      if (actionRef.current || !toggleFailures[agentId]?.canStartNewIntent) return;
      setToggleFailures((current) => {
        const next = { ...current };
        delete next[agentId];
        return next;
      });
    },
    snapshot,
    toggleFailures,
    updateSkill: handleUpdate,
  };
}
