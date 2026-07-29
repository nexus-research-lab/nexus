import { useCallback, useEffect, useRef, useState } from "react";

import {
  getSkillAgentsApi,
  getSkillDetailApi,
  setAgentSkillEnabledApi,
} from "@/lib/api/capability/skill-api";
import { getErrorMessage } from "@/lib/error-message";
import type {
  SkillAgentBinding,
  SkillDetail,
  SkillInfo,
} from "@/types/capability/skill";

import type { SkillDetailSnapshot } from "./skill-detail-model";

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
  const [snapshot, setSnapshot] = useState<SkillDetailSnapshot>({
    errorMessage: null,
    skill: null,
    status: "loading",
  });
  const [activeAction, setActiveAction] = useState<SkillDetailAction | null>(null);
  const [agentBindings, setAgentBindings] = useState<SkillAgentBinding[]>([]);
  const [agentsLoading, setAgentsLoading] = useState(true);
  const [busyAgentId, setBusyAgentId] = useState<string | null>(null);
  const [agentToggleError, setAgentToggleError] = useState<string | null>(null);
  const requestGenerationRef = useRef(0);

  const loadDetail = useCallback(async () => {
    const generation = ++requestGenerationRef.current;
    setSnapshot({ errorMessage: null, skill: null, status: "loading" });
    setAgentBindings([]);
    setAgentsLoading(true);
    setAgentToggleError(null);
    let skill: SkillDetail;
    try {
      skill = await getSkillDetailApi(skillName);
      if (generation !== requestGenerationRef.current) return;
      setSnapshot({ errorMessage: null, skill, status: "ready" });
      if (skill.scope === "room") {
        setAgentsLoading(false);
        return;
      }
    } catch (error) {
      if (generation !== requestGenerationRef.current) return;
      setSnapshot({
        errorMessage: getErrorMessage(error, "加载 skill 详情失败"),
        skill: null,
        status: "error",
      });
      setAgentsLoading(false);
      return;
    }
    try {
      const bindings = await getSkillAgentsApi(skillName);
      if (generation !== requestGenerationRef.current) return;
      setAgentBindings(bindings);
    } catch (error) {
      if (generation !== requestGenerationRef.current) return;
      setAgentToggleError(getErrorMessage(error, "加载 Agent 使用状态失败"));
    } finally {
      if (generation === requestGenerationRef.current) {
        setAgentsLoading(false);
      }
    }
  }, [skillName]);

  useEffect(() => {
    void loadDetail();
    return () => {
      requestGenerationRef.current += 1;
    };
  }, [loadDetail]);

  const handleUpdate = useCallback(async () => {
    if (snapshot.status !== "ready" || activeAction) return;
    setActiveAction("update");
    try {
      const succeeded = await updateSkill(snapshot.skill.name);
      if (succeeded) await loadDetail();
    } finally {
      setActiveAction(null);
    }
  }, [activeAction, loadDetail, snapshot, updateSkill]);

  const handleDelete = useCallback(async () => {
    if (
      snapshot.status !== "ready" ||
      !snapshot.skill.deletable ||
      activeAction
    ) return;
    setActiveAction("delete");
    try {
      const succeeded = await deleteSkill(snapshot.skill);
      if (succeeded) await Promise.resolve(onDeleted());
    } finally {
      setActiveAction(null);
    }
  }, [activeAction, deleteSkill, onDeleted, snapshot]);

  const handleAgentToggle = useCallback(async (
    binding: SkillAgentBinding,
  ) => {
    if (snapshot.status !== "ready" || activeAction || snapshot.skill.locked) {
      return;
    }
    setActiveAction("toggle");
    setBusyAgentId(binding.agent_id);
    setAgentToggleError(null);
    try {
      await setAgentSkillEnabledApi(
        binding.agent_id,
        snapshot.skill.name,
        !binding.enabled,
        "global_library",
      );
      setAgentBindings((current) => current.map((item) => (
        item.agent_id === binding.agent_id
          ? { ...item, enabled: !binding.enabled }
          : item
      )));
      await Promise.resolve(onAgentBindingChanged());
    } catch (error) {
      setAgentToggleError(getErrorMessage(error, "更新 Agent 技能状态失败"));
    } finally {
      setBusyAgentId(null);
      setActiveAction(null);
    }
  }, [activeAction, onAgentBindingChanged, snapshot]);

  return {
    activeAction,
    agentBindings,
    agentToggleError,
    agentsLoading,
    busyAgentId,
    deleteSkill: handleDelete,
    toggleAgent: handleAgentToggle,
    snapshot,
    updateSkill: handleUpdate,
  };
}
