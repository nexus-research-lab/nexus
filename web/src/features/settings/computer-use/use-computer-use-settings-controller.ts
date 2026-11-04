/**
 * INPUT: Host Computer Use installation status and explicit enable/permission actions.
 * OUTPUT: CUA settings state and user-facing actions for the dedicated settings page.
 * POS: Computer Use settings controller; it does not participate in runtime preferences.
 */
import { useCallback, useEffect, useRef, useState } from "react";

import {
  checkComputerUsePermissionsApi,
  getComputerUseStatusApi,
  installComputerUseApi,
  setComputerUseEnabledApi,
} from "@/lib/api/settings/runtime-api";
import {
  startDesktopComputerUsePermissionSetup,
  type DesktopComputerUsePermission,
} from "@/lib/desktop-bridge/desktop-bridge";
import type {
  CUAInstallationPermissionKey,
  CUAInstallationStatus,
} from "@/types/settings/preferences";

const PERMISSION_ORDER: readonly CUAInstallationPermissionKey[] = [
  "screen_capture",
  "accessibility",
  "input_control",
];

function nextMissingPermission(status: CUAInstallationStatus | null): CUAInstallationPermissionKey | null {
  if (!status?.enabled) return null;
  const permissions = status.permissions;
  if (!permissions) return "screen_capture";
  return PERMISSION_ORDER.find((permission) => {
    const state = permissions[permission];
    return state !== "granted" && state !== "not_applicable";
  }) ?? null;
}

export function useComputerUseSettingsController() {
  const [status, setStatus] = useState<CUAInstallationStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(false);
  const requestInFlight = useRef(false);
  const permissionSetupPending = useRef(false);

  const refresh = useCallback(async () => {
    try {
      const next = await getComputerUseStatusApi();
      if (next.enabled && !next.update_required) {
        try {
          setStatus(await checkComputerUsePermissionsApi());
        } catch {
          setStatus(next);
          setError(true);
          return;
        }
      } else {
        setStatus(next);
      }
      setError(false);
    } catch {
      setError(true);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const onChange = useCallback(async (enabled: boolean) => {
    if (requestInFlight.current) return;
    requestInFlight.current = true;
    setBusy(true);
    setError(false);
    try {
      let next = status;
      if (enabled && (!next?.installed || next.update_required)) {
        next = await installComputerUseApi();
      }
      next = await setComputerUseEnabledApi(enabled);
      if (enabled) {
        try {
          next = await checkComputerUsePermissionsApi();
        } catch {
          permissionSetupPending.current = false;
          setError(true);
        }
      }
      setStatus(next);
    } catch {
      await refresh();
      setError(true);
    } finally {
      requestInFlight.current = false;
      setBusy(false);
    }
  }, [refresh, status]);

  const onUpdate = useCallback(async () => {
    if (requestInFlight.current) return;
    requestInFlight.current = true;
    setBusy(true);
    setError(false);
    try {
      const next = await installComputerUseApi();
      setStatus(next);
      if (next.enabled) {
        setStatus(await checkComputerUsePermissionsApi());
      }
    } catch {
      await refresh();
      setError(true);
    } finally {
      requestInFlight.current = false;
      setBusy(false);
    }
  }, [refresh]);

  const onCheckPermissions = useCallback(async () => {
    if (requestInFlight.current) return;
    requestInFlight.current = true;
    setBusy(true);
    setError(false);
    try {
      setStatus(await checkComputerUsePermissionsApi());
    } catch {
      await refresh();
      setError(true);
    } finally {
      requestInFlight.current = false;
      setBusy(false);
    }
  }, [refresh]);

  const onOpenPermissions = useCallback(async (permission?: DesktopComputerUsePermission) => {
    const nextPermission = permission ?? nextMissingPermission(status);
    if (!nextPermission) return;
    permissionSetupPending.current = true;
    try {
      await startDesktopComputerUsePermissionSetup(nextPermission);
    } catch {
      permissionSetupPending.current = false;
      setError(true);
    }
  }, [status]);

  const onPermissionPageReturn = useCallback(() => {
    if (
      !permissionSetupPending.current
      || requestInFlight.current
      || status?.enabled !== true
      || document.visibilityState === "hidden"
    ) return;
    permissionSetupPending.current = false;
    void onCheckPermissions();
  }, [onCheckPermissions, status?.enabled]);

  useEffect(() => {
    window.addEventListener("focus", onPermissionPageReturn);
    document.addEventListener("visibilitychange", onPermissionPageReturn);
    return () => {
      window.removeEventListener("focus", onPermissionPageReturn);
      document.removeEventListener("visibilitychange", onPermissionPageReturn);
    };
  }, [onPermissionPageReturn]);

  return {
    busy,
    error,
    onChange,
    onCheckPermissions,
    onOpenPermissions,
    onUpdate,
    refresh,
    status,
  };
}
