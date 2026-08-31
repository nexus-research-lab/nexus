// INPUT: Skill 写 API、当前目录快照、只读刷新与页面反馈入口。
// OUTPUT: 导入/更新/删除/检查命令，以及 exact 意图锁和只读对账恢复。
// POS: Skill marketplace 写事务控制器；写响应和目录刷新分阶段，unknown 永不自动重放。
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
  getSkillDetailApi,
  importExternalSkillApi,
  importGitSkillApi,
  importLocalSkillApi,
  updateSingleSkillApi,
} from "@/lib/api/capability/skill-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import {
  projectMutationFailure,
  type MutationFailureEffect,
} from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  ExternalSkillSearchItem,
  SkillDetail,
  SkillInfo,
} from "@/types/capability/skill";

import { formatDeployFailureMessage } from "../detail/skill-deploy-failures";
import { externalSkillKey } from "../external/external-skill-model";
import {
  type SkillImportDialogMode,
  type SkillMarketplaceFeedbackActions,
  type SkillOperationsController,
} from "./skill-marketplace-controller";
import {
  reconcileSkillOperation,
  skillOperationIntentKey,
  skillOperationTargetName,
  type SkillOperationIntent,
} from "./skill-operation-recovery";
import {
  buildSkillUpdateCheckNotice,
  type SkillUpdateCheckNotice,
} from "./skill-update-check-model";

const UPDATE_CHECK_INTERVAL_MS = 24 * 60 * 60 * 1000;
const UPDATE_CHECK_MESSAGE_TTL_MS = 5000;
const UPDATE_CHECK_STORAGE_KEY = "nexus.skill_updates.last_checked_at";

type PendingRecoveryEffect = Exclude<MutationFailureEffect, "not_applied">;
type RecoveryResult = "applied" | "failed" | "stale" | "unproven";

interface PendingSkillRecovery {
  committedUIApplied: boolean;
  effect: PendingRecoveryEffect;
  intent: SkillOperationIntent;
  key: string;
  onCommitted?: () => void;
  presentSuccess: () => void;
  reason: string;
}

interface ExecuteSkillMutationOptions<T> {
  failureFallback: string;
  intent: SkillOperationIntent;
  mutate: () => Promise<T>;
  onCommitted?: () => void;
  onResponse?: (result: T) => void;
  presentCommitted: () => void;
  presentSuccess: (result: T) => void;
}

interface UseSkillOperationsOptions {
  catalogSkills: SkillInfo[];
  closeExternalPreview: () => void;
  feedback: SkillMarketplaceFeedbackActions;
  refreshCatalog: () => Promise<boolean>;
  updateAvailableCount: number;
}

function readLastUpdateCheckTime(): number | null {
  if (typeof window === "undefined") return null;
  try {
    const value = Number(window.localStorage.getItem(UPDATE_CHECK_STORAGE_KEY));
    return Number.isFinite(value) && value > 0 ? value : null;
  } catch {
    return null;
  }
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

function applyCommittedUI(recovery: PendingSkillRecovery) {
  if (recovery.committedUIApplied) return;
  recovery.committedUIApplied = true;
  recovery.onCommitted?.();
}

async function readSkillTarget(
  skillName: string,
): Promise<{ detail: SkillDetail | null; readable: true } | { readable: false }> {
  try {
    return {
      detail: await getSkillDetailApi(skillName),
      readable: true,
    };
  } catch (error) {
    if (error instanceof ApiRequestError && error.status === 404) {
      return { detail: null, readable: true };
    }
    return { readable: false };
  }
}

export function useSkillOperations({
  catalogSkills,
  closeExternalPreview,
  feedback,
  refreshCatalog,
  updateAvailableCount,
}: UseSkillOperationsOptions): SkillOperationsController {
  const { locale, t } = useI18n();
  const [checkingUpdates, setCheckingUpdates] = useState(false);
  const [checkUpdateNotice, setCheckUpdateNotice] =
    useState<SkillUpdateCheckNotice | null>(null);
  const [lastUpdateCheckedAt, setLastUpdateCheckedAt] = useState<number | null>(
    readLastUpdateCheckTime,
  );
  const [importing, setImporting] = useState(false);
  const [importDialogMode, setImportDialogMode] =
    useState<SkillImportDialogMode | null>(null);
  const [busySkillNames, setBusySkillNames] =
    useState<ReadonlySet<string>>(() => new Set());
  const [busyExternalKeys, setBusyExternalKeys] =
    useState<ReadonlySet<string>>(() => new Set());
  const checkingRef = useRef(false);
  const importingRef = useRef(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const activeIntentKeysRef = useRef(new Set<string>());
  const pendingRecoveriesRef = useRef(new Map<string, PendingSkillRecovery>());
  const reconcileRecoveryRef = useRef<(
    key: string,
  ) => Promise<RecoveryResult>>(async () => "stale");

  const recordUpdateCheck = useCallback(() => {
    const checkedAt = Date.now();
    try {
      window.localStorage.setItem(UPDATE_CHECK_STORAGE_KEY, String(checkedAt));
    } catch {
      // 浏览器拒绝本地存储不改变服务端检查结果。
    }
    setLastUpdateCheckedAt(checkedAt);
  }, []);

  const operationLabel = useCallback((intent: SkillOperationIntent): string => {
    switch (intent.kind) {
      case "check_updates":
        return t("capability.skill_operation_check_updates");
      case "delete":
        return t("capability.skill_operation_delete", { name: intent.skillName });
      case "update":
        return t("capability.skill_operation_update", { name: intent.skillName });
      case "import_local":
        return t("capability.skill_operation_import_local", { name: intent.fileName });
      case "import_git":
        return t("capability.skill_operation_import_git");
      case "import_external":
        return t("capability.skill_operation_import_external", { name: intent.skillName });
    }
  }, [t]);

  const removeRecovery = useCallback((recovery: PendingSkillRecovery) => {
    if (pendingRecoveriesRef.current.get(recovery.key) === recovery) {
      pendingRecoveriesRef.current.delete(recovery.key);
    }
  }, []);

  const reportCommittedRefreshFailure = useCallback((
    recovery: PendingSkillRecovery,
  ) => {
    const operation = operationLabel(recovery.intent);
    feedback.report({
      action: {
        label: t("capability.skill_operation_refresh_action"),
        onClick: () => {
          void reconcileRecoveryRef.current(recovery.key);
        },
      },
      impact: t("capability.skill_operation_committed_impact"),
      message: t("capability.skill_operation_committed_message", { operation }),
      nextStep: t("capability.skill_operation_committed_next_step"),
      pending: false,
      persistent: true,
      title: t("capability.skill_operation_committed_title"),
      tone: "warning",
    });
  }, [feedback, operationLabel, t]);

  const reconcileRecovery = useCallback(async (
    recoveryKey: string,
  ): Promise<RecoveryResult> => {
    const recovery = pendingRecoveriesRef.current.get(recoveryKey);
    if (!recovery) return "stale";
    const operation = operationLabel(recovery.intent);
    feedback.report({
      impact: t("capability.skill_operation_checking_impact"),
      message: t("capability.skill_operation_checking_message", { operation }),
      nextStep: t("capability.skill_operation_checking_next_step"),
      pending: true,
      persistent: true,
      title: t("capability.skill_operation_checking_title"),
      tone: "warning",
    });

    let catalogRefreshed = false;
    try {
      catalogRefreshed = await refreshCatalog();
    } catch {
      // 只读刷新异常仍按刷新失败处理；不能回到 mutation 路径。
    }
    if (pendingRecoveriesRef.current.get(recoveryKey) !== recovery) {
      return "stale";
    }
    if (recovery.effect === "committed") {
      if (catalogRefreshed) {
        removeRecovery(recovery);
        recovery.presentSuccess();
        return "applied";
      }
      reportCommittedRefreshFailure(recovery);
      return "failed";
    }

    const targetName = skillOperationTargetName(recovery.intent);
    const target = targetName
      ? await readSkillTarget(targetName)
      : { readable: true as const, detail: null };
    if (pendingRecoveriesRef.current.get(recoveryKey) !== recovery) {
      return "stale";
    }
    const outcome = target.readable
      ? reconcileSkillOperation(recovery.intent, target.detail)
      : "unproven";
    if (outcome === "applied") {
      recovery.effect = "committed";
      applyCommittedUI(recovery);
      if (catalogRefreshed) {
        removeRecovery(recovery);
        recovery.presentSuccess();
      } else {
        reportCommittedRefreshFailure(recovery);
      }
      return "applied";
    }

    if (!catalogRefreshed || !target.readable) {
      feedback.report({
        action: {
          label: t("capability.skill_operation_recheck_action"),
          onClick: () => {
            void reconcileRecoveryRef.current(recovery.key);
          },
        },
        impact: t("capability.skill_operation_unknown_impact"),
        message: t("capability.skill_operation_reconcile_failed_message", {
          operation,
        }),
        nextStep: t("capability.skill_operation_reconcile_failed_next_step"),
        pending: false,
        persistent: true,
        title: t("capability.skill_operation_unknown_title"),
        tone: "warning",
      });
      return "failed";
    }

    if (recovery.effect === "accepted") {
      feedback.report({
        action: {
          label: t("capability.skill_operation_recheck_action"),
          onClick: () => {
            void reconcileRecoveryRef.current(recovery.key);
          },
        },
        impact: t("capability.skill_operation_accepted_impact"),
        message: t("capability.skill_operation_accepted_message", { operation }),
        nextStep: t("capability.skill_operation_accepted_next_step"),
        pending: false,
        persistent: true,
        title: t("capability.skill_operation_accepted_title"),
        tone: "warning",
      });
      return "unproven";
    }

    feedback.report({
      action: {
        label: t("capability.skill_operation_new_intent_action"),
        onClick: () => {
          removeRecovery(recovery);
          feedback.clear();
        },
      },
      impact: t("capability.skill_operation_unknown_impact"),
      message: t("capability.skill_operation_unknown_message", {
        operation,
        reason: recovery.reason,
      }),
      nextStep: t("capability.skill_operation_unknown_next_step"),
      pending: false,
      persistent: true,
      title: t("capability.skill_operation_unknown_title"),
      tone: "warning",
    });
    return "unproven";
  }, [
    feedback,
    operationLabel,
    refreshCatalog,
    removeRecovery,
    reportCommittedRefreshFailure,
    t,
  ]);

  useEffect(() => {
    reconcileRecoveryRef.current = reconcileRecovery;
  }, [reconcileRecovery]);

  const executeMutation = useCallback(async <T,>({
    failureFallback,
    intent,
    mutate,
    onCommitted,
    onResponse,
    presentCommitted,
    presentSuccess,
  }: ExecuteSkillMutationOptions<T>): Promise<boolean> => {
    const key = skillOperationIntentKey(intent);
    if (pendingRecoveriesRef.current.has(key)) {
      return (await reconcileRecoveryRef.current(key)) === "applied";
    }
    if (activeIntentKeysRef.current.has(key)) return false;
    activeIntentKeysRef.current.add(key);
    try {
      let result: T;
      try {
        result = await mutate();
      } catch (error) {
        const failure = projectMutationFailure(error, failureFallback);
        if (failure.effect === "not_applied") {
          feedback.report({
            impact: t("capability.skill_operation_not_applied_impact"),
            message: failure.message,
            nextStep: t("capability.skill_operation_not_applied_next_step"),
            pending: false,
            title: t("capability.skill_operation_not_applied_title", {
              operation: operationLabel(intent),
            }),
            tone: "error",
          });
          return false;
        }
        const recovery: PendingSkillRecovery = {
          committedUIApplied: false,
          effect: failure.effect,
          intent,
          key,
          onCommitted,
          presentSuccess: presentCommitted,
          reason: failure.message,
        };
        pendingRecoveriesRef.current.set(key, recovery);
        if (failure.effect === "committed") {
          applyCommittedUI(recovery);
        }
        const recoveryResult = await reconcileRecovery(recovery.key);
        return failure.effect === "committed" || recoveryResult === "applied";
      }

      onResponse?.(result);
      const recovery: PendingSkillRecovery = {
        committedUIApplied: false,
        effect: "committed",
        intent,
        key,
        onCommitted,
        presentSuccess: () => presentSuccess(result),
        reason: t("capability.skill_operation_catalog_refresh_failed"),
      };
      pendingRecoveriesRef.current.set(key, recovery);
      applyCommittedUI(recovery);
      await reconcileRecovery(recovery.key);
      return true;
    } finally {
      activeIntentKeysRef.current.delete(key);
    }
  }, [feedback, operationLabel, reconcileRecovery, t]);

  const runUpdateCheck = useCallback(async (manual: boolean) => {
    if (checkingRef.current) return;
    if (manual) feedback.clear();
    checkingRef.current = true;
    setCheckingUpdates(true);
    let responseReceived = false;
    try {
      await executeMutation({
        failureFallback: t("capability.skills_update_check_failed"),
        intent: { kind: "check_updates" },
        mutate: checkSkillUpdatesApi,
        onResponse: (result) => {
          responseReceived = true;
          recordUpdateCheck();
          setCheckUpdateNotice(buildSkillUpdateCheckNotice(
            result.available_skills.length,
            result.failures,
            manual,
          ));
        },
        presentCommitted: () => feedback.success(
          t("capability.skill_operation_check_updates_complete"),
        ),
        presentSuccess: () => {
          // 详细结果由紧邻更新列表的 notice 展示，不再叠加第二条成功反馈。
          feedback.clear();
        },
      });
    } finally {
      if (!manual && !responseReceived) recordUpdateCheck();
      checkingRef.current = false;
      setCheckingUpdates(false);
    }
  }, [executeMutation, feedback, recordUpdateCheck, t]);

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
    const baseline = catalogSkills.find((skill) => skill.name === skillName);
    const intent: SkillOperationIntent = {
      baselineHasUpdate: baseline?.has_update === true,
      kind: "update",
      skillName,
    };
    if (activeIntentKeysRef.current.has(skillOperationIntentKey(intent))) return false;
    feedback.clear();
    setBusyKey(setBusySkillNames, skillName, true);
    try {
      return await executeMutation({
        failureFallback: t("capability.skills_update_failed"),
        intent,
        mutate: () => updateSingleSkillApi(skillName),
        presentCommitted: () => feedback.success(
          t("capability.skills_updated", { name: skillName }),
        ),
        presentSuccess: (detail) => {
          const warning = formatDeployFailureMessage(
            skillName,
            detail.deploy_failures,
            { locale, t },
          );
          if (warning) feedback.warning(warning);
          else feedback.success(t("capability.skills_updated", { name: skillName }));
        },
      });
    } finally {
      setBusyKey(setBusySkillNames, skillName, false);
    }
  }, [catalogSkills, executeMutation, feedback, locale, t]);

  const deleteSkill = useCallback(async (skill: SkillInfo) => {
    const intent: SkillOperationIntent = {
      kind: "delete",
      skillName: skill.name,
    };
    if (activeIntentKeysRef.current.has(skillOperationIntentKey(intent))) return false;
    feedback.clear();
    setBusyKey(setBusySkillNames, skill.name, true);
    const presentSuccess = () => feedback.success(t("capability.skills_removed", {
      name: skill.title || skill.name,
    }));
    try {
      return await executeMutation({
        failureFallback: t("capability.skills_delete_failed"),
        intent,
        mutate: () => deleteSkillApi(skill.name),
        presentCommitted: presentSuccess,
        presentSuccess,
      });
    } finally {
      setBusyKey(setBusySkillNames, skill.name, false);
    }
  }, [executeMutation, feedback, t]);

  const importLocal = useCallback(async (file: File) => {
    if (importingRef.current) return;
    importingRef.current = true;
    setImporting(true);
    feedback.start(t("capability.skills_importing_file", { name: file.name }));
    const presentSuccess = () => feedback.success(
      t("capability.skills_imported_file", { name: file.name }),
    );
    try {
      await executeMutation({
        failureFallback: t("capability.skills_import_failed"),
        intent: {
          fileLastModified: file.lastModified,
          fileName: file.name,
          fileSize: file.size,
          fileType: file.type,
          kind: "import_local",
        },
        mutate: () => importLocalSkillApi(file),
        onCommitted: () => setImportDialogMode(null),
        presentCommitted: presentSuccess,
        presentSuccess,
      });
    } finally {
      importingRef.current = false;
      setImporting(false);
    }
  }, [executeMutation, feedback, t]);

  const importGit = useCallback(async (
    url: string,
    branch?: string,
    path?: string,
  ) => {
    const normalizedUrl = url.trim();
    if (!normalizedUrl || importingRef.current) return;
    const normalizedBranch = branch?.trim() || "";
    const normalizedPath = path?.trim() || "";
    importingRef.current = true;
    setImporting(true);
    feedback.start(t("capability.skills_git_importing"));
    const presentSuccess = () => feedback.success(t("capability.skills_git_imported"));
    try {
      await executeMutation({
        failureFallback: t("capability.skills_git_import_failed"),
        intent: {
          branch: normalizedBranch,
          kind: "import_git",
          path: normalizedPath,
          url: normalizedUrl,
        },
        mutate: () => importGitSkillApi(
          normalizedUrl,
          normalizedBranch || undefined,
          normalizedPath || undefined,
        ),
        onCommitted: () => setImportDialogMode(null),
        presentCommitted: presentSuccess,
        presentSuccess,
      });
    } finally {
      importingRef.current = false;
      setImporting(false);
    }
  }, [executeMutation, feedback, t]);

  const importExternal = useCallback(async (item: ExternalSkillSearchItem) => {
    const key = externalSkillKey(item);
    const intent: SkillOperationIntent = {
      kind: "import_external",
      skillName: item.skill_slug,
      sourceKey: item.source_key || item.package_spec,
      sourceRef: item.package_spec,
    };
    if (activeIntentKeysRef.current.has(skillOperationIntentKey(intent))) return;
    setBusyKey(setBusyExternalKeys, key, true);
    feedback.start(t("capability.skills_importing_file", {
      name: item.skill_slug,
    }));
    const presentSuccess = () => feedback.success(
      t("capability.skills_imported_file", { name: item.skill_slug }),
    );
    try {
      await executeMutation({
        failureFallback: t("capability.skills_external_import_failed"),
        intent,
        mutate: () => importExternalSkillApi(item),
        onCommitted: closeExternalPreview,
        presentCommitted: presentSuccess,
        presentSuccess,
      });
    } finally {
      setBusyKey(setBusyExternalKeys, key, false);
    }
  }, [closeExternalPreview, executeMutation, feedback, t]);

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
