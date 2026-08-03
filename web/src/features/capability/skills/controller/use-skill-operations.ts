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
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ExternalSkillSearchItem, SkillInfo } from "@/types/capability/skill";

import { formatDeployFailureMessage } from "../detail/skill-deploy-failures";
import { externalSkillKey } from "../external/external-skill-model";
import {
  type SkillImportDialogMode,
  type SkillMarketplaceFeedbackActions,
  type SkillOperationsController,
} from "./skill-marketplace-controller";
import {
  buildSkillUpdateCheckNotice,
  type SkillUpdateCheckNotice,
} from "./skill-update-check-model";

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

export function useSkillOperations({
  closeExternalPreview,
  feedback,
  refreshCatalog,
  updateAvailableCount,
}: UseSkillOperationsOptions): SkillOperationsController {
  const { locale, t } = useI18n();
  const [checkingUpdates, setCheckingUpdates] = useState(false);
  const [checkUpdateNotice, setCheckUpdateNotice] =
    useState<SkillUpdateCheckNotice | null>(null);
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
      setCheckUpdateNotice(buildSkillUpdateCheckNotice(
        result.available_skills.length,
        result.failures,
        manual,
      ));
      await refreshCatalog();
    } catch (error) {
      if (manual) {
        feedback.error(getErrorMessage(
          error,
          t("capability.skills_update_check_failed"),
        ));
      } else {
        recordUpdateCheck();
      }
    } finally {
      checkingRef.current = false;
      setCheckingUpdates(false);
    }
  }, [feedback, recordUpdateCheck, refreshCatalog, t]);

  useEffect(() => {
    const now = Date.now();
    if (lastUpdateCheckedAt && now - lastUpdateCheckedAt < UPDATE_CHECK_INTERVAL_MS) return;
    void runUpdateCheck(false);
  }, [lastUpdateCheckedAt, runUpdateCheck]);

  useEffect(() => {
    if (!checkUpdateNotice || checkingUpdates || updateAvailableCount > 0) return;
    const timer = window.setTimeout(
      () => setCheckUpdateNotice(null),
      UPDATE_CHECK_MESSAGE_TTL_MS,
    );
    return () => window.clearTimeout(timer);
  }, [checkUpdateNotice, checkingUpdates, updateAvailableCount]);

  const updateSkill = useCallback(async (skillName: string) => {
    feedback.clear();
    setBusyKey(setBusySkillNames, skillName, true);
    try {
      const detail = await updateSingleSkillApi(skillName);
      const warning = formatDeployFailureMessage(
        skillName,
        detail.deploy_failures,
        { locale, t },
      );
      if (warning) feedback.warning(warning);
      else feedback.success(t("capability.skills_updated", { name: skillName }));
      await refreshCatalog();
      return true;
    } catch (error) {
      feedback.error(getErrorMessage(
        error,
        t("capability.skills_update_failed"),
      ));
      return false;
    } finally {
      setBusyKey(setBusySkillNames, skillName, false);
    }
  }, [feedback, locale, refreshCatalog, t]);

  const deleteSkill = useCallback(async (skill: SkillInfo) => {
    feedback.clear();
    setBusyKey(setBusySkillNames, skill.name, true);
    try {
      await deleteSkillApi(skill.name);
      feedback.success(t("capability.skills_removed", {
        name: skill.title || skill.name,
      }));
      await refreshCatalog();
      return true;
    } catch (error) {
      feedback.error(getErrorMessage(
        error,
        t("capability.skills_delete_failed"),
      ));
      return false;
    } finally {
      setBusyKey(setBusySkillNames, skill.name, false);
    }
  }, [feedback, refreshCatalog, t]);

  const importLocal = useCallback(async (file: File) => {
    if (importingRef.current) return;
    importingRef.current = true;
    setImporting(true);
    feedback.start(t("capability.skills_importing_file", { name: file.name }));
    try {
      await importLocalSkillApi(file);
      feedback.success(t("capability.skills_imported_file", { name: file.name }));
      setImportDialogMode(null);
      await refreshCatalog();
    } catch (error) {
      feedback.error(getErrorMessage(
        error,
        t("capability.skills_import_failed"),
      ));
    } finally {
      importingRef.current = false;
      setImporting(false);
    }
  }, [feedback, refreshCatalog, t]);

  const importGit = useCallback(async (
    url: string,
    branch?: string,
    path?: string,
  ) => {
    const normalizedUrl = url.trim();
    if (!normalizedUrl || importingRef.current) return;
    importingRef.current = true;
    setImporting(true);
    feedback.start(t("capability.skills_git_importing"));
    try {
      await importGitSkillApi(
        normalizedUrl,
        branch?.trim() || undefined,
        path?.trim() || undefined,
      );
      feedback.success(t("capability.skills_git_imported"));
      setImportDialogMode(null);
      await refreshCatalog();
    } catch (error) {
      feedback.error(getErrorMessage(
        error,
        t("capability.skills_git_import_failed"),
      ));
    } finally {
      importingRef.current = false;
      setImporting(false);
    }
  }, [feedback, refreshCatalog, t]);

  const importExternal = useCallback(async (item: ExternalSkillSearchItem) => {
    const key = externalSkillKey(item);
    setBusyKey(setBusyExternalKeys, key, true);
    feedback.start(t("capability.skills_importing_file", {
      name: item.skill_slug,
    }));
    try {
      await importExternalSkillApi(item);
      feedback.success(t("capability.skills_imported_file", {
        name: item.skill_slug,
      }));
      await refreshCatalog();
      closeExternalPreview();
    } catch (error) {
      feedback.error(getErrorMessage(
        error,
        t("capability.skills_external_import_failed"),
      ));
    } finally {
      setBusyKey(setBusyExternalKeys, key, false);
    }
  }, [closeExternalPreview, feedback, refreshCatalog, t]);

  return {
    busyExternalKeys,
    busySkillNames,
    checkUpdateNotice,
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
