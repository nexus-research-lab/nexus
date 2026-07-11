import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";

import {
  checkSkillUpdatesApi,
  deleteSkillApi,
  importExternalSkillApi,
  importGitSkillApi,
  importLocalSkillApi,
  updateSingleSkillApi,
} from "@/lib/api/capability/skill-api";
import type { ExternalSkillSearchItem, SkillInfo } from "@/types/capability/skill";

import { formatDeployFailureMessage } from "../detail/skill-deploy-failures";
import {
  externalSkillKey,
  type SkillImportDialogMode,
  type SkillMarketplaceFeedbackActions,
  type SkillOperationsController,
} from "./skill-marketplace-controller";

const UPDATE_CHECK_INTERVAL_MS = 24 * 60 * 60 * 1000;
const UPDATE_CHECK_MESSAGE_TTL_MS = 5000;
const UPDATE_CHECK_STORAGE_KEY = "nexus.skill_updates.last_checked_at";

interface UseSkillOperationsOptions {
  closeExternalPreview: () => void;
  feedback: SkillMarketplaceFeedbackActions;
  refreshCatalog: () => Promise<void>;
  updateAvailableCount: number;
}

function readLastUpdateCheckTime(): number | null {
  if (typeof window === "undefined") return null;
  const value = Number(window.localStorage.getItem(UPDATE_CHECK_STORAGE_KEY));
  return Number.isFinite(value) && value > 0 ? value : null;
}

function setBusyKey(
  setter: Dispatch<SetStateAction<ReadonlySet<string>>>,
  key: string,
  busy: boolean,
) {
  setter((current) => {
    const next = new Set(current);
    if (busy) next.add(key);
    else next.delete(key);
    return next;
  });
}

function buildUpdateCheckMessage(
  availableCount: number,
  failures: Array<{ skill_name: string }>,
  manual: boolean,
): string | null {
  const failureCount = failures.length;
  const failureLabel = failureCount === 1
    ? `${failures[0]?.skill_name || "1 个来源"}无法检查`
    : `${failureCount} 个来源无法检查`;
  const cases = [
    {
      matches: availableCount > 0 && failureCount > 0,
      message: `发现 ${availableCount} 个可更新，${failureLabel}`,
    },
    {
      matches: availableCount > 0,
      message: `发现 ${availableCount} 个可更新`,
    },
    {
      matches: failureCount > 0,
      message: `暂无可更新，${failureLabel}`,
    },
    { matches: manual, message: "暂无更新" },
  ];
  return cases.find((item) => item.matches)?.message ?? null;
}

export function useSkillOperations({
  closeExternalPreview,
  feedback,
  refreshCatalog,
  updateAvailableCount,
}: UseSkillOperationsOptions): SkillOperationsController {
  const [checkingUpdates, setCheckingUpdates] = useState(false);
  const [checkUpdateMessage, setCheckUpdateMessage] = useState<string | null>(null);
  const [lastUpdateCheckedAt, setLastUpdateCheckedAt] = useState<number | null>(readLastUpdateCheckTime);
  const [importing, setImporting] = useState(false);
  const [importDialogMode, setImportDialogMode] = useState<SkillImportDialogMode | null>(null);
  const [busySkillNames, setBusySkillNames] = useState<ReadonlySet<string>>(() => new Set());
  const [busyExternalKeys, setBusyExternalKeys] = useState<ReadonlySet<string>>(() => new Set());
  const checkingRef = useRef(false);
  const importingRef = useRef(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const recordUpdateCheck = useCallback(() => {
    const checkedAt = Date.now();
    window.localStorage.setItem(UPDATE_CHECK_STORAGE_KEY, String(checkedAt));
    setLastUpdateCheckedAt(checkedAt);
  }, []);

  const runUpdateCheck = useCallback(async (manual: boolean) => {
    if (checkingRef.current) return;
    if (manual) feedback.clear();
    checkingRef.current = true;
    setCheckingUpdates(true);
    try {
      const result = await checkSkillUpdatesApi();
      recordUpdateCheck();
      setCheckUpdateMessage(buildUpdateCheckMessage(
        result.available_skills.length,
        result.failures,
        manual,
      ));
      await refreshCatalog();
    } catch (error) {
      if (manual) {
        feedback.error(error instanceof Error ? error.message : "技能更新检查失败");
      } else {
        recordUpdateCheck();
      }
    } finally {
      checkingRef.current = false;
      setCheckingUpdates(false);
    }
  }, [feedback, recordUpdateCheck, refreshCatalog]);

  useEffect(() => {
    const now = Date.now();
    if (lastUpdateCheckedAt && now - lastUpdateCheckedAt < UPDATE_CHECK_INTERVAL_MS) return;
    void runUpdateCheck(false);
  }, [lastUpdateCheckedAt, runUpdateCheck]);

  useEffect(() => {
    if (!checkUpdateMessage || checkingUpdates || updateAvailableCount > 0) return;
    const timer = window.setTimeout(
      () => setCheckUpdateMessage(null),
      UPDATE_CHECK_MESSAGE_TTL_MS,
    );
    return () => window.clearTimeout(timer);
  }, [checkUpdateMessage, checkingUpdates, updateAvailableCount]);

  const updateSkill = useCallback(async (skillName: string) => {
    feedback.clear();
    setBusyKey(setBusySkillNames, skillName, true);
    try {
      const detail = await updateSingleSkillApi(skillName);
      const warning = formatDeployFailureMessage(skillName, detail.deploy_failures);
      if (warning) feedback.warning(warning);
      else feedback.success(`已更新 ${skillName}`);
      await refreshCatalog();
    } catch (error) {
      feedback.error(error instanceof Error ? error.message : "技能更新失败");
    } finally {
      setBusyKey(setBusySkillNames, skillName, false);
    }
  }, [feedback, refreshCatalog]);

  const deleteSkill = useCallback(async (skill: SkillInfo) => {
    feedback.clear();
    setBusyKey(setBusySkillNames, skill.name, true);
    try {
      await deleteSkillApi(skill.name);
      feedback.success(`${skill.title || skill.name} 已从技能库删除`);
      await refreshCatalog();
    } catch (error) {
      feedback.error(error instanceof Error ? error.message : "技能删除失败");
    } finally {
      setBusyKey(setBusySkillNames, skill.name, false);
    }
  }, [feedback, refreshCatalog]);

  const importLocal = useCallback(async (file: File) => {
    if (importingRef.current) return;
    importingRef.current = true;
    setImporting(true);
    feedback.start(`正在导入：${file.name}...`);
    try {
      await importLocalSkillApi(file);
      feedback.success(`已导入：${file.name}`);
      setImportDialogMode(null);
      await refreshCatalog();
    } catch (error) {
      feedback.error(error instanceof Error ? error.message : "技能导入失败");
    } finally {
      importingRef.current = false;
      setImporting(false);
    }
  }, [feedback, refreshCatalog]);

  const importGit = useCallback(async (
    url: string,
    branch?: string,
    path?: string,
  ) => {
    const normalizedUrl = url.trim();
    if (!normalizedUrl || importingRef.current) return;
    importingRef.current = true;
    setImporting(true);
    feedback.start("正在从 Git 拉取并导入 Skill...");
    try {
      await importGitSkillApi(
        normalizedUrl,
        branch?.trim() || undefined,
        path?.trim() || undefined,
      );
      feedback.success("已通过 Git 导入");
      setImportDialogMode(null);
      await refreshCatalog();
    } catch (error) {
      feedback.error(error instanceof Error ? error.message : "Git 技能导入失败");
    } finally {
      importingRef.current = false;
      setImporting(false);
    }
  }, [feedback, refreshCatalog]);

  const importExternal = useCallback(async (item: ExternalSkillSearchItem) => {
    const key = externalSkillKey(item);
    setBusyKey(setBusyExternalKeys, key, true);
    feedback.start(`正在导入：${item.skill_slug}...`);
    try {
      await importExternalSkillApi(item);
      feedback.success(`已导入：${item.skill_slug}`);
      await refreshCatalog();
      closeExternalPreview();
    } catch (error) {
      feedback.error(error instanceof Error ? error.message : "外部技能导入失败");
    } finally {
      setBusyKey(setBusyExternalKeys, key, false);
    }
  }, [closeExternalPreview, feedback, refreshCatalog]);

  return {
    busyExternalKeys,
    busySkillNames,
    checkUpdateMessage,
    checkUpdates: () => runUpdateCheck(true),
    checkingUpdates,
    deleteSkill,
    fileInputRef,
    importDialogMode,
    importExternal,
    importGit,
    importLocal,
    importing,
    lastUpdateCheckedAt,
    setImportDialogMode,
    updateSkill,
  };
}
