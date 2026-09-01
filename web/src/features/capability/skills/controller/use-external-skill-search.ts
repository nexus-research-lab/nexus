import { useCallback, useEffect, useRef, useState } from "react";

import {
  getExternalSkillPreviewApi,
  searchExternalSkillsApi,
} from "@/lib/api/capability/skill-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  ExternalSkillSearchItem,
  ExternalSkillSourceInfo,
  ExternalSkillSourceStatus,
} from "@/types/capability/skill";

import { isExternalSkillPreviewUnavailable } from "../external/external-skill-model";
import type { ExternalSkillSearchController } from "./skill-marketplace-controller";

const MIN_EXTERNAL_SEARCH_LENGTH = 2;

interface UseExternalSkillSearchOptions {
  active: boolean;
  onError: (kind: "preview" | "search", message: string) => void;
  sourceRevision: number;
  sources: ExternalSkillSourceInfo[];
  sourcesLoading: boolean;
}

export function useExternalSkillSearch({
  active,
  onError,
  sourceRevision,
  sources,
  sourcesLoading,
}: UseExternalSkillSearchOptions): ExternalSkillSearchController {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [submittedQuery, setSubmittedQuery] = useState("");
  const [sourceId, setSourceId] = useState("");
  const [searchRevision, setSearchRevision] = useState(0);
  const [results, setResults] = useState<ExternalSkillSearchItem[]>([]);
  const [sourceStatuses, setSourceStatuses] = useState<ExternalSkillSourceStatus[]>([]);
  const [loading, setLoading] = useState(false);
  const [previewItem, setPreviewItem] = useState<ExternalSkillSearchItem | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const searchRequestRef = useRef(0);
  const searchAbortRef = useRef<AbortController | null>(null);
  const previewRequestRef = useRef(0);
  const selectedSourceSearchable = !sourceId || (
    !sourcesLoading
    && sources.some((source) => source.source_id === sourceId && source.enabled)
  );

  const clearResults = useCallback(() => {
    searchAbortRef.current?.abort();
    searchAbortRef.current = null;
    searchRequestRef.current += 1;
    setSubmittedQuery("");
    setLoading(false);
    setResults([]);
    setSourceStatuses([]);
  }, []);

  useEffect(() => {
    if (!active || !submittedQuery || query.trim().length >= MIN_EXTERNAL_SEARCH_LENGTH) return;
    clearResults();
  }, [active, clearResults, query, submittedQuery]);

  useEffect(() => {
    if (!sourceId || sourcesLoading || selectedSourceSearchable) return;
    setSourceId("");
  }, [selectedSourceSearchable, sourceId, sourcesLoading]);

  useEffect(() => {
    if (!active || !selectedSourceSearchable) return;
    const normalizedQuery = submittedQuery.trim();
    if (normalizedQuery.length > 0 && normalizedQuery.length < MIN_EXTERNAL_SEARCH_LENGTH) return;

    const requestId = ++searchRequestRef.current;
    searchAbortRef.current?.abort();
    const abortController = new AbortController();
    searchAbortRef.current = abortController;
    void (async () => {
      try {
        setLoading(true);
        const response = await searchExternalSkillsApi(
          normalizedQuery,
          false,
          sourceId || undefined,
          abortController.signal,
        );
        if (requestId !== searchRequestRef.current) return;
        setResults(response.results);
        setSourceStatuses(response.sources);
      } catch (error) {
        if (abortController.signal.aborted) return;
        if (requestId !== searchRequestRef.current) return;
        setSourceStatuses([]);
        onError(
          "search",
          getErrorMessage(error, t("capability.skills_external_search_failed")),
        );
      } finally {
        if (searchAbortRef.current === abortController) {
          searchAbortRef.current = null;
        }
        if (requestId === searchRequestRef.current) {
          setLoading(false);
        }
      }
    })();

    return () => abortController.abort();
  }, [
    active,
    onError,
    searchRevision,
    selectedSourceSearchable,
    sourceId,
    sourceRevision,
    submittedQuery,
    t,
  ]);

  const submit = useCallback(() => {
    const normalizedQuery = query.trim();
    if (normalizedQuery.length < MIN_EXTERNAL_SEARCH_LENGTH) return;
    setSubmittedQuery(normalizedQuery);
    setSearchRevision((value) => value + 1);
  }, [query]);

  const closePreview = useCallback(() => {
    previewRequestRef.current += 1;
    setPreviewLoading(false);
    setPreviewItem(null);
  }, []);

  const preview = useCallback(async (item: ExternalSkillSearchItem) => {
    const requestId = ++previewRequestRef.current;
    setPreviewItem(item);
    const previewUrl = item.raw_url || item.detail_url;
    if (isExternalSkillPreviewUnavailable(item) || item.readme_markdown || !previewUrl) {
      setPreviewLoading(false);
      return;
    }
    try {
      setPreviewLoading(true);
      const response = await getExternalSkillPreviewApi(previewUrl);
      if (requestId !== previewRequestRef.current) return;
      setPreviewItem((current) => current && current.detail_url === item.detail_url
        ? { ...current, readme_markdown: response.readme_markdown }
        : current);
    } catch (error) {
      if (requestId === previewRequestRef.current) {
        onError(
          "preview",
          getErrorMessage(error, t("capability.skills_external_preview_failed")),
        );
      }
    } finally {
      if (requestId === previewRequestRef.current) {
        setPreviewLoading(false);
      }
    }
  }, [onError, t]);

  return {
    closePreview,
    loading,
    preview,
    previewItem,
    previewLoading,
    query,
    results,
    setQuery,
    setSourceId,
    sourceId,
    sourceStatuses,
    submit,
    submittedQuery,
  };
}
