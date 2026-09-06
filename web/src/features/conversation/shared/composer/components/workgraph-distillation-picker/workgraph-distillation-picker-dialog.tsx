/**
 * INPUT: 当前 owner 的已保存 WorkGraph 与版本化内置模板目录。
 * OUTPUT: 具搜索焦点、键盘选项导航和可读命令/预览的工作图选择器；复用仅写入原始 Slash。
 * POS: Composer 能力菜单中的工作图入口；只组合工作图内容，不拥有选项 DOM。
 */
"use client";

import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
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
import { UiPanel } from "@/shared/ui/panel";
import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";

export function WorkGraphDistillationPickerDialog({
  isOpen,
  onClose,
  onUseCommand,
}: {
  isOpen: boolean;
  onClose: () => void;
  onUseCommand: (command: string) => void;
}) {
  if (!isOpen) {
    return null;
  }
  return (
    <OpenWorkGraphDistillationPickerDialog
      onClose={onClose}
      onUseCommand={onUseCommand}
    />
  );
}

function OpenWorkGraphDistillationPickerDialog({
  onClose,
  onUseCommand,
}: {
  onClose: () => void;
  onUseCommand: (command: string) => void;
}) {
  const { locale, t } = useI18n();
  const [items, setItems] = useState<WorkGraphWorkflow[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState<ResourceFailure | null>(null);
  const [loadedLocale, setLoadedLocale] = useState<string | null>(null);
  const [loadRevision, setLoadRevision] = useState(0);
  const searchInputRef = useRef<HTMLInputElement>(null);

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
  const moveOptionFocus = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    if (event.altKey || event.ctrlKey || event.metaKey) return;
    const nextIndex = { ArrowDown: Math.min(index + 1, filtered.length - 1), ArrowUp: Math.max(index - 1, 0),
      Home: 0, End: filtered.length - 1 }[event.key];
    if (nextIndex === undefined) return;
    event.preventDefault();
    setSelectedId(filtered[nextIndex].id);
    event.currentTarget.parentElement?.querySelectorAll<HTMLButtonElement>('[role="option"]')[nextIndex]?.focus();
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop initialFocusRef={searchInputRef} onClose={onClose}>
        <UiDialogShell size="lg" viewport="compact">
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={t("composer.workgraph_picker_title")}
          />
          <UiDialogBody className="flex min-h-0 flex-1 flex-col gap-3">
            <div className="flex shrink-0 items-center gap-2">
              <UiSearchInput
                ref={searchInputRef}
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
                size="sm"
                state="loading"
                title={t("capability.workgraph_loading")}
                variant="plain"
              />
            ) : failure && (failure.access || !hasSnapshot) ? (
              <UiResourceState
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
              <UiPanel className="grid min-h-0 flex-1 overflow-hidden md:grid-cols-[minmax(0,0.82fr)_minmax(0,1.18fr)]" padding="none" radius="sm">
                <div
                  aria-label={t("composer.workgraph_picker_title")}
                  className="soft-scrollbar min-h-0 divide-y divide-(--divider-subtle-color) overflow-y-auto md:border-r md:border-(--divider-subtle-color)"
                  role="listbox"
                >
                  {filtered.map((item, index) => (
                    <SelectMenuOptionRow
                      active={selected?.id === item.id}
                      className={cn(
                        "flex min-h-12 w-full flex-col items-stretch justify-center px-3 py-2 text-left",
                        getSelectMenuOptionStateClassName(selected?.id === item.id),
                      )}
                      key={item.id}
                      onClick={() => setSelectedId(item.id)}
                      onKeyDown={(event) => moveOptionFocus(event, index)}
                      tabIndex={selected?.id === item.id ? 0 : -1}
                    >
                      <div className={`truncate ${getUiTypographyClassName({
                        role: "sectionTitle",
                        tone: "strong",
                      })}`}>{item.title}</div>
                      <div className={`mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 ${getUiTypographyClassName({
                        role: "metadata",
                        tone: "muted",
                      })}`}>
                        <code className="max-w-full break-words [overflow-wrap:anywhere]">/{item.slash_name}</code>
                        {item.built_in ? <span>{t("capability.workgraph_builtin")}</span> : null}
                      </div>
                    </SelectMenuOptionRow>
                  ))}
                </div>
                {selected ? (
                  <div className="soft-scrollbar min-h-0 overflow-y-auto border-t border-(--divider-subtle-color) bg-(--surface-panel-background) p-4 md:border-t-0">
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <h3 className={cn("min-w-0 flex-1 break-words [overflow-wrap:anywhere]", getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }))}>{selected.title}</h3>
                      <UiButton
                        onClick={() => { onUseCommand(`/${selected.slash_name} `); onClose(); }}
                        size="sm"
                        tone="primary"
                        variant="solid"
                      >
                        {t("composer.use_workgraph_command")}
                      </UiButton>
                    </div>
                    {selected.description ? (
                      <p className={cn("mt-1 break-words [overflow-wrap:anywhere]", getUiTypographyClassName({ role: "supporting", tone: "muted" }))}>{selected.description}</p>
                    ) : null}
                    <NamedWorkGraphSketch
                      className="mt-4"
                      dependencies={selected.dependencies}
                      nodes={selected.nodes}
                    />
                  </div>
                ) : null}
              </UiPanel>
            )}
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
