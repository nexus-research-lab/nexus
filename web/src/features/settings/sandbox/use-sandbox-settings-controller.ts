/**
 * INPUT: 桌面沙箱偏好与 nxs 沙箱诊断 API。
 * OUTPUT: 沙箱分区的开关与显式诊断。
 * POS: 沙箱设置控制器；诊断不修改 Preferences，开关复用版本化偏好事务。
 */
import { useCallback, useRef, useState } from "react";

import { getNxsRuntimeStatusApi } from "@/lib/api/settings/runtime-api";
import type { NXSSandboxDiagnosticState } from "@/types/settings/preferences";

import { useUserPreferences } from "../general/use-user-preferences";

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
  return {
    desktopSandboxEnabled: preferences.desktop_sandbox_enabled === true,
    onDesktopSandboxEnabledChange,
    sandboxChecking, sandboxState, onCheckSandbox,
    preferencesFeedback: feedback,
    preferencesRecovery: recovery,
    loading,
    preferencesBusy: saving || !writable,
  };
}
