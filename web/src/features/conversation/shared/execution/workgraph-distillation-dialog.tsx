/**
 * INPUT: 默认对话模型从 exact 完成图抽取的临时草图。
 * OUTPUT: 左侧命名表单、右侧低文字结构预览，以及明确的保存/调整动作。
 * POS: 完成态 WorkGraph 到持久化流程的克制确认台，不展示模型或后台实现自述。
 */
"use client";

import { useState } from "react";
import { CheckCircle2, LoaderCircle, MessageSquareText } from "lucide-react";

import { scheduleWorkGraphWorkflowSaveApi } from "@/lib/api/conversation/execution-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogCloseButton,
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
              <header className="pr-10">
                <h2
                  className="text-[19px] font-semibold tracking-[-0.01em] text-(--text-strong)"
                  id="workgraph-distillation-dialog-title"
                >
                  {t("execution.workflow_distill_title")}
                </h2>
                <code className="mt-2 inline-flex rounded-md bg-(--surface-panel-background) px-2 py-1 text-xs text-(--text-muted)">
                  /{normalizedSlashName || "—"}
                </code>
              </header>

              <div className="soft-scrollbar mt-7 min-h-0 flex-1 space-y-4 overflow-y-auto pr-1">
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
                error={metadataError && (!title.trim() || !description.trim()) ? metadataError : null}
                htmlFor="workgraph-description"
                label={t("execution.workflow_description")}
              >
                <UiTextarea
                  className="min-h-28 resize-none"
                  disabled={saveState !== "idle"}
                  id="workgraph-description"
                  maxLength={500}
                  value={description}
                  variant="dialog"
                  onChange={(event) => setDescription(event.target.value)}
                />
              </UiField>
              </div>

              <div className="mt-5 space-y-3">
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
                {saveError ? (
                  <p className="text-xs leading-5 text-(--destructive)" role="alert">{saveError}</p>
                ) : null}
                {saveState === "scheduled" ? (
                  <button className={`${getDialogActionClassName("primary", "compact")} w-full`} type="button" onClick={onClose}>
                    {t("common.close")}
                  </button>
                ) : (
                  <button className={`${getDialogActionClassName("primary", "compact")} w-full`} disabled={saveState === "saving" || metadataError !== null} type="button" onClick={() => void handleSave()}>
                    {saveState === "saving" ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
                    {t(saveState === "saving" ? "execution.workflow_scheduling" : "execution.workflow_save_sketch")}
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
              <div className="soft-scrollbar min-h-0 flex-1 overflow-auto">
                <NamedWorkGraphSketch
                  appearance="editor"
                  className="min-h-full"
                  dependencies={workingPreview.dependencies}
                  nodes={workingPreview.nodes}
                />
              </div>
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
