/**
 * INPUT: owner-scoped 已固定保存的 WorkGraph 草图目录。
 * OUTPUT: 与 Loop picker 同层级的只读查看与命令复用入口。
 * POS: Composer 能力菜单中的工作图入口；不展示运行图，也不发起草图保存。
 */
"use client";

import { useEffect, useMemo, useState } from "react";
import { LoaderCircle } from "lucide-react";

import { NamedWorkGraphSketch } from "@/features/conversation/shared/execution/named-workgraph-sketch";
import { getWorkGraphWorkflowsApi } from "@/lib/api/conversation/execution-api";
import { getErrorMessage } from "@/lib/error-message";
import { UiButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";

import { WORKGRAPH_WORKFLOWS_CHANGED_EVENT } from "../../../execution/workgraph-distillation-intent";

export function WorkGraphDistillationPickerDialog({
  isOpen,
  onClose,
  onUseCommand,
  sessionKey,
}: {
  isOpen: boolean;
  onClose: () => void;
  onUseCommand: (command: string) => void;
  sessionKey: string;
}) {
  if (!isOpen) {
    return null;
  }
  return (
    <OpenWorkGraphDistillationPickerDialog
      onClose={onClose}
      onUseCommand={onUseCommand}
      sessionKey={sessionKey}
    />
  );
}

function OpenWorkGraphDistillationPickerDialog({
  onClose,
  onUseCommand,
}: {
  onClose: () => void;
  onUseCommand: (command: string) => void;
  sessionKey?: string;
}) {
  const { locale, t } = useI18n();
  const [items, setItems] = useState<WorkGraphWorkflow[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    setLoading(true);
    getWorkGraphWorkflowsApi(locale).then((nextItems) => {
      if (!active) return;
      setItems(nextItems);
      setSelectedId(nextItems[0]?.id ?? null);
      setError(null);
      window.dispatchEvent(new CustomEvent(WORKGRAPH_WORKFLOWS_CHANGED_EVENT));
    }).catch((reason: unknown) => {
      if (active) setError(getErrorMessage(reason, t("composer.workgraph_picker_failed")));
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; };
  }, [locale, t]);

  const filtered = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase();
    if (!normalized) return items;
    return items.filter((item) => [
      item.slash_name,
      item.title,
      item.description ?? "",
      item.objective,
    ].join(" ").toLocaleLowerCase().includes(normalized));
  }, [items, query]);
  const selected = filtered.find((item) => item.id === selectedId) ?? filtered[0] ?? null;

  return (
    <UiDialogPortal>
      <UiDialogBackdrop onClose={onClose}>
        <UiDialogShell size="lg" style={{ maxHeight: "min(680px, calc(100vh - 72px))" }}>
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={t("composer.workgraph_picker_title")}
          />
          <UiDialogBody className="flex min-h-0 flex-1 flex-col gap-3">
            <div className="flex shrink-0 items-center gap-2">
              <UiSearchInput
                aria-label={t("composer.workgraph_search_placeholder")}
                className="min-w-0 flex-1"
                onChange={setQuery}
                placeholder={t("composer.workgraph_search_placeholder")}
                value={query}
              />
            </div>
            {loading ? (
              <div className="grid min-h-48 place-items-center"><LoaderCircle className="h-5 w-5 animate-spin text-(--icon-muted)" /></div>
            ) : error ? (
              <div className="py-10 text-center text-sm text-(--destructive)">{error}</div>
            ) : filtered.length === 0 ? (
              <div className="py-10 text-center text-sm text-(--text-muted)">{t("composer.workgraph_picker_empty")}</div>
            ) : (
              <div className="grid min-h-0 flex-1 overflow-hidden rounded-[10px] border border-(--divider-subtle-color) md:grid-cols-[minmax(0,0.82fr)_minmax(0,1.18fr)]">
                <div className="soft-scrollbar min-h-0 divide-y divide-(--divider-subtle-color) overflow-y-auto md:border-r md:border-(--divider-subtle-color)">
                  {filtered.map((item) => (
                    <button
                      className={`w-full px-3.5 py-3 text-left transition-colors ${selected?.id === item.id ? "bg-(--surface-interactive-hover-background)" : "bg-(--surface-raised-background) hover:bg-(--surface-interactive-hover-background)"}`}
                      key={item.id}
                      onClick={() => setSelectedId(item.id)}
                      type="button"
                    >
                      <div className="truncate text-sm font-medium text-(--text-strong)">{item.title}</div>
                      <div className="mt-1 flex items-center gap-2 text-xs text-(--text-soft)">
                        <code>/{item.slash_name}</code>
                        {item.built_in ? <span>{t("capability.workgraph_builtin")}</span> : null}
                      </div>
                    </button>
                  ))}
                </div>
                {selected ? (
                  <div className="soft-scrollbar min-h-0 overflow-y-auto border-t border-(--divider-subtle-color) bg-(--surface-raised-background) p-4 md:border-t-0">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="min-w-0 flex-1">
                        <h3 className="text-sm font-semibold text-(--text-strong)">{selected.title}</h3>
                        {selected.description ? (
                          <p className="mt-1 text-compact leading-5 text-(--text-muted)">{selected.description}</p>
                        ) : null}
                      </div>
                      <UiButton
                        onClick={() => { onUseCommand(`/${selected.slash_name} `); onClose(); }}
                        size="xs"
                        tone="primary"
                        variant="solid"
                      >
                        {t("composer.use_workgraph_command")}
                      </UiButton>
                    </div>
                    <NamedWorkGraphSketch
                      className="mt-4"
                      dependencies={selected.dependencies}
                      nodes={selected.nodes}
                    />
                  </div>
                ) : null}
              </div>
            )}
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
