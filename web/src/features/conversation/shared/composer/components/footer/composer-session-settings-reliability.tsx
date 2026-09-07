// INPUT: Composer Provider/Connector/Session-setting 读取与 mutation 失败投影。
// OUTPUT: 单一就近失败面；写入未知优先，恢复动作按资源投影并在读取中防重。
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
  const mutationFailure = controller.mutationFailure;
  const readFailure = controller.settingsReadFailure ?? controller.providerFailure ?? controller.connectorsFailure;
  const failure = mutationFailure ?? readFailure;
  if (!failure) return null;

  const retry = mutationFailure
    ? mutationFailure.blocksRepeat ? {
      busy: controller.settingsLoading,
      onClick: () => void controller.retrySessionSettings(),
    } : undefined
    : readFailure ? getReadRecovery(controller, readFailure) : undefined;

  return (
    <div className="px-2" data-composer-settings-reliability>
      <UiResourceState
        impact={failure.impact}
        primaryAction={retry ? {
          ...retry,
          label: t(mutationFailure ? "state.reload_check" : "state.retry"),
        } : undefined}
        size="sm"
        state="error"
        title={failure.title}
        variant="card"
      />
    </div>
  );
}

function getReadRecovery(
  controller: ComposerSessionSettingsController,
  failure: ComposerReadFailure,
): { busy: boolean; onClick: () => void } | undefined {
  switch (failure.resource) {
    case "connectors":
      return { busy: controller.connectorsLoading, onClick: controller.retryConnectors };
    case "providers":
      return { busy: controller.providerOptionsLoading, onClick: controller.retryProviderOptions };
    case "session_settings":
      return { busy: controller.settingsLoading, onClick: () => void controller.retrySessionSettings() };
    case "models":
    case "skills":
      return undefined;
  }
}
