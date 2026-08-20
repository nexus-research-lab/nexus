/**
 * INPUT: exact ExecutionView 与用户命名/节点角色选择。
 * OUTPUT: 写入当前 Session Composer 的可见沉淀请求。
 * POS: 历史图 UI 到 execution-orchestrator Skill + Nexus CLI 的确认层；不直写 Workflow。
 */
"use client";

import { type FormEvent, useMemo, useRef, useState } from "react";
import { GitBranchPlus } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { getDialogActionClassName } from "@/shared/ui/dialog/dialog-styles";
import { UiField, UiInput, UiTextarea } from "@/shared/ui/form/form-control";
import type { ExecutionView } from "@/types/conversation/execution";
import type { WorkGraphWorkflowNodeRole } from "@/types/conversation/workgraph-workflow";

import { dispatchWorkGraphDistillationIntent } from "./workgraph-distillation-intent";
import {
  buildDistillationPrompt,
  normalizeWorkGraphWorkflowSlashName,
  type WorkGraphDistillationSelection,
} from "./workgraph-distillation-model";

export function WorkGraphDistillationDialog({
  execution,
  onClose,
  sessionKey,
}: {
  execution: ExecutionView;
  onClose: () => void;
  sessionKey: string;
}) {
  const { t } = useI18n();
  const nameRef = useRef<HTMLInputElement | null>(null);
  const items = useMemo(() => execution.work_items ?? [], [execution.work_items]);
  const [slashName, setSlashName] = useState("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [selections, setSelections] = useState<Map<string, WorkGraphDistillationSelection>>(
    () => new Map(items.map((item) => [item.id, { enabled: true, role: "key" }])),
  );
  const selectedCount = Array.from(selections.values())
    .filter((selection) => selection.enabled).length;
  const normalizedName = normalizeWorkGraphWorkflowSlashName(slashName);
  const canSubmit = normalizedName.length > 0 && title.trim().length > 0
    && selectedCount > 0;

  const updateSelection = (
    itemId: string,
    update: (
      current: WorkGraphDistillationSelection,
    ) => WorkGraphDistillationSelection,
  ) => {
    setSelections((current) => {
      const next = new Map(current);
      next.set(itemId, update(next.get(itemId) ?? { enabled: true, role: "key" }));
      return next;
    });
  };

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    if (!canSubmit) {
      return;
    }
    dispatchWorkGraphDistillationIntent({
      sessionKey,
      prompt: buildDistillationPrompt({
        description,
        execution,
        selections,
        slashName: normalizedName,
        title: title.trim(),
      }),
    });
    onClose();
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9998]"
        initialFocusRef={nameRef}
        labelledBy="workgraph-distillation-dialog-title"
        onClose={onClose}
      >
        <UiDialogFormShell className="pointer-events-auto max-h-[86vh]" size="lg" onSubmit={handleSubmit}>
          <UiDialogHeader
            icon={<GitBranchPlus className="h-4 w-4" />}
            iconClassName="text-(--primary)"
            onClose={onClose}
            subtitle={t("execution.workflow_distill_subtitle")}
            title={t("execution.workflow_distill_title")}
            titleId="workgraph-distillation-dialog-title"
          />
          <UiDialogBody className="flex flex-col gap-4" scrollable>
            <div className="grid gap-3 sm:grid-cols-2">
              <UiField htmlFor="workgraph-workflow-name" label={t("execution.workflow_command_name")}>
                <div className="relative">
                  <span className="pointer-events-none absolute inset-y-0 left-3 flex items-center font-mono text-sm text-(--text-soft)">/</span>
                  <UiInput
                    ref={nameRef}
                    className="pl-6 font-mono"
                    data-autofocus="true"
                    id="workgraph-workflow-name"
                    placeholder="deep-research"
                    value={slashName}
                    variant="dialog"
                    onChange={(event) => setSlashName(event.target.value)}
                    onBlur={() => setSlashName(normalizedName)}
                  />
                </div>
              </UiField>
              <UiField htmlFor="workgraph-workflow-title" label={t("execution.workflow_title_label")}>
                <UiInput
                  id="workgraph-workflow-title"
                  placeholder={t("execution.workflow_title_placeholder")}
                  value={title}
                  variant="dialog"
                  onChange={(event) => setTitle(event.target.value)}
                />
              </UiField>
            </div>
            <UiField htmlFor="workgraph-workflow-description" label={t("execution.workflow_description_label")}>
              <UiTextarea
                className="min-h-20"
                id="workgraph-workflow-description"
                placeholder={t("execution.workflow_description_placeholder")}
                value={description}
                variant="dialog"
                onChange={(event) => setDescription(event.target.value)}
              />
            </UiField>

            <div className="rounded-[10px] border border-[color:color-mix(in_srgb,var(--primary)_20%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_6%,transparent)] px-3 py-2 text-xs leading-5 text-(--text-soft)">
              {t("execution.workflow_cli_notice")}
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between gap-3">
                <span className="text-xs font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
                  {t("execution.workflow_nodes_label")}
                </span>
                <span className="font-mono text-[11px] text-(--text-muted)">
                  {selectedCount}/{items.length}
                </span>
              </div>
              {items.map((item) => {
                const selection = selections.get(item.id) ?? { enabled: true, role: "key" as const };
                return (
                  <div
                    key={item.id}
                    className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 rounded-[9px] border dialog-divider bg-(--surface-muted-background) px-3 py-2.5"
                  >
                    <input
                      checked={selection.enabled}
                      className="h-4 w-4 accent-(--primary)"
                      type="checkbox"
                      onChange={(event) => updateSelection(item.id, (current) => ({
                        ...current,
                        enabled: event.target.checked,
                      }))}
                    />
                    <div className="min-w-0">
                      <div className="truncate text-sm font-medium text-(--text-strong)">{item.subject}</div>
                      <div className="truncate font-mono text-[10px] text-(--text-muted)">{item.logical_key}</div>
                    </div>
                    <select
                      aria-label={t("execution.workflow_node_role")}
                      className="h-8 rounded-[7px] border dialog-divider bg-(--surface-control-background) px-2 text-xs text-(--text-default)"
                      disabled={!selection.enabled}
                      value={selection.role}
                      onChange={(event) => updateSelection(item.id, (current) => ({
                        ...current,
                        role: event.target.value as WorkGraphWorkflowNodeRole,
                      }))}
                    >
                      <option value="key">{t("execution.workflow_role_key")}</option>
                      <option value="collaboration">{t("execution.workflow_role_collaboration")}</option>
                    </select>
                  </div>
                );
              })}
            </div>
          </UiDialogBody>
          <UiDialogFooter className="justify-between gap-3">
            <span className="text-xs text-(--text-muted)">{t("execution.workflow_prompt_notice")}</span>
            <div className="flex gap-2">
              <button className={getDialogActionClassName("default", "compact")} type="button" onClick={onClose}>
                {t("common.cancel")}
              </button>
              <button className={getDialogActionClassName("primary", "compact")} disabled={!canSubmit} type="submit">
                {t("execution.workflow_send_to_agent")}
              </button>
            </div>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
