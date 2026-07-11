import { useCallback, useEffect, useRef, useState } from "react";

import { getSkillDetailApi } from "@/lib/api/capability/skill-api";
import { getErrorMessage } from "@/lib/error-message";
import type { SkillInfo } from "@/types/capability/skill";

import type { SkillDetailSnapshot } from "./skill-detail-model";

type SkillDetailAction = "delete" | "update";

interface UseSkillDetailControllerOptions {
  deleteSkill: (skill: SkillInfo) => Promise<boolean>;
  onDeleted: () => Promise<void> | void;
  skillName: string;
  updateSkill: (skillName: string) => Promise<boolean>;
}

export function useSkillDetailController({
  deleteSkill,
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
  const requestGenerationRef = useRef(0);

  const loadDetail = useCallback(async () => {
    const generation = ++requestGenerationRef.current;
    setSnapshot({ errorMessage: null, skill: null, status: "loading" });
    try {
      const skill = await getSkillDetailApi(skillName);
      if (generation !== requestGenerationRef.current) return;
      setSnapshot({ errorMessage: null, skill, status: "ready" });
    } catch (error) {
      if (generation !== requestGenerationRef.current) return;
      setSnapshot({
        errorMessage: getErrorMessage(error, "加载 skill 详情失败"),
        skill: null,
        status: "error",
      });
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

  return {
    activeAction,
    deleteSkill: handleDelete,
    snapshot,
    updateSkill: handleUpdate,
  };
}
