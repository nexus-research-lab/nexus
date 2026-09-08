// INPUT: Composer Provider/Connector/Session-setting 读取与 mutation 失败投影。
// OUTPUT: 就近、持久、polite 的 Problem/Impact/Recovery 状态和显式动作。
// POS: Composer Session controls 共用可见错误面；不把读取当作 mutation 对账。
import { useI18n } from "@/shared/i18n/i18n-context";
import { ChevronRight, CircleAlert, RotateCw } from "lucide-react";
import { useId, useState } from "react";
import { UiDialogPortal, UiDialogBackdrop, UiDialogShell, UiDialogHeader, UiDialogBody } from "@/shared/ui/dialog/dialog";

import type { ComposerSessionSettingsController } from "../../controller/use-composer-session-settings";
import type { ComposerReadFailure } from "../../controller/composer-settings-reliability";

export function ComposerSessionSettingsReliability({
  controller,
}: {
  controller: ComposerSessionSettingsController;
}) {
  const { t } = useI18n();
  const [isDialogOpen, setDialogOpen] = useState(false);
  const readFailures = [
    controller.settingsReadFailure,
    controller.providerFailure,
    controller.connectorsFailure,
  ].filter((failure): failure is ComposerReadFailure => Boolean(failure));
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
  const canRetry = Boolean(activeReadFailure);
  const retrying = activeReadFailure
    ? isReadRetrying(controller, activeReadFailure)
    : controller.settingsLoading;

  const retryLabel = t(activeReadFailure ? "state.retry" : "state.reload_check");

  return (
    <div
      className="mb-2 min-w-0 px-3 text-left text-xs text-(--text-soft)"
      data-composer-settings-reliability
    >
      <div className="flex min-w-0 items-center gap-1">
        <button
          aria-haspopup="dialog"
          className="flex min-h-8 min-w-0 items-center gap-2 rounded-md text-left focus-visible:outline-2 focus-visible:outline-(--text-strong)"
          onClick={() => setDialogOpen(true)}
          type="button"
        >
          <CircleAlert aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-(--destructive)" />
          <span aria-live="polite" className="min-w-0 truncate" title={failure.title}>
            {failure.title}
          </span>
          <ChevronRight aria-hidden="true" className="h-3.5 w-3.5 shrink-0" />
        </button>
        {canRetry ? (
          <button
            aria-label={retryLabel}
            className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-(--icon-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:outline-2 focus-visible:outline-(--text-strong) disabled:cursor-wait disabled:opacity-50"
            disabled={retrying}
            onClick={() => activeReadFailure
              ? retryReadFailure(controller, activeReadFailure)
              : void controller.retrySessionSettings()}
            title={retryLabel}
            type="button"
          >
            <RotateCw aria-hidden="true" className={`h-3.5 w-3.5 ${retrying ? "animate-spin motion-reduce:animate-none" : ""}`} />
          </button>
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
