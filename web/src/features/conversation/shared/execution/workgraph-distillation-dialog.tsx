/**
 * INPUT: 默认后台模型从 exact 完成图抽取的临时只读草图。
 * OUTPUT: 用户保存或放弃；保存启动不进入聊天时间线的内部 Agent round。
 * POS: 完成态 WorkGraph 到后台 execution-orchestrator Skill + Nexus CLI 的确认层。
 */
"use client";

import { useState } from "react";
import { CheckCircle2, GitBranchPlus, LoaderCircle, Sparkles } from "lucide-react";

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
import type { WorkGraphWorkflowPreview } from "@/types/conversation/workgraph-workflow";

import { NamedWorkGraphSketch } from "./named-workgraph-sketch";

export function WorkGraphDistillationDialog({
  onClose,
  preview,
  sessionKey,
}: {
  onClose: () => void;
  preview: WorkGraphWorkflowPreview;
  sessionKey: string;
}) {
  const { t } = useI18n();
  const [saveState, setSaveState] = useState<"idle" | "saving" | "scheduled">("idle");
  const [saveError, setSaveError] = useState<string | null>(null);

  const handleSave = async () => {
    setSaveState("saving");
    setSaveError(null);
    try {
      await scheduleWorkGraphWorkflowSaveApi(sessionKey, preview.preview_id);
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
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <h3 className="text-base font-semibold text-(--text-strong)">{preview.title}</h3>
                  <code className="rounded-[6px] bg-(--surface-muted-background) px-1.5 py-0.5 text-[11px] text-(--text-soft)">/{preview.slash_name}</code>
                </div>
                <p className="mt-1 max-w-2xl text-compact leading-5 text-(--text-muted)">{preview.description}</p>
              </div>
              <span className="inline-flex items-center gap-1 rounded-full border border-[color:color-mix(in_srgb,var(--primary)_22%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] px-2 py-1 text-[10px] font-medium text-(--primary)">
                <Sparkles className="h-3 w-3" />
                {t("execution.workflow_model_extracted")}
              </span>
            </div>

            <NamedWorkGraphSketch
              className="min-h-56"
              dependencies={preview.dependencies}
              nodes={preview.nodes}
            />

            <div className="rounded-[10px] border border-(--divider-subtle-color) bg-(--surface-muted-background) px-3 py-2.5">
              <div className="text-[10px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">{t("execution.workflow_reusable_objective")}</div>
              <p className="mt-1 text-xs leading-5 text-(--text-default)">{preview.objective}</p>
            </div>
            {saveState === "scheduled" ? (
              <div className="flex items-start gap-2 rounded-[10px] border border-[color:color-mix(in_srgb,var(--success)_28%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--success)_7%,transparent)] px-3 py-2.5 text-xs text-(--text-default)">
                <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-(--success)" />
                <div>
                  <div className="font-semibold text-(--text-strong)">{t("execution.workflow_scheduled_title")}</div>
                  <div className="mt-0.5 leading-5 text-(--text-muted)">{t("execution.workflow_scheduled_message")}</div>
                </div>
              </div>
            ) : null}
          </UiDialogBody>
          <UiDialogFooter className="justify-between gap-3">
            <span className={`max-w-lg text-xs leading-4 ${saveError ? "text-(--destructive)" : "text-(--text-muted)"}`}>
              {saveError ?? t("execution.workflow_cli_notice")}
            </span>
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
                  <button className={getDialogActionClassName("primary", "compact")} disabled={saveState === "saving"} type="button" onClick={() => void handleSave()}>
                    {saveState === "saving" ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
                    {t(saveState === "saving" ? "execution.workflow_scheduling" : "execution.workflow_save_sketch")}
                  </button>
                </>
              )}
            </div>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
