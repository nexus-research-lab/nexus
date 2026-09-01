// INPUT: Composer 资源读取异常或 exact Session-setting 修改异常。
// OUTPUT: 本地化三问状态、保守 mutation effect 与同意图锁身份。
// POS: Composer 设置可靠性的纯模型；不读取业务状态，也不执行重试。
import type { SessionRuntimeSettings } from "@/lib/api/conversation/session-api";
import {
  projectMutationFailure,
  type MutationFailureEffect,
} from "@/lib/error-message";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";

export type ComposerReadResource =
  | "connectors"
  | "models"
  | "providers"
  | "session_settings"
  | "skills";

export interface ComposerReadFailure {
  impact: string;
  resource: ComposerReadResource;
  title: string;
}

export type ComposerSettingKind = "connectors" | "model" | "permission";

export interface ComposerSettingsMutationIntent {
  fingerprint: string;
  sessionKey: string;
  setting: ComposerSettingKind;
}

export interface ComposerSettingsMutationFailure {
  blocksRepeat: boolean;
  effect: MutationFailureEffect;
  impact: string;
  intent: ComposerSettingsMutationIntent;
  title: string;
}

export function buildComposerReadFailure(
  _error: unknown,
  resource: ComposerReadResource,
  fallback: string,
  t: I18nContextValue["t"],
): ComposerReadFailure {
  return {
    impact: resource === "session_settings"
      ? t("composer.session_settings_read_impact")
      : t("composer.selection_read_impact"),
    resource,
    title: fallback,
  };
}

export function createComposerSettingsMutationIntent(
  sessionKey: string,
  setting: ComposerSettingKind,
  settings: SessionRuntimeSettings,
): ComposerSettingsMutationIntent {
  return {
    fingerprint: JSON.stringify({
      connector_ids: settings.connector_ids
        ? [...settings.connector_ids].sort()
        : null,
      model: settings.model,
      permission_mode: settings.permission_mode,
      provider: settings.provider,
    }),
    sessionKey,
    setting,
  };
}

export function isSameComposerSettingsIntent(
  left: ComposerSettingsMutationIntent,
  right: ComposerSettingsMutationIntent,
): boolean {
  return left.sessionKey === right.sessionKey
    && left.setting === right.setting
    && left.fingerprint === right.fingerprint;
}

export function buildComposerSettingsMutationFailure(
  error: unknown,
  intent: ComposerSettingsMutationIntent,
  t: I18nContextValue["t"],
): ComposerSettingsMutationFailure {
  const failure = projectMutationFailure(
    error,
    t("composer.session_settings_save_failed"),
  );
  const notApplied = failure.effect === "not_applied";
  const committed = failure.effect === "committed";
  return {
    blocksRepeat: !notApplied,
    effect: failure.effect,
    impact: notApplied
      ? t("composer.session_settings_not_applied_impact")
      : committed
        ? t("composer.session_settings_committed_impact")
        : t("composer.session_settings_unknown_impact"),
    intent,
    title: notApplied
      ? t("composer.session_settings_save_failed")
      : committed
        ? t("composer.session_settings_committed_title")
        : t("composer.session_settings_unknown_title"),
  };
}
