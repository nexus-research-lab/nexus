/**
 * INPUT: 桌面沙箱偏好、nxs 沙箱诊断、owner-scoped 残留资源与启动恢复受阻会话 API。
 * OUTPUT: 沙箱分区的开关、显式诊断、预览后回收动作与确认后的受阻记录放弃。
 * POS: 沙箱设置控制器；诊断与回收不修改 Preferences，开关复用版本化偏好事务。
 */
import { useCallback, useRef, useState } from "react";

import {
  discardSandboxRecoveryApi,
  getNxsRuntimeStatusApi,
  inspectSandboxResourcesApi,
  reconcileSandboxResourcesApi,
} from "@/lib/api/settings/runtime-api";
import type {
  NXSSandboxDiagnosticState,
  SandboxResourceInspection,
  SandboxResourceRecord,
} from "@/types/settings/preferences";

import { useUserPreferences } from "../general/use-user-preferences";

const SANDBOX_RECOVERY_AGE_SECONDS = 60 * 60;

export interface SandboxRecoverySummary {
  blockedCount: number;
  candidateCount: number;
  discardableLaunchIds: string[];
  discardedCount: number;
  removedCount: number;
  resourceCount: number;
  unknownCount: number;
}

export function useSandboxSettingsController() {
  const {
    feedback,
    loading,
    preferences,
    recovery,
    saving,
    updatePreferences,
    writable,
  } = useUserPreferences();
  const onDesktopSandboxEnabledChange = useCallback((checked: boolean) => {
    updatePreferences((current) => ({
      ...current,
      desktop_sandbox_enabled: checked,
    }));
  }, [updatePreferences]);
  const [sandboxChecking, setSandboxChecking] = useState(false);
  const [sandboxState, setSandboxState] = useState<NXSSandboxDiagnosticState | null>(null);
  const sandboxRequest = useRef(false);
  const onCheckSandbox = useCallback(async () => {
    if (sandboxRequest.current) return;
    sandboxRequest.current = true;
    setSandboxChecking(true);
    setSandboxState(null);
    try {
      const status = await getNxsRuntimeStatusApi(true);
      setSandboxState(status.sandbox?.state ?? "unknown");
    } catch {
      setSandboxState("unknown");
    } finally {
      sandboxRequest.current = false;
      setSandboxChecking(false);
    }
  }, []);
  const [sandboxRecoveryChecking, setSandboxRecoveryChecking] = useState(false);
  const [sandboxRecoveryApplying, setSandboxRecoveryApplying] = useState(false);
  const [sandboxRecoveryDiscarding, setSandboxRecoveryDiscarding] = useState(false);
  const [sandboxRecoverySummary, setSandboxRecoverySummary] = useState<SandboxRecoverySummary | null>(null);
  const [sandboxRecoveryError, setSandboxRecoveryError] = useState(false);
  const sandboxRecoveryRequest = useRef(false);
  const projectSandboxRecoverySummary = useCallback((
    inspection: SandboxResourceInspection,
    candidateCount: number,
    removedCount = 0,
    discardedCount = 0,
  ): SandboxRecoverySummary => {
    const resources: SandboxResourceRecord[] = inspection.resources;
    const blocked = inspection.blocked_sessions ?? [];
    return {
      blockedCount: blocked.length,
      candidateCount,
      discardableLaunchIds: blocked.filter((record) => record.discardable).map((record) => record.launch_id),
      discardedCount,
      removedCount,
      resourceCount: resources.length,
      unknownCount: resources.filter((resource) => resource.marker.cleanup_state === "cleanup_unknown").length,
    };
  }, []);
  const refreshSandboxRecoverySummary = useCallback(async (removedCount = 0, discardedCount = 0) => {
    const inspection = await inspectSandboxResourcesApi();
    const preview = await reconcileSandboxResourcesApi({
      older_than_seconds: SANDBOX_RECOVERY_AGE_SECONDS,
      apply: false,
    });
    setSandboxRecoverySummary(projectSandboxRecoverySummary(
      inspection,
      preview.candidates.length,
      removedCount,
      discardedCount,
    ));
  }, [projectSandboxRecoverySummary]);
  const onInspectSandboxResources = useCallback(async () => {
    if (sandboxRecoveryRequest.current) return;
    sandboxRecoveryRequest.current = true;
    setSandboxRecoveryChecking(true);
    setSandboxRecoveryError(false);
    try {
      await refreshSandboxRecoverySummary();
    } catch {
      setSandboxRecoveryError(true);
    } finally {
      sandboxRecoveryRequest.current = false;
      setSandboxRecoveryChecking(false);
    }
  }, [refreshSandboxRecoverySummary]);
  const onReconcileSandboxResources = useCallback(async () => {
    if (sandboxRecoveryRequest.current) return;
    sandboxRecoveryRequest.current = true;
    setSandboxRecoveryApplying(true);
    setSandboxRecoveryError(false);
    try {
      const result = await reconcileSandboxResourcesApi({
        older_than_seconds: SANDBOX_RECOVERY_AGE_SECONDS,
        apply: true,
      });
      await refreshSandboxRecoverySummary(result.removed.length);
    } catch {
      setSandboxRecoveryError(true);
    } finally {
      sandboxRecoveryRequest.current = false;
      setSandboxRecoveryApplying(false);
    }
  }, [refreshSandboxRecoverySummary]);
  // 只放弃最近一次检查中服务端标记为可放弃的记录；逐条提交，失败即停并重新检查。
  const onDiscardBlockedSandboxRecords = useCallback(async () => {
    const launchIds = sandboxRecoverySummary?.discardableLaunchIds ?? [];
    if (sandboxRecoveryRequest.current || launchIds.length === 0) return;
    sandboxRecoveryRequest.current = true;
    setSandboxRecoveryDiscarding(true);
    setSandboxRecoveryError(false);
    let discarded = 0;
    try {
      for (const launchId of launchIds) {
        await discardSandboxRecoveryApi(launchId);
        discarded += 1;
      }
    } catch {
      setSandboxRecoveryError(true);
    }
    try {
      await refreshSandboxRecoverySummary(0, discarded);
    } catch {
      setSandboxRecoveryError(true);
    } finally {
      sandboxRecoveryRequest.current = false;
      setSandboxRecoveryDiscarding(false);
    }
  }, [refreshSandboxRecoverySummary, sandboxRecoverySummary]);
  return {
    desktopSandboxEnabled: preferences.desktop_sandbox_enabled === true,
    onDesktopSandboxEnabledChange,
    sandboxChecking, sandboxState, onCheckSandbox,
    sandboxRecoveryApplying,
    sandboxRecoveryChecking,
    sandboxRecoveryDiscarding,
    sandboxRecoveryError,
    sandboxRecoverySummary,
    onInspectSandboxResources,
    onReconcileSandboxResources,
    onDiscardBlockedSandboxRecords,
    preferencesFeedback: feedback,
    preferencesRecovery: recovery,
    loading,
    preferencesBusy: saving || !writable,
  };
}
