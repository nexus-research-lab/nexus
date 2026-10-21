/**
 * INPUT: 桌面沙箱偏好、nxs 沙箱诊断与 owner-scoped 残留资源 API。
 * OUTPUT: 沙箱分区的开关、显式诊断和预览后回收动作。
 * POS: 沙箱设置控制器；诊断与回收不修改 Preferences，开关复用版本化偏好事务。
 */
import { useCallback, useRef, useState } from "react";

import {
  getNxsRuntimeStatusApi,
  inspectSandboxResourcesApi,
  reconcileSandboxResourcesApi,
} from "@/lib/api/settings/runtime-api";
import type {
  NXSSandboxDiagnosticState,
  SandboxResourceRecord,
} from "@/types/settings/preferences";

import { useUserPreferences } from "../general/use-user-preferences";

const SANDBOX_RECOVERY_AGE_SECONDS = 60 * 60;

export interface SandboxRecoverySummary {
  candidateCount: number;
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
  const [sandboxRecoverySummary, setSandboxRecoverySummary] = useState<SandboxRecoverySummary | null>(null);
  const [sandboxRecoveryError, setSandboxRecoveryError] = useState(false);
  const sandboxRecoveryRequest = useRef(false);
  const projectSandboxRecoverySummary = useCallback((
    resourceCount: number,
    resources: SandboxResourceRecord[],
    candidateCount: number,
    removedCount = 0,
  ): SandboxRecoverySummary => ({
    candidateCount,
    removedCount,
    resourceCount,
    unknownCount: resources.filter((resource) => resource.marker.cleanup_state === "cleanup_unknown").length,
  }), []);
  const onInspectSandboxResources = useCallback(async () => {
    if (sandboxRecoveryRequest.current) return;
    sandboxRecoveryRequest.current = true;
    setSandboxRecoveryChecking(true);
    setSandboxRecoveryError(false);
    try {
      const inspection = await inspectSandboxResourcesApi();
      const preview = await reconcileSandboxResourcesApi({
        older_than_seconds: SANDBOX_RECOVERY_AGE_SECONDS,
        apply: false,
      });
      setSandboxRecoverySummary(projectSandboxRecoverySummary(
        inspection.resources.length,
        inspection.resources,
        preview.candidates.length,
      ));
    } catch {
      setSandboxRecoveryError(true);
    } finally {
      sandboxRecoveryRequest.current = false;
      setSandboxRecoveryChecking(false);
    }
  }, [projectSandboxRecoverySummary]);
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
      const inspection = await inspectSandboxResourcesApi();
      const preview = await reconcileSandboxResourcesApi({
        older_than_seconds: SANDBOX_RECOVERY_AGE_SECONDS,
        apply: false,
      });
      setSandboxRecoverySummary(projectSandboxRecoverySummary(
        inspection.resources.length,
        inspection.resources,
        preview.candidates.length,
        result.removed.length,
      ));
    } catch {
      setSandboxRecoveryError(true);
    } finally {
      sandboxRecoveryRequest.current = false;
      setSandboxRecoveryApplying(false);
    }
  }, [projectSandboxRecoverySummary]);
  return {
    desktopSandboxEnabled: preferences.desktop_sandbox_enabled === true,
    onDesktopSandboxEnabledChange,
    sandboxChecking, sandboxState, onCheckSandbox,
    sandboxRecoveryApplying,
    sandboxRecoveryChecking,
    sandboxRecoveryError,
    sandboxRecoverySummary,
    onInspectSandboxResources,
    onReconcileSandboxResources,
    preferencesFeedback: feedback,
    preferencesRecovery: recovery,
    loading,
    preferencesBusy: saving || !writable,
  };
}
