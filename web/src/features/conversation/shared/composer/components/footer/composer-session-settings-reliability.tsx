// INPUT: Composer Provider/Connector/Session-setting 读取与 mutation 失败投影。
// OUTPUT: 自动弹出的写入失败 Dialog，及可展开、可独立重试的紧凑读取失败提示。
// POS: Composer Session controls 共用可见错误面；不把读取当作 mutation 对账。
import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { ChevronRight, CircleAlert, RotateCw } from "lucide-react";
import { useId } from "react";
import { UiDialogPortal, UiDialogBackdrop, UiDialogShell, UiDialogHeader, UiDialogBody } from "@/shared/ui/dialog/dialog";

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
  const [isDialogOpen, setDialogOpen] = useResettableState(false, JSON.stringify([
    controller.target?.sessionKey, readFailures[0]?.resource ?? null, Boolean(controller.mutationFailure),
  ]));
  if (readFailures.length === 0 && !controller.mutationFailure) {
    return null;
  }
  if (controller.mutationFailure) {
    return (
      <ComposerSettingsFailureDialog
        title={controller.mutationFailure.title}
        impact={controller.mutationFailure.impact}
        onClose={controller.dismissMutationFailure}
      />
    );
  }
  const activeReadFailure = readFailures[0] ?? null;

  const failure = activeReadFailure;
  if (!failure) return null;
  const recovery = getReadRecovery(controller, activeReadFailure);
  const retryLabel = t("state.retry");

  return (
    <div
      className="mb-2 min-w-0 px-3 text-left text-xs text-(--text-soft)"
      data-composer-settings-reliability
    >
      <div className="flex min-w-0 items-center gap-1">
        <UiButton
          aria-haspopup="dialog"
          className="min-w-0 justify-start"
          onClick={() => setDialogOpen(true)}
          size="xs"
          variant="text"
        >
          <CircleAlert aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-(--destructive)" />
          <UiTooltip label={failure.title}><span aria-live="polite" className="min-w-0 truncate" >
            {failure.title}
          </span></UiTooltip>
          <ChevronRight aria-hidden="true" className="h-3.5 w-3.5 shrink-0" />
        </UiButton>
        {recovery ? (
          <UiIconButton
            aria-busy={recovery.busy}
            aria-label={retryLabel}
            disabled={recovery.busy}
            onClick={recovery.run}
            size="sm"
            tooltip={retryLabel}
            variant="ghost"
          >
            <RotateCw
              aria-hidden="true"
              className={recovery.busy
                ? getUiSpinnerClassName({ size: "sm", tone: "muted" })
                : "h-3.5 w-3.5"}
            />
          </UiIconButton>
        ) : null}
      </div>
      {isDialogOpen ? (
        <ComposerSettingsFailureDialog
          title={failure.title}
          impact={failure.impact}
          onClose={() => setDialogOpen(false)}
        />
      ) : null}
    </div>
  );
}

function getReadRecovery(
  controller: ComposerSessionSettingsController,
  failure: ComposerReadFailure,
): { busy: boolean; run: () => void } | null {
  switch (failure.resource) {
    case "connectors":
      return { busy: controller.connectorsLoading, run: controller.retryConnectors };
    case "providers":
      return { busy: controller.providerOptionsLoading, run: controller.retryProviderOptions };
    case "session_settings":
      return { busy: controller.settingsLoading, run: () => void controller.retrySessionSettings() };
    case "models":
    case "skills":
      return null;
  }
}

/** 修改失败时直接打开模态框，关闭后由控制器清除本次错误。 */
function ComposerSettingsFailureDialog({ title, impact, onClose }: {
  title: string;
  impact: string;
  onClose: () => void;
}) {
  const titleId = useId();
  const descriptionId = useId();
  return (
    <UiDialogPortal>
      <UiDialogBackdrop describedBy={descriptionId} labelledBy={titleId} onClose={onClose}>
        <UiDialogShell size="md">
          <UiDialogHeader appearance="plain" onClose={onClose} title={title} titleId={titleId} />
          <UiDialogBody>
            <p className="break-words text-left text-sm leading-relaxed text-(--text-soft)" id={descriptionId}>
              {impact}
            </p>
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
