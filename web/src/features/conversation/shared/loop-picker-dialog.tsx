"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Repeat2 } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { list_loops_api } from "@/lib/api/loop-api";
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
import { UiSelectMenu } from "@/shared/ui/select-menu";
import type { LoopCatalogItem } from "@/types/capability/loop";

const ALL_CATEGORIES = "__all__";

interface LoopPickerDialogProps {
  is_open: boolean;
  on_close: () => void;
  on_select: (loop: LoopCatalogItem) => void | Promise<void>;
}

interface LoopPickerState {
  error: string | null;
  loading: boolean;
  loops: LoopCatalogItem[];
}

function matches_loop(loop: LoopCatalogItem, query: string): boolean {
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
  is_open,
  on_close,
  on_select,
}: LoopPickerDialogProps) {
  const { locale, t } = useI18n();
  const [loop_state, set_loop_state] = useResettableState<LoopPickerState>(
    { error: null, loading: is_open, loops: [] },
    `${is_open ? "open" : "closed"}\x1f${locale}`,
  );
  const { error, loading, loops } = loop_state;
  const [query, set_query] = useState("");
  const [category, set_category] = useState(ALL_CATEGORIES);
  const [busy_slug, set_busy_slug] = useState<string | null>(null);
  const search_input_ref = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (is_open) {
      search_input_ref.current?.focus();
    }
  }, [is_open]);

  useEffect(() => {
    if (!is_open) {
      return;
    }
    let cancelled = false;
    list_loops_api(locale)
      .then((items) => {
        if (!cancelled) {
          set_loop_state({ error: null, loading: false, loops: items });
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          set_loop_state({
            error: err instanceof Error ? err.message : t("composer.loop_picker_failed"),
            loading: false,
            loops: [],
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [is_open, locale, set_loop_state, t]);

  const category_options = useMemo(() => {
    const categories = Array.from(new Set(loops.map((loop) => loop.category))).sort();
    return [
      { value: ALL_CATEGORIES, label: t("capability.category_all") },
      ...categories.map((item) => ({ value: item, label: item })),
    ];
  }, [loops, t]);

  const filtered_loops = useMemo(() => {
    const normalized_query = query.trim().toLowerCase();
    return loops.filter((loop) =>
      (category === ALL_CATEGORIES || loop.category === category) &&
      matches_loop(loop, normalized_query),
    );
  }, [category, loops, query]);

  const handle_select = useCallback(async (loop: LoopCatalogItem) => {
    set_busy_slug(loop.slug);
    set_loop_state((current) => ({ ...current, error: null }));
    try {
      await on_select(loop);
      on_close();
    } catch (err) {
      set_loop_state((current) => ({
        ...current,
        error: err instanceof Error ? err.message : t("composer.loop_picker_failed"),
      }));
    } finally {
      set_busy_slug(null);
    }
  }, [on_close, on_select, set_loop_state, t]);

  if (!is_open) {
    return null;
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop on_close={on_close}>
        <UiDialogShell
          size="lg"
          style={{ maxHeight: "min(640px, calc(100vh - 96px))" }}
        >
          <UiDialogHeader
            icon={<Repeat2 className="h-4 w-4" />}
            on_close={on_close}
            subtitle={t("composer.loop_picker_subtitle")}
            title={t("composer.loop_picker_title")}
          />
          <UiDialogBody class_name="flex min-h-0 flex-1 flex-col gap-3">
            <div className="flex shrink-0 flex-col gap-2 sm:flex-row">
              <UiSearchInput
                aria-label={t("composer.loop_search_placeholder")}
                class_name="min-w-0 flex-1"
                input_class_name="text-[13px]"
                ref={search_input_ref}
                on_change={set_query}
                placeholder={t("composer.loop_search_placeholder")}
                value={query}
              />
              <UiSelectMenu
                aria_label={t("capability.loops_filter_aria")}
                class_name="sm:w-[180px]"
                on_change={set_category}
                options={category_options}
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
            ) : filtered_loops.length === 0 ? (
              <div className="py-10 text-center text-[13px] text-(--text-muted)">
                {t("composer.loop_picker_empty")}
              </div>
            ) : (
              <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto pr-1">
                <div className="grid grid-cols-1 gap-2">
                {filtered_loops.map((loop) => (
                  <div
                    className="rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-raised-background) p-3 transition-colors hover:bg-(--surface-interactive-hover-background)"
                    key={loop.slug}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <button
                        disabled={busy_slug !== null}
                        className="min-w-0 flex-1 text-left"
                        onClick={() => void handle_select(loop)}
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
                          disabled={busy_slug !== null}
                          onClick={() => void handle_select(loop)}
                          size="xs"
                          tone="primary"
                          variant="solid"
                        >
                          {busy_slug === loop.slug ? t("composer.loop_starting") : t("composer.use_loop")}
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
