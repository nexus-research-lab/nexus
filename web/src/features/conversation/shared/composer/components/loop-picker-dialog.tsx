"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Repeat2 } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { listLoopsApi } from "@/lib/api/loop-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiButton } from "@/shared/ui/button";
import { UiSearchInput } from "@/shared/ui/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { LoopCatalogItem } from "@/types/capability/loop";

const ALL_CATEGORIES = "__all__";

interface LoopPickerDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onSelect: (loop: LoopCatalogItem) => void | Promise<void>;
}

interface LoopPickerState {
  error: string | null;
  loading: boolean;
  loops: LoopCatalogItem[];
}

function matchesLoop(loop: LoopCatalogItem, query: string): boolean {
  if (!query) {
    return true;
  }
  return [
    loop.title,
    loop.description,
    loop.category,
    loop.trigger_type,
    ...loop.tags,
    ...loop.compatible_agents,
  ].join(" ").toLowerCase().includes(query);
}

export function LoopPickerDialog({
  isOpen,
  onClose,
  onSelect,
}: LoopPickerDialogProps) {
  const { locale, t } = useI18n();
  const [loopState, setLoopState] = useResettableState<LoopPickerState>(
    { error: null, loading: isOpen, loops: [] },
    `${isOpen ? "open" : "closed"}\x1f${locale}`,
  );
  const { error, loading, loops } = loopState;
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState(ALL_CATEGORIES);
  const [busySlug, setBusySlug] = useState<string | null>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen) {
      searchInputRef.current?.focus();
    }
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    let cancelled = false;
    listLoopsApi(locale)
      .then((items) => {
        if (!cancelled) {
          setLoopState({ error: null, loading: false, loops: items });
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setLoopState({
            error: err instanceof Error ? err.message : t("composer.loop_picker_failed"),
            loading: false,
            loops: [],
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, locale, setLoopState, t]);

  const categoryOptions = useMemo(() => {
    const categories = Array.from(new Set(loops.map((loop) => loop.category))).sort();
    return [
      { value: ALL_CATEGORIES, label: t("capability.category_all") },
      ...categories.map((item) => ({ value: item, label: item })),
    ];
  }, [loops, t]);

  const filteredLoops = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return loops.filter((loop) =>
      (category === ALL_CATEGORIES || loop.category === category) &&
      matchesLoop(loop, normalizedQuery),
    );
  }, [category, loops, query]);

  const handleSelect = useCallback(async (loop: LoopCatalogItem) => {
    setBusySlug(loop.slug);
    setLoopState((current) => ({ ...current, error: null }));
    try {
      await onSelect(loop);
      onClose();
    } catch (err) {
      setLoopState((current) => ({
        ...current,
        error: err instanceof Error ? err.message : t("composer.loop_picker_failed"),
      }));
    } finally {
      setBusySlug(null);
    }
  }, [onClose, onSelect, setLoopState, t]);

  if (!isOpen) {
    return null;
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop onClose={onClose}>
        <UiDialogShell
          size="lg"
          style={{ maxHeight: "min(640px, calc(100vh - 96px))" }}
        >
          <UiDialogHeader
            icon={<Repeat2 className="h-4 w-4" />}
            onClose={onClose}
            subtitle={t("composer.loop_picker_subtitle")}
            title={t("composer.loop_picker_title")}
          />
          <UiDialogBody className="flex min-h-0 flex-1 flex-col gap-3">
            <div className="flex shrink-0 flex-col gap-2 sm:flex-row">
              <UiSearchInput
                aria-label={t("composer.loop_search_placeholder")}
                className="min-w-0 flex-1"
                inputClassName="text-[13px]"
                ref={searchInputRef}
                onChange={setQuery}
                placeholder={t("composer.loop_search_placeholder")}
                value={query}
              />
              <UiSelectMenu
                ariaLabel={t("capability.loops_filter_aria")}
                className="sm:w-[180px]"
                onChange={setCategory}
                options={categoryOptions}
                size="sm"
                surface="dialog"
                value={category}
              />
            </div>

            {loading ? (
              <div className="py-10 text-center text-[13px] text-(--text-muted)">
                {t("composer.loop_picker_loading")}
              </div>
            ) : error ? (
              <div className="py-10 text-center text-[13px] text-(--destructive)">
                {error}
              </div>
            ) : filteredLoops.length === 0 ? (
              <div className="py-10 text-center text-[13px] text-(--text-muted)">
                {t("composer.loop_picker_empty")}
              </div>
            ) : (
              <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto pr-1">
                <div className="grid grid-cols-1 gap-2">
                {filteredLoops.map((loop) => (
                  <div
                    className="rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-raised-background) p-3 transition-colors hover:bg-(--surface-interactive-hover-background)"
                    key={loop.slug}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <button
                        disabled={busySlug !== null}
                        className="min-w-0 flex-1 text-left"
                        onClick={() => void handleSelect(loop)}
                        type="button"
                      >
                        <div className="flex flex-wrap items-center gap-1.5">
                          <span className="rounded-[6px] bg-(--surface-interactive-hover-background) px-2 py-0.5 text-[11px] text-(--text-soft)">
                            {loop.category}
                          </span>
                          <span className="rounded-[6px] bg-(--surface-interactive-hover-background) px-2 py-0.5 text-[11px] text-(--text-soft)">
                            {loop.trigger_type}
                          </span>
                        </div>
                        <div className="mt-2 text-[14px] font-semibold text-(--text-strong)">
                          {loop.title}
                        </div>
                        <p className="mt-1 line-clamp-2 text-[12px] leading-5 text-(--text-muted)">
                          {loop.description}
                        </p>
                      </button>
                      <div className="mt-1 flex shrink-0 flex-col gap-1.5">
                        <UiButton
                          disabled={busySlug !== null}
                          onClick={() => void handleSelect(loop)}
                          size="xs"
                          tone="primary"
                          variant="solid"
                        >
                          {busySlug === loop.slug ? t("composer.loop_starting") : t("composer.use_loop")}
                        </UiButton>
                      </div>
                    </div>
                  </div>
                ))}
                </div>
              </div>
            )}
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
