/**
 * INPUT: 沙箱设置控制器提供的桌面沙箱偏好、诊断状态和残留资源摘要。
 * OUTPUT: 桌面沙箱开关、显式“检查沙箱支持”与预览后回收的残留资源行。
 * POS: 设置目录的沙箱分区，仅桌面端可见；不直接调用 API。
 */
"use client";

import { Box, RotateCcw, ShieldCheck } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";

import { PreferencesReliabilityNotice } from "../general/components/preferences-reliability-notice";
import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTENT_BODY_CLASS_NAME,
  SETTINGS_DIVIDER_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
  SettingsToggleRow,
} from "../shared/settings-panel-ui";
import { useSandboxSettingsController } from "./use-sandbox-settings-controller";

export function SettingsSandboxSection() {
  const { t } = useI18n();
  const settings = useSandboxSettingsController();

  return (
    <div className={cn(WORKSPACE_CONTENT_PAGE_CLASS_NAME, "flex flex-col")}>
      <WorkspaceContentHeader
        className="max-sm:hidden"
        title={t("settings.sandbox.section_title")}
      />
      <section className={cn(SETTINGS_CONTENT_BODY_CLASS_NAME, "space-y-2.5")}>
        <PreferencesReliabilityNotice
          feedback={settings.preferencesFeedback}
          recovery={settings.preferencesRecovery}
        />
        <div className={SETTINGS_CARD_CLASS_NAME}>
          <SettingsToggleRow
            checked={settings.desktopSandboxEnabled}
            description={t("settings.general.desktop_sandbox_description")}
            disabled={settings.loading || settings.preferencesBusy}
            icon={<Box className="h-3.5 w-3.5" />}
            onChange={settings.onDesktopSandboxEnabledChange}
            title={t("settings.general.desktop_sandbox_title")}
          />
          <div className={SETTINGS_DIVIDER_CLASS_NAME} />
          <div className={SETTINGS_ROW_CLASS_NAME}>
            <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
              <div className={SETTINGS_ICON_CLASS_NAME}>
                <ShieldCheck className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                  {t("settings.runtime.sandbox_check")}
                </h3>
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                  {t("settings.runtime.sandbox_check_description")}
                </p>
                {settings.sandboxState ? (
                  <p role="status" className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                    {t(`settings.runtime.sandbox_${settings.sandboxState}`)}
                  </p>
                ) : null}
              </div>
            </div>
            <UiButton variant="text" disabled={settings.sandboxChecking} onClick={() => void settings.onCheckSandbox()}>
              {t(settings.sandboxChecking ? "settings.runtime.sandbox_checking" : "settings.runtime.sandbox_check")}
            </UiButton>
          </div>
          <div className={SETTINGS_DIVIDER_CLASS_NAME} />
          <SandboxRecoveryRow
            applying={settings.sandboxRecoveryApplying}
            checking={settings.sandboxRecoveryChecking}
            disabled={settings.loading}
            error={settings.sandboxRecoveryError}
            onInspect={() => void settings.onInspectSandboxResources()}
            onReconcile={() => void settings.onReconcileSandboxResources()}
            summary={settings.sandboxRecoverySummary}
          />
        </div>
      </section>
    </div>
  );
}

function SandboxRecoveryRow({
  applying,
  checking,
  disabled,
  error,
  onInspect,
  onReconcile,
  summary,
}: {
  applying: boolean;
  checking: boolean;
  disabled: boolean;
  error: boolean;
  onInspect: () => void;
  onReconcile: () => void;
  summary: {
    candidateCount: number;
    removedCount: number;
    resourceCount: number;
    unknownCount: number;
  } | null;
}) {
  const { t } = useI18n();
  const busy = checking || applying;
  return (
    <div className={SETTINGS_ROW_CLASS_NAME}>
      <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
        <div className={SETTINGS_ICON_CLASS_NAME}>
          <RotateCcw className="h-3.5 w-3.5" />
        </div>
        <div className="min-w-0">
          <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
            {t("settings.runtime.sandbox_recovery_title")}
          </h3>
          <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
            {t("settings.runtime.sandbox_recovery_description")}
          </p>
          {summary ? (
            <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME} role="status">
              {t("settings.runtime.sandbox_recovery_resources", { count: summary.resourceCount })}
              {" · "}
              {t("settings.runtime.sandbox_recovery_unknown", { count: summary.unknownCount })}
              {summary.candidateCount > 0 ? (
                <>
                  {" · "}
                  {t("settings.runtime.sandbox_recovery_candidates", { count: summary.candidateCount })}
                </>
              ) : null}
              {summary.removedCount > 0 ? (
                <>
                  {" · "}
                  {t("settings.runtime.sandbox_recovery_removed", { count: summary.removedCount })}
                </>
              ) : null}
            </p>
          ) : null}
          {error ? (
            <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME} role="alert">
              {t("settings.runtime.sandbox_recovery_failed")}
            </p>
          ) : null}
        </div>
      </div>
      <div className="flex flex-wrap items-center justify-end gap-2">
        <UiButton
          disabled={disabled || busy}
          onClick={onInspect}
          variant="text"
        >
          {t(checking
            ? "settings.runtime.sandbox_recovery_checking"
            : "settings.runtime.sandbox_recovery_check")}
        </UiButton>
        {summary && summary.candidateCount > 0 ? (
          <UiButton
            disabled={disabled || busy}
            onClick={onReconcile}
            tone="danger"
            variant="text"
          >
            {t(applying
              ? "settings.runtime.sandbox_recovery_applying"
              : "settings.runtime.sandbox_recovery_apply")}
          </UiButton>
        ) : null}
      </div>
    </div>
  );
}
