/**
 * INPUT: 默认后台模型从 exact 完成图抽取的临时草图。
 * OUTPUT: 用户可修订命令、说明与图结构后保存或放弃。
 * POS: 完成态 WorkGraph 到后台 execution-orchestrator Skill + Nexus CLI 的确认层。
 */
"use client";

import { useState } from "react";
import { CheckCircle2, GitBranchPlus, LoaderCircle, MessageSquareText, Sparkles } from "lucide-react";

import { scheduleWorkGraphWorkflowSaveApi } from "@/lib/api/conversation/execution-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { getDialogActionClassName } from "@/shared/ui/dialog/dialog-styles";
import { UiField, UiInput, UiTextarea } from "@/shared/ui/form/form-control";
import type { Agent } from "@/types/agent/agent";
import type { WorkGraphWorkflowPreview } from "@/types/conversation/workgraph-workflow";

import { NamedWorkGraphSketch } from "./named-workgraph-sketch";
import { WorkGraphMetadataEditorDialog } from "./workgraph-metadata-editor-dialog";

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
  const [saveError, setSaveError] = useState<string | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [workingPreview, setWorkingPreview] = useState(preview);
  const [slashName, setSlashName] = useState(preview.slash_name);
  const [title, setTitle] = useState(preview.title);
  const [description, setDescription] = useState(preview.description ?? "");
  const normalizedSlashName = slashName.trim().replace(/^\/+/, "").toLowerCase();
  const metadataError = !/^[a-z][a-z0-9-]{0,63}$/.test(normalizedSlashName)
    ? t("execution.workflow_slash_invalid")
    : !title.trim() || !description.trim()
      ? t("execution.workflow_metadata_required")
      : null;

  const handleSave = async () => {
    setSaveState("saving");
    setSaveError(null);
    try {
      await scheduleWorkGraphWorkflowSaveApi(sessionKey, workingPreview.preview_id, {
        description: description.trim(),
        slash_name: normalizedSlashName,
        title: title.trim(),
      });
      setSaveState("scheduled");
    } catch (reason: unknown) {
      setSaveError(getErrorMessage(reason, t("execution.workflow_schedule_failed")));
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
        <UiDialogShell className="pointer-events-auto max-h-[86vh]" size="xl">
          <UiDialogHeader
            icon={<GitBranchPlus className="h-4 w-4" />}
            iconClassName="text-(--primary)"
            onClose={onClose}
            subtitle={t("execution.workflow_distill_subtitle")}
            title={t("execution.workflow_distill_title")}
            titleId="workgraph-distillation-dialog-title"
          />
          <UiDialogBody className="flex flex-col gap-4" scrollable>
            <div className="flex items-center justify-between gap-3">
              <div className="text-xs font-semibold text-(--text-strong)">{t("execution.workflow_metadata_title")}</div>
              <div className="flex items-center gap-2">
                <span className="inline-flex items-center gap-1 rounded-full border border-[color:color-mix(in_srgb,var(--primary)_22%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] px-2 py-1 text-[10px] font-medium text-(--primary)">
                  <Sparkles className="h-3 w-3" />
                  {t("execution.workflow_model_extracted")}
                </span>
                <button
                  className={getDialogActionClassName("default", "compact")}
                  disabled={saveState !== "idle" || metadataError !== null}
                  type="button"
                  onClick={() => setEditorOpen(true)}
                >
                  <MessageSquareText className="h-3.5 w-3.5" />
                  {t("execution.workflow_edit_with_chat")}
                </button>
              </div>
            </div>

            <div className="grid gap-3 rounded-[12px] border border-(--divider-subtle-color) bg-(--surface-muted-background) p-3 sm:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)]">
              <UiField
                error={metadataError && !/^[a-z][a-z0-9-]{0,63}$/.test(normalizedSlashName) ? metadataError : null}
                htmlFor="workgraph-slash-name"
                label={t("execution.workflow_slash_name")}
              >
                <div className="relative">
                  <span className="pointer-events-none absolute inset-y-0 left-3 flex items-center font-mono text-sm text-(--text-soft)">/</span>
                  <UiInput
                    autoCapitalize="none"
                    autoComplete="off"
                    className="pl-6 font-mono"
                    disabled={saveState !== "idle"}
                    id="workgraph-slash-name"
                    maxLength={64}
                    spellCheck={false}
                    value={slashName}
                    variant="dialog"
                    onChange={(event) => setSlashName(event.target.value)}
                  />
                </div>
              </UiField>
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
                className="sm:col-span-2"
                error={metadataError && (!title.trim() || !description.trim()) ? metadataError : null}
                htmlFor="workgraph-description"
                label={t("execution.workflow_description")}
              >
                <UiTextarea
                  className="min-h-20 resize-none"
                  disabled={saveState !== "idle"}
                  id="workgraph-description"
                  maxLength={500}
                  value={description}
                  variant="dialog"
                  onChange={(event) => setDescription(event.target.value)}
                />
              </UiField>
            </div>

            <NamedWorkGraphSketch
              className="min-h-56"
              dependencies={workingPreview.dependencies}
              nodes={workingPreview.nodes}
            />

            {saveState === "scheduled" ? (
              <div className="flex items-start gap-2 rounded-[10px] border border-[color:color-mix(in_srgb,var(--success)_28%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--success)_7%,transparent)] px-3 py-2.5 text-xs text-(--text-default)">
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
          </UiDialogBody>
          <UiDialogFooter className={saveState === "scheduled" ? "justify-end gap-3" : "justify-between gap-3"}>
            {saveState !== "scheduled" ? (
              <span className={`max-w-lg text-xs leading-4 ${saveError ? "text-(--destructive)" : "text-(--text-muted)"}`}>
                {saveError ?? t("execution.workflow_reuse_notice", {
                  command: `/${normalizedSlashName}`,
                })}
              </span>
            ) : null}
            <div className="flex shrink-0 gap-2">
              {saveState === "scheduled" ? (
                <button className={getDialogActionClassName("primary", "compact")} type="button" onClick={onClose}>
                  {t("common.close")}
                </button>
              ) : (
                <>
                  <button className={getDialogActionClassName("default", "compact")} disabled={saveState === "saving"} type="button" onClick={onClose}>
                    {t("execution.workflow_discard_sketch")}
                  </button>
                  <button className={getDialogActionClassName("primary", "compact")} disabled={saveState === "saving" || metadataError !== null} type="button" onClick={() => void handleSave()}>
                    {saveState === "saving" ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
                    {t(saveState === "saving" ? "execution.workflow_scheduling" : "execution.workflow_save_sketch")}
                  </button>
                </>
              )}
            </div>
          </UiDialogFooter>
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
