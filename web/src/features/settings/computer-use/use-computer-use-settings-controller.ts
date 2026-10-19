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
import { openDesktopExternalURL } from "@/lib/desktop-bridge/desktop-bridge";
import type { CUAInstallationStatus } from "@/types/settings/preferences";

export function useComputerUseSettingsController() {
  const [status, setStatus] = useState<CUAInstallationStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(false);
  const requestInFlight = useRef(false);

  const refresh = useCallback(async () => {
    try {
      setStatus(await getComputerUseStatusApi());
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
      if (enabled && !next?.installed) {
        next = await installComputerUseApi();
      }
      next = await setComputerUseEnabledApi(enabled);
      if (enabled) {
        try {
          next = await checkComputerUsePermissionsApi();
        } catch {
          setError(true);
        }
      }
      setStatus(next);
    } catch {
      setError(true);
      await refresh();
    } finally {
      requestInFlight.current = false;
      setBusy(false);
    }
  }, [refresh, status]);

  const onCheckPermissions = useCallback(async () => {
    if (requestInFlight.current) return;
    requestInFlight.current = true;
    setBusy(true);
    setError(false);
    try {
      setStatus(await checkComputerUsePermissionsApi());
    } catch {
      setError(true);
      await refresh();
    } finally {
      requestInFlight.current = false;
      setBusy(false);
    }
  }, [refresh]);

  const onOpenPermissions = useCallback(async () => {
    try {
      await openDesktopExternalURL(
        "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture",
      );
    } catch {
      setError(true);
    }
  }, []);

  return {
    busy,
    error,
    onChange,
    onCheckPermissions,
    onOpenPermissions,
    refresh,
    status,
  };
}
