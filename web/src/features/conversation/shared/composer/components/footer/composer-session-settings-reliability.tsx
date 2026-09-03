// INPUT: Composer Provider/Connector/Session-setting 读取与 mutation 失败投影。
// OUTPUT: 就近、持久、polite 的 Problem/Impact/Recovery 状态和显式动作。
// POS: Composer Session controls 共用可见错误面；不把读取当作 mutation 对账。
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";

import type { ComposerSessionSettingsController } from "../../controller/use-composer-session-settings";
import type { ComposerReadFailure } from "../../controller/composer-settings-reliability";

export function ComposerSessionSettingsReliability({
  controller,
}: {
  controller: ComposerSessionSettingsController;
}) {
  const { t } = useI18n();
  const readFailures = [
    controller.settingsReadFailure,
    controller.providerFailure,
    controller.connectorsFailure,
  ].filter((failure): failure is ComposerReadFailure => Boolean(failure));
  if (readFailures.length === 0 && !controller.mutationFailure) {
    return null;
  }
  const activeReadFailure = controller.mutationFailure ? null : readFailures[0] ?? null;

  return (
    <div className="px-2" data-composer-settings-reliability>
      {activeReadFailure ? (
        <UiResourceState
          impact={activeReadFailure.impact}
          primaryAction={{
            busy: isReadRetrying(controller, activeReadFailure),
            label: t("state.retry"),
            onClick: () => retryReadFailure(controller, activeReadFailure),
          }}
          size="sm"
          state="error"
          title={activeReadFailure.title}
          variant="card"
        />
      ) : null}
      {controller.mutationFailure ? (
        <UiResourceState
          impact={controller.mutationFailure.impact}
          primaryAction={controller.mutationFailure.blocksRepeat ? {
            label: t("state.reload_check"),
            onClick: () => void controller.retrySessionSettings(),
          } : undefined}
          size="sm"
          state="error"
          title={controller.mutationFailure.title}
          variant="card"
        />
      ) : null}
    </div>
  );
}

function isReadRetrying(
  controller: ComposerSessionSettingsController,
  failure: ComposerReadFailure,
): boolean {
  switch (failure.resource) {
    case "connectors":
      return controller.connectorsLoading;
    case "providers":
      return controller.providerOptionsLoading;
    case "session_settings":
      return controller.settingsLoading;
    case "models":
    case "skills":
      return false;
  }
}

function retryReadFailure(
  controller: ComposerSessionSettingsController,
  failure: ComposerReadFailure,
): void {
  switch (failure.resource) {
    case "connectors":
      controller.retryConnectors();
      return;
    case "providers":
      controller.retryProviderOptions();
      return;
    case "session_settings":
      void controller.retrySessionSettings();
      return;
    case "models":
    case "skills":
      return;
  }
}
