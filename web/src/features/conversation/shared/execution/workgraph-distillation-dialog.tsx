/**
 * INPUT: 默认对话模型从 exact 完成图抽取的临时草图。
 * OUTPUT: 命名表单、完整工作图画布、保存/调整动作，以及明确说明事务保存结果的恢复状态。
 * POS: 完成态 WorkGraph 到持久化流程的确认台；只向编辑器提交本次表单修改，命名预检不替代服务端栅栏。
 */
"use client";

import { useCallback, useEffect, useState } from "react";
import {
  CheckCircle2,
  LoaderCircle,
  MessageSquareText,
  SquareTerminal,
  Workflow,
} from "lucide-react";

import { getWorkGraphWorkflowSaveStateApi, saveWorkGraphWorkflowApi } from "@/lib/api/conversation/execution-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import { WORKGRAPH_WORKFLOWS_CHANGED_EVENT } from "@/lib/conversation/workgraph-workflow-events";
import {
  projectMutationFailure,
  type MutationFailureEffect,
} from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogCloseButton,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiField, UiInput, UiTextarea } from "@/shared/ui/form/form-control";
import type { Agent } from "@/types/agent/agent";
import type { WorkGraphWorkflow, WorkGraphWorkflowPreview } from "@/types/conversation/workgraph-workflow";

import { WorkGraphMetadataEditorDialog } from "./workgraph-metadata-editor-dialog";
import { WorkGraphWorkflowCanvasPreview } from "./workgraph-workflow-canvas-preview";
import { useWorkGraphSlashNameAvailability } from "./use-workgraph-slash-name-availability";
import { savedWorkGraphMatchesPreview } from "./workgraph-save-state";

const WORKGRAPH_SLASH_NAME_PATTERN = /^[a-z][a-z0-9-]{0,63}$/;

interface WorkGraphSaveFailure {
  effect: MutationFailureEffect;
  changed?: boolean;
}

export function WorkGraphDistillationDialog({
  agents,
  onClose,
  onSaved,
  preview,
  sessionKey,
}: {
  agents: readonly Agent[];
  onClose: () => void;
  onSaved?: (workflow: WorkGraphWorkflow) => void;
  preview: WorkGraphWorkflowPreview;
  sessionKey: string;
}) {
  const { t } = useI18n();
  const [saveState, setSaveState] = useState<"idle" | "saving" | "saved">("idle");
  const [saveFailure, setSaveFailure] = useState<WorkGraphSaveFailure | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [savedWorkflow, setSavedWorkflow] = useState<WorkGraphWorkflow>();
  const [editorOpen, setEditorOpen] = useState(false);
  const [editorMetadata, setEditorMetadata] = useState<Partial<Pick<WorkGraphWorkflowPreview, "slash_name" | "title" | "description">>>({});
  const [workingPreview, setWorkingPreview] = useState(preview);
  const [slashName, setSlashName] = useState(preview.slash_name);
  const [title, setTitle] = useState(preview.title);
  const [description, setDescription] = useState(preview.description ?? "");
  const [confirmedConflictName, setConfirmedConflictName] = useState<string | null>(null);
  const loadCurrentDraft = useCallback(async () => {
    setLoading(true);
    setLoadFailed(false);
    try {
      const current = await getWorkGraphWorkflowSaveStateApi(sessionKey, preview.preview_id);
      setWorkingPreview(current.preview);
      setSlashName(current.preview.slash_name);
      setTitle(current.preview.title);
      setDescription(current.preview.description ?? "");
      setSavedWorkflow(current.workflow);
      setSaveFailure(null);
      setSaveState("idle");
    } catch {
      setLoadFailed(true);
    } finally {
      setLoading(false);
    }
  }, [preview.preview_id, sessionKey]);
  useEffect(() => { void loadCurrentDraft(); }, [loadCurrentDraft]);
  const locked = loading || loadFailed || saveState !== "idle" || Boolean(saveFailure && (saveFailure.changed || saveFailure.effect !== "not_applied"));
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
  const handleOpenEditor = () => {
    const nextMetadata = {
      slash_name: normalizedSlashName,
      title: title.trim(),
      description: description.trim(),
    };
    setEditorMetadata({
      ...(nextMetadata.slash_name !== workingPreview.slash_name ? { slash_name: nextMetadata.slash_name } : {}),
      ...(nextMetadata.title !== workingPreview.title ? { title: nextMetadata.title } : {}),
      ...(nextMetadata.description !== (workingPreview.description ?? "") ? { description: nextMetadata.description } : {}),
    });
    setEditorOpen(true);
  };
  const handleSave = async () => {
    if (locked) return;
    setSaveState("saving");
    setSaveFailure(null);
    try {
      const receipt = await saveWorkGraphWorkflowApi(sessionKey, workingPreview.preview_id, {
        head_revision: workingPreview.head_revision,
        selected_revision: workingPreview.selected_revision,
        description: description.trim(),
        slash_name: normalizedSlashName,
        title: title.trim(),
      });
      if (receipt.status !== "saved" || !receipt.workflow) {
        throw new Error("WorkGraph save was not confirmed");
      }
      setSlashName(receipt.workflow.slash_name);
      setSaveState("saved");
      setSavedWorkflow(receipt.workflow);
      onSaved?.(receipt.workflow);
      window.dispatchEvent(new CustomEvent(WORKGRAPH_WORKFLOWS_CHANGED_EVENT));
    } catch (reason: unknown) {
      if (reason instanceof ApiRequestError && reason.status === 409) {
        setConfirmedConflictName(normalizedSlashName);
      } else if (reason instanceof ApiRequestError && reason.status === 412) {
        setSaveFailure({ effect: "not_applied", changed: true });
      } else {
        const failure = projectMutationFailure(
          reason,
          t("execution.workflow_schedule_failed"),
        );
        setSaveFailure({ effect: failure.effect });
      }
      setSaveState("idle");
    }
  };
  const verifySave = async () => {
    setLoading(true);
    try {
      const current = await getWorkGraphWorkflowSaveStateApi(sessionKey, workingPreview.preview_id);
      setSavedWorkflow(current.workflow);
      const intended = { ...workingPreview, slash_name: normalizedSlashName, title: title.trim(), description: description.trim() };
      if (current.workflow && savedWorkGraphMatchesPreview(current.workflow, intended)) {
        setSaveFailure(null);
        setSaveState("saved");
        onSaved?.(current.workflow);
        window.dispatchEvent(new CustomEvent(WORKGRAPH_WORKFLOWS_CHANGED_EVENT));
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        layer="dialogUnderlay"
        labelledBy="workgraph-distillation-dialog-title"
        onClose={onClose}
        trapFocus={!editorOpen}
      >
        <UiDialogShell
          className="pointer-events-auto"
          size="workbench"
          viewport="workbench"
        >
          <UiDialogCloseButton
            className="absolute right-5 top-5 z-30"
            onClose={onClose}
          />
          <div className="grid min-h-0 flex-1 md:grid-cols-[360px_minmax(0,1fr)]">
            <aside className="flex min-h-0 flex-col border-r border-(--divider-subtle-color) bg-(--surface-muted-background) px-7 pb-6 pt-7">
              <header className="flex items-start gap-3 pr-10">
                <span className="radius-control-md grid h-9 w-9 shrink-0 place-items-center border border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--icon-default)">
                  <Workflow className="h-4 w-4" />
                </span>
                <div className="min-w-0 pt-0.5">
                  <h2
                    className="text-lg font-semibold leading-6 tracking-[-0.01em] text-(--text-strong)"
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
                        <LoaderCircle className={getUiSpinnerClassName({ size: "sm" })} />
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
                      className="pl-11"
                      disabled={locked}
                      id="workgraph-slash-name"
                      maxLength={64}
                      spellCheck={false}
                      textRole="code"
                      value={slashName}
                      variant="dialog"
                      onChange={(event) => {
                        setConfirmedConflictName(null);
                        setSlashName(event.target.value);
                      }}
                    />
                  </div>
                </UiField>

                {!loading && !loadFailed ? <p className="text-xs text-(--text-muted)">
                  {savedWorkflow
                    ? t("execution.workflow_current_command", { command: `/${savedWorkflow.slash_name}` })
                    : t("execution.workflow_no_saved_command")}
                </p> : null}

                <div className="space-y-4 border-t border-(--divider-subtle-color) pt-5">
                  <UiField htmlFor="workgraph-title" label={t("execution.workflow_title")}>
                    <UiInput
                      disabled={locked}
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
                      className="min-h-28 resize-none"
                      disabled={locked}
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
                {loadFailed ? (
                  <UiResourceState state="error" size="sm" variant="card"
                    title={t("execution.workflow_state_failed")}
                    impact={t("execution.workflow_state_failed_impact")}
                    primaryAction={{ label: t("execution.workflow_reload_draft"), onClick: () => void loadCurrentDraft() }} />
                ) : null}
                {saveState === "saved" ? (
                  <div className="flex items-start gap-2 text-xs text-(--text-default)">
                    <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-(--success)" />
                    <div>
                      <div className="font-semibold text-(--text-strong)">{t("execution.workflow_saved_title")}</div>
                      <div className="mt-0.5 leading-5 text-(--text-muted)">
                    {t("execution.workflow_saved_message", {
                      command: `/${normalizedSlashName}`,
                    })}
                      </div>
                    </div>
                  </div>
                ) : null}
                {saveFailure ? (
                  <WorkGraphSaveFailureState failure={saveFailure} checking={loading}
                    onCheck={() => { void verifySave().catch(() => {}); }} onReload={() => void loadCurrentDraft()} />
                ) : null}
                {saveState === "saved" || (saveFailure && saveFailure.effect !== "not_applied") ? (
                  <UiButton className="w-full" onClick={onClose} size="sm" tone="primary" variant="solid">
                    {t("common.close")}
                  </UiButton>
                ) : (
                  <UiButton className="w-full" disabled={locked || metadataError !== null || !slashNameAvailable} onClick={() => void handleSave()} size="sm" tone="primary" variant="solid">
                    {saveState === "saving" ? (
                      <LoaderCircle className={getUiSpinnerClassName({ size: "sm" })} />
                    ) : null}
                    {t(saveState === "saving"
                      ? "execution.workflow_scheduling"
                      : saveFailure
                        ? "state.retry"
                        : "execution.workflow_save_sketch")}
                  </UiButton>
                )}
              </div>
            </aside>

            <main className="flex min-h-0 flex-col bg-(--surface-canvas-background)">
              <header className="flex shrink-0 items-center justify-between gap-4 border-b border-(--divider-subtle-color) px-7 py-5 pr-16">
                <h3 className="text-sm font-semibold text-(--text-strong)">
                  {t("execution.workflow_sketch_label")}
                </h3>
                <UiButton
                  disabled={locked || metadataError !== null}
                  onClick={handleOpenEditor}
                  size="sm"
                  variant="surface"
                >
                  <MessageSquareText className="h-3.5 w-3.5" />
                  {t("execution.workflow_edit_with_chat")}
                </UiButton>
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
          savedCommandName={savedWorkflow?.slash_name}
          metadata={editorMetadata}
          onMetadataApplied={(acceptedPreview) => {
            // Advance the baseline only after the editor accepts these edits.
            // A plain reopen must preserve later chat edits; a failed start must retry them.
            setWorkingPreview((current) => {
              const confirmed = { ...current, ...editorMetadata };
              // Carry the accepted revision only when the visible graph is unchanged.
              // Other chat edits still require the editor's explicit Apply action.
              return savedWorkGraphMatchesPreview(acceptedPreview, confirmed) ? acceptedPreview : confirmed;
            });
          }}
          preview={workingPreview}
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
  checking,
  onCheck,
  onReload,
}: {
  failure: WorkGraphSaveFailure;
  checking: boolean;
  onCheck: () => void;
  onReload: () => void;
}) {
  const { t } = useI18n();
  const notApplied = failure.effect === "not_applied";
  const title = failure.changed ? t("execution.workflow_draft_changed") : notApplied
    ? t("execution.workflow_save_not_applied_title")
    : failure.effect === "accepted"
      ? t("execution.workflow_save_accepted_title")
      : failure.effect === "committed"
        ? t("execution.workflow_save_committed_title")
        : t("execution.workflow_save_unknown_title");
  return (
    <UiResourceState
      className="min-h-0 py-3"
      impact={t(failure.changed ? "execution.workflow_draft_changed_impact" : notApplied
        ? "execution.workflow_save_not_applied_impact"
        : "execution.workflow_save_unknown_impact")}
      size="sm"
      primaryAction={failure.changed
        ? { label: t("execution.workflow_reload_draft"), onClick: onReload }
        : !notApplied ? { label: t("execution.workflow_check_save"), busy: checking, onClick: onCheck } : undefined}
      state="error"
      title={title}
      urgency="polite"
      variant="card"
    />
  );
}
