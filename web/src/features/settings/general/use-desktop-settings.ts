import { useCallback, useEffect, useRef, useState } from "react";

import {
  exportDesktopLogs,
  getDesktopAppVersion,
  isDesktopBridgeAvailable,
  type DesktopAppVersion,
} from "@/lib/desktop-bridge";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";

function describeDesktopVersion(
  version: DesktopAppVersion | null,
  fallbackMessage: string,
  valueMessage: string,
): string {
  if (!version) {
    return fallbackMessage;
  }
  return valueMessage
    .replace("{version}", version.app_version)
    .replace("{build}", version.build_number);
}

export function useDesktopSettings() {
  const { t } = useI18n();
  const [available] = useState(() => isDesktopBridgeAvailable());
  const [version, setVersion] = useState<DesktopAppVersion | null>(null);
  const [versionLoading, setVersionLoading] = useState(available);
  const [feedbackMessage, setFeedbackMessage] = useState("");
  const [exportingLogs, setExportingLogs] = useState(false);
  const exportingRef = useRef(false);

  useEffect(() => {
    if (!available) {
      return;
    }
    let cancelled = false;
    void getDesktopAppVersion()
      .then((result) => {
        if (!cancelled) {
          setVersion(result);
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setFeedbackMessage(getErrorMessage(
            error,
            t("settings.desktop.version_failed"),
          ));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setVersionLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [available, t]);

  const exportLogs = useCallback(async () => {
    if (exportingRef.current) {
      return;
    }
    exportingRef.current = true;
    setExportingLogs(true);
    setFeedbackMessage("");
    try {
      const result = await exportDesktopLogs();
      if (!result.cancelled) {
        setFeedbackMessage(result.path
          ? t("settings.desktop.export_logs_success_with_path").replace(
            "{path}",
            result.path,
          )
          : t("settings.desktop.export_logs_success"));
      }
    } catch (error) {
      setFeedbackMessage(getErrorMessage(
        error,
        t("settings.desktop.export_logs_failed"),
      ));
    } finally {
      exportingRef.current = false;
      setExportingLogs(false);
    }
  }, [t]);

  return {
    available,
    exportLogs,
    exportingLogs,
    feedbackMessage,
    versionDescription: describeDesktopVersion(
      version,
      t(versionLoading
        ? "settings.desktop.version_loading"
        : "settings.desktop.version_failed"),
      t("settings.desktop.version_value"),
    ),
  };
}
