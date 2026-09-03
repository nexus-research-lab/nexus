/**
 * INPUT: Loop 目录、筛选、复制动作与可选详情路由。
 * OUTPUT: 展示用途、触发方式与步骤规模的工作循环目录或当前 Loop 详情。
 * POS: “能力 > 工作循环”的唯一页面入口。
 */
"use client";

import { useEffect, useMemo, useState } from "react";
import { Check, Copy, Repeat2, RotateCcw } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import {
  CAPABILITY_DIRECTORY_GRID_CLASS_NAME,
  CAPABILITY_DIRECTORY_ROW_CLASS_NAME,
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
  CapabilityPageLayout,
} from "@/features/capability/shared/capability-page-layout";
import { listLoopsApi } from "@/lib/api/capability/loop-api";
import { getResourceFailure, type ResourceFailure } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiListRow } from "@/shared/ui/list/list-row";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { LoopCatalogItem } from "@/types/capability/loop";
import { writeTextToClipboard } from "@/hooks/ui/clipboard";

import { LoopDetailView } from "./loop-detail-view";
import { getLoopTriggerLabel } from "./loop-presentation";

const ALL_CATEGORIES = "__all__";

function matchesLoop(loop: LoopCatalogItem, query: string): boolean {
  if (!query) {
    return true;
  }
  const haystack = [
    loop.title,
    loop.description,
    loop.category,
    loop.trigger_type,
    ...loop.tags,
    ...loop.compatible_agents,
  ].join(" ").toLowerCase();
  return haystack.includes(query);
}

export function LoopsDirectory() {
  const { locale, t } = useI18n();
  const navigate = useNavigate();
  const { slug } = useParams<{ slug?: string }>();
  const [loops, setLoops] = useState<LoopCatalogItem[]>([]);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState(ALL_CATEGORIES);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ResourceFailure | null>(null);
  const [loadedLocale, setLoadedLocale] = useState<string | null>(null);
  const [loadRevision, setLoadRevision] = useState(0);
  const [copiedSlug, setCopiedSlug] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError((current) => current?.access ? current : null);
    listLoopsApi(locale)
      .then((items) => {
        if (!cancelled) {
          setLoops(items);
          setLoadedLocale(locale);
          setError(null);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(getResourceFailure(err, t("capability.loops_loading_failed")));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [loadRevision, locale, t]);

  const retryLoad = () => setLoadRevision((current) => current + 1);
  const clearFilters = () => {
    setQuery("");
    setCategory(ALL_CATEGORIES);
  };

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
  const hasSnapshot = loadedLocale === locale;

  const copyPrompt = async (loop: LoopCatalogItem) => {
    await writeTextToClipboard(loop.kickoff_prompt);
    setCopiedSlug(loop.slug);
    window.setTimeout(() => setCopiedSlug((current) => current === loop.slug ? null : current), 1800);
  };

  return (
    <WorkspaceSurfaceScaffold
      bodyScrollable
      stableGutter
    >
      {slug ? (
        <LoopDetailView
          slug={slug}
          onBack={() => navigate(AppRouteBuilders.loops())}
        />
      ) : (
        <CapabilityPageLayout
          description={t("capability.loops_intro_description")}
          title={t("capability.loops_intro_title")}
        >
          <CapabilityFilterBar>
            <CapabilityFilterSearchInput
              onChange={setQuery}
              placeholder={t("capability.loops_search_placeholder")}
              value={query}
            />
            <CapabilityFilterSelect
              ariaLabel={t("capability.loops_filter_aria")}
              label={t("capability.category_label")}
              onChange={setCategory}
              options={categoryOptions}
              value={category}
            />
          </CapabilityFilterBar>

          {loading && !hasSnapshot ? (
            <UiResourceState
              className="min-h-48"
              size="sm"
              state="loading"
              title={t("capability.loops_loading")}
            />
          ) : error && (error.access || !hasSnapshot) ? (
            <UiResourceState
              className="min-h-48"
              impact={t(error.access
                ? "state.access_failure_impact"
                : "state.read_failure_impact")}
              primaryAction={{
                icon: <RotateCcw className="h-3.5 w-3.5" />,
                label: t("state.retry"),
                onClick: retryLoad,
              }}
              size="sm"
              state="error"
              title={t(error.access
                ? "state.permission_title"
                : "capability.loops_loading_failed")}
            />
          ) : (
            <>
              {error ? (
                <UiResourceState
                  className="mb-3 min-h-0 py-3"
                  impact={t("state.stale_snapshot_impact")}
                  primaryAction={{
                    icon: <RotateCcw className="h-3.5 w-3.5" />,
                    label: t("state.retry"),
                    onClick: retryLoad,
                  }}
                  role="status"
                  size="sm"
                  state="error"
                  title={t("capability.loops_loading_failed")}
                />
              ) : null}
              {filteredLoops.length === 0 ? (
                <UiResourceState
                  className="min-h-48"
                  description={t("capability.loops_empty_description")}
                  icon={<Repeat2 className="h-5 w-5 text-(--icon-default)" />}
                  impact={loops.length > 0 ? t("state.filter_impact") : undefined}
                  primaryAction={loops.length > 0 ? {
                    label: t("state.clear_filters"),
                    onClick: clearFilters,
                  } : {
                    icon: <RotateCcw className="h-3.5 w-3.5" />,
                    label: t("state.retry"),
                    onClick: retryLoad,
                  }}
                  size="sm"
                  state="empty"
                  title={t("capability.loops_empty")}
                />
              ) : (
                <div
                  aria-busy={loading}
                  className={CAPABILITY_DIRECTORY_GRID_CLASS_NAME}
                >
                  {filteredLoops.map((loop) => (
                    <UiListRow
                      className={CAPABILITY_DIRECTORY_ROW_CLASS_NAME}
                      key={loop.slug}
                      onClick={() => navigate(AppRouteBuilders.loopDetail(loop.slug))}
                      leading={<UiSeededAvatar seed={loop.slug} size="sm" />}
                      right={(
                        <UiIconButton
                          aria-label={t("capability.loops_copy_prompt")}
                          className="shrink-0"
                          onClick={(event) => {
                            event.stopPropagation();
                            void copyPrompt(loop);
                          }}
                          size="md"
                          variant="ghost"
                        >
                          {copiedSlug === loop.slug
                            ? <Check className="h-4 w-4" />
                            : <Copy className="h-4 w-4" />}
                        </UiIconButton>
                      )}
                    >
                      <div className="min-w-0 flex-1">
                        <h3 className={cn(
                          "truncate",
                          getUiTypographyClassName({ role: "control", tone: "strong", weight: "medium" }),
                        )}>
                          {loop.title}
                        </h3>
                        <p className={cn(
                          "mt-0.5 truncate",
                          getUiTypographyClassName({ role: "metadata", tone: "muted" }),
                        )}>
                          {loop.description}
                        </p>
                        <div className={cn(
                          "mt-0.5 flex min-w-0 items-center gap-1.5 overflow-hidden",
                          getUiTypographyClassName({ role: "caption", tone: "soft" }),
                        )}>
                          <span className="truncate">{loop.category}</span>
                          <span aria-hidden="true">·</span>
                          <span className="shrink-0">
                            {getLoopTriggerLabel(loop.trigger_type, t)}
                          </span>
                          <span aria-hidden="true">·</span>
                          <span className="shrink-0">
                            {t("capability.loops_step_count", {
                              count: loop.steps.length,
                            })}
                          </span>
                        </div>
                      </div>
                    </UiListRow>
                  ))}
                </div>
              )}
            </>
          )}
        </CapabilityPageLayout>
      )}
    </WorkspaceSurfaceScaffold>
  );
}
