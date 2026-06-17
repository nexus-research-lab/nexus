"use client";

import { useEffect, useMemo, useState } from "react";
import { Repeat2 } from "lucide-react";

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
  on_select: (loop: LoopCatalogItem) => void;
  on_select_goal?: (loop: LoopCatalogItem) => void;
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
  on_select_goal,
}: LoopPickerDialogProps) {
  const { locale, t } = useI18n();
  const [loops, set_loops] = useState<LoopCatalogItem[]>([]);
  const [query, set_query] = useState("");
  const [category, set_category] = useState(ALL_CATEGORIES);
  const [loading, set_loading] = useState(false);
  const [error, set_error] = useState<string | null>(null);

  useEffect(() => {
    if (!is_open) {
      return;
    }
    let cancelled = false;
    set_loading(true);
    set_error(null);
    list_loops_api(locale)
      .then((items) => {
        if (!cancelled) {
          set_loops(items);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          set_error(err instanceof Error ? err.message : t("composer.loop_picker_failed"));
        }
      })
      .finally(() => {
        if (!cancelled) {
          set_loading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [is_open, locale, t]);

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
                autoFocus
                class_name="min-w-0 flex-1"
                input_class_name="text-[13px]"
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
                        className="min-w-0 flex-1 text-left"
                        onClick={() => {
                          on_select(loop);
                          on_close();
                        }}
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
                          onClick={() => {
                            on_select(loop);
                            on_close();
                          }}
                          size="xs"
                          tone="primary"
                          variant="solid"
                        >
                          {t("composer.use_loop")}
                        </UiButton>
                        {on_select_goal ? (
                          <UiButton
                            onClick={() => {
                              on_select_goal(loop);
                              on_close();
                            }}
                            size="xs"
                            variant="surface"
                          >
                            {t("composer.use_loop_as_goal")}
                          </UiButton>
                        ) : null}
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
