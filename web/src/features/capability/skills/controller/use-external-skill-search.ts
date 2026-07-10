import { useCallback, useEffect, useRef, useState } from "react";

import {
  getExternalSkillPreviewApi,
  searchExternalSkillsApi,
} from "@/lib/api/skill-api";
import type {
  ExternalSkillSearchItem,
  ExternalSkillSourceStatus,
} from "@/types/capability/skill";

import type { ExternalSkillSearchController } from "./skill-marketplace-controller";

const MIN_EXTERNAL_SEARCH_LENGTH = 2;

interface UseExternalSkillSearchOptions {
  active: boolean;
  onError: (message: string) => void;
  sourceRevision: number;
}

export function useExternalSkillSearch({
  active,
  onError,
  sourceRevision,
}: UseExternalSkillSearchOptions): ExternalSkillSearchController {
  const [query, setQuery] = useState("");
  const [submittedQuery, setSubmittedQuery] = useState("");
  const [searchRevision, setSearchRevision] = useState(0);
  const [results, setResults] = useState<ExternalSkillSearchItem[]>([]);
  const [sourceStatuses, setSourceStatuses] = useState<ExternalSkillSourceStatus[]>([]);
  const [loading, setLoading] = useState(false);
  const [previewItem, setPreviewItem] = useState<ExternalSkillSearchItem | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const searchRequestRef = useRef(0);
  const searchAbortRef = useRef<AbortController | null>(null);
  const previewRequestRef = useRef(0);

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
    if (!active || query.trim().length >= MIN_EXTERNAL_SEARCH_LENGTH) return;
    clearResults();
  }, [active, clearResults, query]);

  useEffect(() => {
    if (!active) return;
    const normalizedQuery = submittedQuery.trim();
    if (normalizedQuery.length < MIN_EXTERNAL_SEARCH_LENGTH) return;

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
          abortController.signal,
        );
        if (requestId !== searchRequestRef.current) return;
        setResults(response.results);
        setSourceStatuses(response.sources);
      } catch (error) {
        if (abortController.signal.aborted) return;
        if (requestId !== searchRequestRef.current) return;
        setSourceStatuses([]);
        onError(error instanceof Error ? error.message : "外部技能搜索失败");
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
  }, [active, onError, searchRevision, sourceRevision, submittedQuery]);

  const submit = useCallback(() => {
    const normalizedQuery = query.trim();
    if (normalizedQuery.length < MIN_EXTERNAL_SEARCH_LENGTH) {
      clearResults();
      return;
    }
    setSubmittedQuery(normalizedQuery);
    setSearchRevision((value) => value + 1);
  }, [clearResults, query]);

  const closePreview = useCallback(() => {
    previewRequestRef.current += 1;
    setPreviewLoading(false);
    setPreviewItem(null);
  }, []);

  const preview = useCallback(async (item: ExternalSkillSearchItem) => {
    const requestId = ++previewRequestRef.current;
    setPreviewItem(item);
    const builtInPreviewUnavailable =
      item.source_kind === "skills_sh" || item.import_mode === "skills_sh";
    const previewUrl = item.raw_url || item.detail_url;
    if (builtInPreviewUnavailable || item.readme_markdown || !previewUrl) {
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
        onError(error instanceof Error ? error.message : "技能预览加载失败");
      }
    } finally {
      if (requestId === previewRequestRef.current) {
        setPreviewLoading(false);
      }
    }
  }, [onError]);

  return {
    closePreview,
    loading,
    preview,
    previewItem,
    previewLoading,
    query,
    results,
    setQuery,
    sourceStatuses,
    submit,
    submittedQuery,
  };
}
