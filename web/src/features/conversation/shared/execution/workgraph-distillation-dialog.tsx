/**
 * INPUT: 默认对话模型从 exact 完成图抽取的临时草图。
 * OUTPUT: 命名表单、完整工作图画布、保存/调整动作，以及明确说明保存调度结果的恢复状态。
 * POS: 完成态 WorkGraph 到持久化流程的确认台；命名预检不替代服务端栅栏，同一 preview 的后台调度可安全复用。
 */
"use client";

import { useState } from "react";
import {
  CheckCircle2,
  LoaderCircle,
  MessageSquareText,
  SquareTerminal,
  Workflow,
} from "lucide-react";

import { scheduleWorkGraphWorkflowSaveApi } from "@/lib/api/conversation/execution-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import {
  projectMutationFailure,
  type MutationFailureEffect,
} from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogCloseButton,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { getDialogActionClassName } from "@/shared/ui/dialog/dialog-styles";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiField, UiInput, UiTextarea } from "@/shared/ui/form/form-control";
import type { Agent } from "@/types/agent/agent";
import type { WorkGraphWorkflowPreview } from "@/types/conversation/workgraph-workflow";

import { WorkGraphMetadataEditorDialog } from "./workgraph-metadata-editor-dialog";
import { WorkGraphWorkflowCanvasPreview } from "./workgraph-workflow-canvas-preview";
import { useWorkGraphSlashNameAvailability } from "./use-workgraph-slash-name-availability";

const WORKGRAPH_SLASH_NAME_PATTERN = /^[a-z][a-z0-9-]{0,63}$/;

interface WorkGraphSaveFailure {
  effect: MutationFailureEffect;
  message: string;
}

export function WorkGraphDistillationDialog({
  agents,
  onClose,
  preview,
  sessionKey,
}: {
  agents: readonly Agent[];
  onClose: () => void;
  preview: WorkGraphWorkflowPreview;
  sessionKey: string;
}) {
  const { t } = useI18n();
  const [saveState, setSaveState] = useState<"idle" | "saving" | "scheduled">("idle");
  const [saveFailure, setSaveFailure] = useState<WorkGraphSaveFailure | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [workingPreview, setWorkingPreview] = useState(preview);
  const [slashName, setSlashName] = useState(preview.slash_name);
  const [title, setTitle] = useState(preview.title);
  const [description, setDescription] = useState(preview.description ?? "");
  const [confirmedConflictName, setConfirmedConflictName] = useState<string | null>(null);
  const normalizedSlashName = slashName.trim().replace(/^\/+/, "").toLowerCase();
  const slashNameFormatError = !WORKGRAPH_SLASH_NAME_PATTERN.test(normalizedSlashName)
    ? t("execution.workflow_slash_invalid")
    : null;
  const slashNameAvailability = useWorkGraphSlashNameAvailability({
    enabled: slashNameFormatError === null && saveState === "idle",
    previewId: workingPreview.preview_id,
    slashName: normalizedSlashName,
  });
  const availabilityMatchesInput = slashNameAvailability.slashName === normalizedSlashName;
  const slashNameUnavailable = confirmedConflictName === normalizedSlashName || (
    availabilityMatchesInput && slashNameAvailability.status === "unavailable"
  );
  const slashNameCheckFailed = availabilityMatchesInput && slashNameAvailability.status === "error";
  const slashNameAvailable = confirmedConflictName !== normalizedSlashName
    && availabilityMatchesInput
    && slashNameAvailability.status === "available";
  const slashNameError = slashNameFormatError
    ?? (slashNameUnavailable ? t("execution.workflow_slash_unavailable") : null)
    ?? (slashNameCheckFailed ? t("execution.workflow_slash_check_failed") : null);
  const metadataError = slashNameError
    ? slashNameError
    : !title.trim() || !description.trim()
      ? t("execution.workflow_metadata_required")
      : null;
  const handleSave = async () => {
    setSaveState("saving");
    setSaveFailure(null);
    try {
      await scheduleWorkGraphWorkflowSaveApi(sessionKey, workingPreview.preview_id, {
        description: description.trim(),
        slash_name: normalizedSlashName,
        title: title.trim(),
      });
      setSaveState("scheduled");
    } catch (reason: unknown) {
      if (reason instanceof ApiRequestError && reason.status === 409) {
        setConfirmedConflictName(normalizedSlashName);
      } else {
        const failure = projectMutationFailure(
          reason,
          t("execution.workflow_schedule_failed"),
        );
        setSaveFailure({ effect: failure.effect, message: failure.message });
      }
      setSaveState("idle");
    }
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9998]"
        labelledBy="workgraph-distillation-dialog-title"
        onClose={onClose}
        trapFocus={!editorOpen}
      >
        <UiDialogShell
          className="pointer-events-auto h-[min(760px,calc(100dvh-64px))] max-h-[calc(100dvh-64px)]"
          size="wide"
          style={{ maxWidth: "min(1180px, calc(100vw - 64px))" }}
        >
          <UiDialogCloseButton
            className="absolute right-5 top-5 z-30"
            onClose={onClose}
          />
          <div className="grid min-h-0 flex-1 md:grid-cols-[360px_minmax(0,1fr)]">
            <aside className="flex min-h-0 flex-col border-r border-(--divider-subtle-color) bg-(--surface-muted-background) px-7 pb-6 pt-7">
              <header className="flex items-start gap-3 pr-10">
                <span className="radius-control-md grid h-9 w-9 shrink-0 place-items-center border border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--icon-default) shadow-[0_1px_0_color-mix(in_srgb,var(--text-strong)_4%,transparent)]">
                  <Workflow className="h-4 w-4" />
                </span>
                <div className="min-w-0 pt-0.5">
                  <h2
                    className="text-[19px] font-semibold leading-6 tracking-[-0.01em] text-(--text-strong)"
                    id="workgraph-distillation-dialog-title"
                  >
                    {t("execution.workflow_distill_title")}
                  </h2>
                  <p className="mt-1 text-xs text-(--text-muted)">
                    {t("execution.workflow_sketch_label")} · {workingPreview.nodes.length} {t("execution.workflow_nodes_short")}
                  </p>
                </div>
              </header>

              <div className="soft-scrollbar mt-6 min-h-0 flex-1 space-y-5 overflow-y-auto pr-1">
                <UiField
                  className="surface-radius-md border border-[color:color-mix(in_srgb,var(--primary)_18%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--surface-panel-background)_72%,transparent)] p-3.5"
                  description={slashNameFormatError === null && availabilityMatchesInput ? (
                    slashNameAvailability.status === "checking" ? (
                      <span className="inline-flex items-center gap-1.5">
                        <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
                        {t("execution.workflow_slash_checking")}
                      </span>
                    ) : slashNameAvailable ? (
                      <span className="inline-flex items-center gap-1.5 text-(--success)">
                        <CheckCircle2 className="h-3.5 w-3.5" />
                        {t("execution.workflow_slash_available")}
                      </span>
                    ) : null
                  ) : null}
                  error={slashNameError}
                  htmlFor="workgraph-slash-name"
                  label={(
                    <span className="inline-flex items-center gap-1.5 font-medium text-(--text-default)">
                      <SquareTerminal className="h-3.5 w-3.5 text-(--primary)" />
                      {t("execution.workflow_slash_name")}
                    </span>
                  )}
                >
                  <div className="relative">
                    <span className="radius-control-xs pointer-events-none absolute bottom-1 left-1 top-1 z-10 grid w-8 place-items-center bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] font-mono text-sm font-semibold text-(--primary)">/</span>
                    <UiInput
                      autoCapitalize="none"
                      autoComplete="off"
                      className="h-10 pl-11 font-mono font-medium tracking-[-0.01em]"
                      disabled={saveState !== "idle"}
                      id="workgraph-slash-name"
                      maxLength={64}
                      spellCheck={false}
                      value={slashName}
                      variant="dialog"
                      onChange={(event) => {
                        setConfirmedConflictName(null);
                        setSlashName(event.target.value);
                      }}
                    />
                  </div>
                </UiField>

                <div className="space-y-4 border-t border-(--divider-subtle-color) pt-5">
                  <UiField htmlFor="workgraph-title" label={t("execution.workflow_title")}>
                    <UiInput
                      disabled={saveState !== "idle"}
                      id="workgraph-title"
                      maxLength={120}
                      value={title}
                      variant="dialog"
                      onChange={(event) => setTitle(event.target.value)}
                    />
                  </UiField>
                  <UiField
                    error={metadataError && (!title.trim() || !description.trim()) ? metadataError : null}
                    htmlFor="workgraph-description"
                    label={t("execution.workflow_description")}
                  >
                    <UiTextarea
                      className="min-h-28 resize-none leading-6"
                      disabled={saveState !== "idle"}
                      id="workgraph-description"
                      maxLength={500}
                      value={description}
                      variant="dialog"
                      onChange={(event) => setDescription(event.target.value)}
                    />
                  </UiField>
                </div>
              </div>

              <div className="mt-5 space-y-3 border-t border-(--divider-subtle-color) pt-5">
                {saveState === "scheduled" ? (
                  <div className="flex items-start gap-2 text-xs text-(--text-default)">
                    <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-(--success)" />
                    <div>
                      <div className="font-semibold text-(--text-strong)">{t("execution.workflow_scheduled_title")}</div>
                      <div className="mt-0.5 leading-5 text-(--text-muted)">
                    {t("execution.workflow_scheduled_message", {
                      command: `/${normalizedSlashName}`,
                    })}
                      </div>
                    </div>
                  </div>
                ) : null}
                {saveFailure ? (
                  <WorkGraphSaveFailureState failure={saveFailure} />
                ) : null}
                {saveState === "scheduled" ? (
                  <button className={`${getDialogActionClassName("primary", "compact")} w-full`} type="button" onClick={onClose}>
                    {t("common.close")}
                  </button>
                ) : (
                  <button className={`${getDialogActionClassName("primary", "compact")} w-full`} disabled={saveState === "saving" || metadataError !== null || !slashNameAvailable} type="button" onClick={() => void handleSave()}>
                    {saveState === "saving" ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
                    {t(saveState === "saving"
                      ? "execution.workflow_scheduling"
                      : saveFailure
                        ? "execution.workflow_save_confirm_again"
                        : "execution.workflow_save_sketch")}
                  </button>
                )}
              </div>
            </aside>

            <main className="flex min-h-0 flex-col bg-(--surface-canvas-background)">
              <header className="flex shrink-0 items-center justify-between gap-4 border-b border-(--divider-subtle-color) px-7 py-5 pr-16">
                <h3 className="text-sm font-semibold text-(--text-strong)">
                  {t("execution.workflow_sketch_label")}
                </h3>
                <button
                  className={getDialogActionClassName("default", "compact")}
                  disabled={saveState !== "idle" || metadataError !== null}
                  type="button"
                  onClick={() => setEditorOpen(true)}
                >
                  <MessageSquareText className="h-3.5 w-3.5" />
                  {t("execution.workflow_edit_with_chat")}
                </button>
              </header>
              <WorkGraphWorkflowCanvasPreview
                className="flex-1"
                revision={1}
                workflow={workingPreview}
              />
            </main>
          </div>
        </UiDialogShell>
      </UiDialogBackdrop>
      {editorOpen ? (
        <WorkGraphMetadataEditorDialog
          agents={agents}
          preview={{
            ...workingPreview,
            description: description.trim(),
            slash_name: normalizedSlashName,
            title: title.trim(),
          }}
          sessionKey={sessionKey}
          onApply={(nextPreview) => {
            setWorkingPreview(nextPreview);
            setConfirmedConflictName(null);
            setSlashName(nextPreview.slash_name);
            setTitle(nextPreview.title);
            setDescription(nextPreview.description ?? "");
            setEditorOpen(false);
          }}
          onClose={() => setEditorOpen(false)}
        />
      ) : null}
    </UiDialogPortal>
  );
}

function WorkGraphSaveFailureState({
  failure,
}: {
  failure: WorkGraphSaveFailure;
}) {
  const { t } = useI18n();
  const notApplied = failure.effect === "not_applied";
  const title = notApplied
    ? t("execution.workflow_save_not_applied_title")
    : failure.effect === "accepted"
      ? t("execution.workflow_save_accepted_title")
      : failure.effect === "committed"
        ? t("execution.workflow_save_committed_title")
        : t("execution.workflow_save_unknown_title");
  return (
    <UiResourceState
      className="min-h-0 py-3"
      description={failure.message}
      impact={t(notApplied
        ? "execution.workflow_save_not_applied_impact"
        : "execution.workflow_save_unknown_impact")}
      nextStep={t(notApplied
        ? "execution.workflow_save_not_applied_next_step"
        : "execution.workflow_save_unknown_next_step")}
      size="sm"
      state="error"
      title={title}
      urgency="polite"
      variant="card"
    />
  );
}
