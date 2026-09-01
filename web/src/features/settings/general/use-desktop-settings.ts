/**
 * INPUT: 桌面桥接提供的版本读取与日志导出结果。
 * OUTPUT: 分离版本资源状态和日志导出反馈；失败文案不透传原生异常。
 * POS: 桌面设置控制器；只编排现有 bridge 调用，不改变命令、请求或文件行为。
 */
import { useCallback, useEffect, useRef, useState } from "react";

import {
  exportDesktopLogs,
  getDesktopAppVersion,
  isDesktopBridgeAvailable,
  type DesktopAppVersion,
} from "@/lib/desktop-bridge";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";

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
  const [versionFailed, setVersionFailed] = useState(false);
  const [exportFeedback, setExportFeedback] =
    useState<FeedbackBannerProps | null>(null);
  const [exportingLogs, setExportingLogs] = useState(false);
  const exportingRef = useRef(false);
  const versionLoadingRef = useRef(false);
  const versionRequestRef = useRef(0);

  const loadVersion = useCallback(() => {
    if (!available || versionLoadingRef.current) {
      return;
    }
    versionLoadingRef.current = true;
    const requestId = versionRequestRef.current + 1;
    versionRequestRef.current = requestId;
    setVersionLoading(true);
    setVersionFailed(false);
    void getDesktopAppVersion()
      .then((result) => {
        if (versionRequestRef.current === requestId) {
          setVersion(result);
        }
      })
      .catch(() => {
        if (versionRequestRef.current === requestId) {
          setVersionFailed(true);
        }
      })
      .finally(() => {
        if (versionRequestRef.current === requestId) {
          versionLoadingRef.current = false;
          setVersionLoading(false);
        }
      });
  }, [available]);

  useEffect(() => {
    loadVersion();
    return () => {
      versionRequestRef.current += 1;
      versionLoadingRef.current = false;
    };
  }, [loadVersion]);

  const exportLogs = useCallback(async () => {
    if (exportingRef.current) {
      return;
    }
    exportingRef.current = true;
    setExportingLogs(true);
    setExportFeedback(null);
    try {
      const result = await exportDesktopLogs();
      if (!result.cancelled) {
        setExportFeedback({
          message: result.path
            ? t("settings.desktop.export_logs_success_with_path").replace(
              "{path}",
              result.path,
            )
            : t("settings.desktop.export_logs_success"),
          onDismiss: () => setExportFeedback(null),
          title: t("settings.desktop.export_logs_success"),
          tone: "success",
        });
      }
    } catch {
      setExportFeedback({
        impact: t("settings.desktop.export_logs_failed_impact"),
        nextStep: t("settings.desktop.export_logs_failed_next_step"),
        title: t("settings.desktop.export_logs_failed_title"),
        tone: "warning",
      });
    } finally {
      exportingRef.current = false;
      setExportingLogs(false);
    }
  }, [t]);

  return {
    available,
    exportLogs,
    feedback: exportFeedback,
    exportingLogs,
    retryVersion: loadVersion,
    versionFailed,
    versionLoading,
    versionDescription: describeDesktopVersion(
      version,
      t(versionLoading
        ? "settings.desktop.version_loading"
        : "settings.desktop.version_failed"),
      t("settings.desktop.version_value"),
    ),
  };
}
