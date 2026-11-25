// INPUT: 电脑操控安装状态、运行时权限检测与用户主动发起的授权操作。
// OUTPUT: 设置开关、可恢复的 macOS 权限向导和按真实检测结果推进的步骤。
// POS: 设置域管理引导生命周期，系统授权由桌面宿主与用户完成。
import { useCallback, useEffect, useRef, useState } from "react";

import { getDesktopPlatform } from "@/config/desktop-runtime";
import {
  checkComputerUsePermissionsApi,
  getComputerUseStatusApi,
  installComputerUseApi,
  setComputerUseEnabledApi,
} from "@/lib/api/settings/runtime-api";
import { startDesktopComputerUsePermissionSetup, updateDesktopComputerUsePermissionSetup } from "@/lib/desktop-bridge/desktop-bridge";
import type { Locale } from "@/shared/i18n/messages";
import type { CUAInstallationStatus } from "@/types/settings/preferences";

export const PERMISSION_ORDER = ["accessibility", "screen_capture", "input_control"] as const;

export function nextMissingPermission(status: CUAInstallationStatus | null) {
  if (!status?.enabled || status.update_required) return;
  return PERMISSION_ORDER.find((key) => {
    const state = status.permissions?.[key];
    return state !== "granted" && state !== "not_applicable";
  });
}

export function useComputerUseSettingsController(locale: Locale) {
  const [status, setStatus] = useState<CUAInstallationStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<"component" | "check" | "setup" | null>(null);
  const guideOpen = useRef(false);
  const mounted = useRef(false);
  const requestInFlight = useRef(false);

  const dismissGuide = useCallback(() => {
    if (!guideOpen.current) return;
    guideOpen.current = false;
    void updateDesktopComputerUsePermissionSetup({ dismiss: true }).catch(() => {});
  }, []);

  const openGuide = useCallback(async (next: CUAInstallationStatus | null) => {
    if (!mounted.current || getDesktopPlatform() !== "macos" || !nextMissingPermission(next)) return;
    guideOpen.current = true;
    try {
      await startDesktopComputerUsePermissionSetup(locale, next?.permissions);
    } catch {
      guideOpen.current = false;
      setError("setup");
    }
  }, [locale]);

  const checkPermissions = useCallback(async () => {
    try {
      const next = await checkComputerUsePermissionsApi();
      setStatus(next);
      setError(null);
      if (guideOpen.current) {
        if (!next.enabled || next.update_required) dismissGuide();
        else await updateDesktopComputerUsePermissionSetup({ permissions: next.permissions ?? {} }).catch(() => {
          setError("setup");
        });
      }
      return next;
    } catch {
      setError("check");
      if (guideOpen.current) await updateDesktopComputerUsePermissionSetup({ check_failed: true }).catch(() => {});
    }
  }, [dismissGuide]);

  const refresh = useCallback(async () => {
    try {
      const next = await getComputerUseStatusApi();
      setStatus(next);
      setError(null);
      if (next.enabled && !next.update_required) await checkPermissions();
    } catch {
      setError("component");
    }
  }, [checkPermissions]);

  // 初次读取、开关变更和返回时的复查共用一条请求路径，避免旧结果覆盖新的开关状态。
  const runRequest = useCallback(async (action: () => Promise<unknown>) => {
    if (requestInFlight.current) return;
    requestInFlight.current = true;
    setBusy(true);
    setError(null);
    try {
      await action();
    } finally {
      requestInFlight.current = false;
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    mounted.current = true;
    void runRequest(refresh);
    return () => {
      mounted.current = false;
      dismissGuide();
    };
  }, [dismissGuide, refresh, runRequest]);

  const onChange = (enabled: boolean) => runRequest(async () => {
    dismissGuide();
    try {
      if (enabled && (!status?.installed || status.update_required)) {
        await installComputerUseApi();
      }
      const next = await setComputerUseEnabledApi(enabled);
      setStatus(next);
      if (next.enabled) {
        const checked = await checkPermissions();
        // 只有用户主动启用时才弹出向导。重新进入设置时保留继续入口。
        await openGuide(checked ?? next);
      }
    } catch {
      await refresh();
      setError("component");
    }
  });

  const onUpdate = () => runRequest(async () => {
    try {
      const next = await installComputerUseApi();
      setStatus(next);
      if (next.enabled) await checkPermissions();
    } catch {
      await refresh();
      setError("component");
    }
  });

  const onCheckPermissions = useCallback(() => runRequest(checkPermissions), [checkPermissions, runRequest]);

  useEffect(() => {
    // 设置 App 返回的是原生向导窗口，WKWebView 不一定获得 focus。
    const recheck = () => { if (guideOpen.current) void onCheckPermissions(); };
    const closed = () => {
      if (!guideOpen.current) return;
      guideOpen.current = false;
      void onCheckPermissions();
    };
    window.addEventListener("focus", recheck);
    window.addEventListener("nexus:computer-use-check", recheck);
    window.addEventListener("nexus:computer-use-guide-closed", closed);
    return () => {
      window.removeEventListener("focus", recheck);
      window.removeEventListener("nexus:computer-use-check", recheck);
      window.removeEventListener("nexus:computer-use-guide-closed", closed);
    };
  }, [onCheckPermissions]);

  return {
    busy,
    error,
    onChange,
    onCheckPermissions,
    onOpenPermissions: () => openGuide(status),
    onUpdate,
    status,
  };
}
