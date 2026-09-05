/**
 * INPUT: owner-scoped 已固定保存的 WorkGraph 草图目录。
 * OUTPUT: 与 Loop picker 同层级、具共享 listbox 选项语义的只读查看与命令复用入口。
 * POS: Composer 能力菜单中的工作图入口；只组合工作图内容，不拥有选项 DOM。
 */
"use client";

import { useEffect, useMemo, useState } from "react";
import { RotateCcw } from "lucide-react";

import { NamedWorkGraphSketch } from "@/features/conversation/shared/execution/named-workgraph-sketch";
import { getWorkGraphWorkflowsApi } from "@/lib/api/conversation/execution-api";
import { WORKGRAPH_WORKFLOWS_CHANGED_EVENT } from "@/lib/conversation/workgraph-workflow-events";
import { getResourceFailure, type ResourceFailure } from "@/lib/error-message";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import { getSelectMenuOptionStateClassName } from "@/shared/ui/menu/select-menu-styles";
import { SelectMenuOptionRow } from "@/shared/ui/menu/select-menu-primitives";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";

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
  const [failure, setFailure] = useState<ResourceFailure | null>(null);
  const [loadedLocale, setLoadedLocale] = useState<string | null>(null);
  const [loadRevision, setLoadRevision] = useState(0);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setFailure((current) => current?.access ? current : null);
    getWorkGraphWorkflowsApi(locale).then((nextItems) => {
      if (!active) return;
      setItems(nextItems);
      setLoadedLocale(locale);
      setSelectedId(nextItems[0]?.id ?? null);
      setFailure(null);
      window.dispatchEvent(new CustomEvent(WORKGRAPH_WORKFLOWS_CHANGED_EVENT));
    }).catch((reason: unknown) => {
      if (active) setFailure(getResourceFailure(reason, t("composer.workgraph_picker_failed")));
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; };
  }, [loadRevision, locale, t]);
  const hasSnapshot = loadedLocale === locale;

  const filtered = useMemo(() => {
    const search = createUiSearchMatcher(query);
    return items.filter((item) => search.matches([
      item.slash_name,
      item.title,
      item.description,
      item.objective,
    ]));
  }, [items, query]);
  const selected = filtered.find((item) => item.id === selectedId) ?? filtered[0] ?? null;

  return (
    <UiDialogPortal>
      <UiDialogBackdrop onClose={onClose}>
        <UiDialogShell size="lg" viewport="compact">
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
            {failure && hasSnapshot && !failure.access ? (
              <UiResourceState
                className="min-h-0 py-3"
                impact={t("state.stale_snapshot_impact")}
                primaryAction={{
                  icon: <RotateCcw className="h-3.5 w-3.5" />,
                  label: t("state.retry"),
                  onClick: () => setLoadRevision((current) => current + 1),
                }}
                role="status"
                size="sm"
                state="error"
                title={t("composer.workgraph_picker_failed")}
                variant="plain"
              />
            ) : null}
            {loading && !hasSnapshot ? (
              <UiResourceState
                className="min-h-48"
                size="sm"
                state="loading"
                title={t("capability.workgraph_loading")}
                variant="plain"
              />
            ) : failure && (failure.access || !hasSnapshot) ? (
              <UiResourceState
                className="min-h-48"
                impact={t(failure.access
                  ? "state.access_failure_impact"
                  : "state.read_failure_impact")}
                primaryAction={{
                  icon: <RotateCcw className="h-3.5 w-3.5" />,
                  label: t("state.retry"),
                  onClick: () => setLoadRevision((current) => current + 1),
                }}
                size="sm"
                state="error"
                title={t(failure.access
                  ? "state.permission_title"
                  : "composer.workgraph_picker_failed")}
                variant="plain"
              />
            ) : filtered.length === 0 ? (
              <UiResourceState
                className="min-h-48"
                impact={items.length > 0 ? t("state.filter_impact") : undefined}
                {...(items.length > 0
                  ? {
                      primaryAction: {
                        label: t("state.clear_filters"),
                        onClick: () => setQuery(""),
                      },
                    }
                  : { nextStep: t("capability.workgraph_empty_description") })}
                size="sm"
                state="empty"
                title={t("composer.workgraph_picker_empty")}
                variant="plain"
              />
            ) : (
              <div className="grid min-h-0 flex-1 overflow-hidden rounded-[10px] border border-(--divider-subtle-color) md:grid-cols-[minmax(0,0.82fr)_minmax(0,1.18fr)]">
                <div
                  aria-label={t("composer.workgraph_picker_title")}
                  className="soft-scrollbar min-h-0 divide-y divide-(--divider-subtle-color) overflow-y-auto md:border-r md:border-(--divider-subtle-color)"
                  role="listbox"
                >
                  {filtered.map((item) => (
                    <SelectMenuOptionRow
                      active={selected?.id === item.id}
                      className={cn(
                        "flex min-h-12 w-full flex-col items-stretch justify-center px-3 py-2 text-left",
                        getSelectMenuOptionStateClassName(selected?.id === item.id),
                      )}
                      key={item.id}
                      onClick={() => setSelectedId(item.id)}
                    >
                      <div className={`truncate ${getUiTypographyClassName({
                        role: "sectionTitle",
                        tone: "strong",
                      })}`}>{item.title}</div>
                      <div className={`mt-0.5 flex items-center gap-2 ${getUiTypographyClassName({
                        role: "caption",
                        tone: "soft",
                      })}`}>
                        <code>/{item.slash_name}</code>
                        {item.built_in ? <span>{t("capability.workgraph_builtin")}</span> : null}
                      </div>
                    </SelectMenuOptionRow>
                  ))}
                </div>
                {selected ? (
                  <div className="soft-scrollbar min-h-0 overflow-y-auto border-t border-(--divider-subtle-color) bg-(--surface-panel-background) p-4 md:border-t-0">
                    <div className="flex items-center justify-between gap-3">
                      <h3 className="min-w-0 flex-1 text-sm font-semibold text-(--text-strong)">{selected.title}</h3>
                      <UiButton
                        onClick={() => { onUseCommand(`/${selected.slash_name} `); onClose(); }}
                        size="xs"
                        tone="primary"
                        variant="solid"
                      >
                        {t("composer.use_workgraph_command")}
                      </UiButton>
                    </div>
                    {selected.description ? (
                      <p className="mt-1 text-compact leading-5 text-(--text-muted)">{selected.description}</p>
                    ) : null}
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
