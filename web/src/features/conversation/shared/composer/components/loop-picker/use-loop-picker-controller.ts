import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { listLoopsApi } from "@/lib/api/capability/loop-api";
import {
  getErrorMessage,
  getResourceFailure,
  type ResourceFailure,
} from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { LoopCatalogItem } from "@/types/capability/loop";

import {
  ALL_LOOP_CATEGORIES,
  buildLoopCategoryOptions,
  filterLoops,
} from "./loop-picker-model";

interface LoopPickerResource {
  error: ResourceFailure | null;
  hasSnapshot: boolean;
  isLoading: boolean;
  loops: LoopCatalogItem[];
  scopeKey: string;
}

function createLoopPickerResource(scopeKey: string): LoopPickerResource {
  return {
    error: null,
    hasSnapshot: false,
    isLoading: true,
    loops: [],
    scopeKey,
  };
}

export function useLoopPickerController({
  onClose,
  onSelect,
}: {
  onClose: () => void;
  onSelect: (loop: LoopCatalogItem) => void | Promise<void>;
}) {
  const { locale, t } = useI18n();
  const [resource, setResource] = useState(() => createLoopPickerResource(locale));
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState(ALL_LOOP_CATEGORIES);
  const [busySlug, setBusySlug] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [loadRevision, setLoadRevision] = useState(0);
  const searchInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    searchInputRef.current?.focus();
  }, []);

  useEffect(() => {
    let active = true;
    setResource((current) => current.scopeKey === locale
      ? {
          ...current,
          error: current.error?.access ? current.error : null,
          isLoading: true,
        }
      : createLoopPickerResource(locale));
    void listLoopsApi(locale)
      .then((loops) => {
        if (!active) {
          return;
        }
        setResource({
          error: null,
          hasSnapshot: true,
          isLoading: false,
          loops,
          scopeKey: locale,
        });
      })
      .catch((error: unknown) => {
        if (!active) {
          return;
        }
        setResource((current) => current.scopeKey === locale
          ? {
              ...current,
              error: getResourceFailure(error, t("composer.loop_picker_failed")),
              isLoading: false,
            }
          : current);
      });
    return () => {
      active = false;
    };
  }, [loadRevision, locale, t]);

  const categoryOptions = useMemo(
    () => buildLoopCategoryOptions(
      resource.loops,
      t("capability.category_all"),
    ),
    [resource.loops, t],
  );
  const filteredLoops = useMemo(
    () => filterLoops(resource.loops, category, query),
    [category, query, resource.loops],
  );

  const selectLoop = useCallback(async (loop: LoopCatalogItem) => {
    if (busySlug) {
      return;
    }
    setBusySlug(loop.slug);
    setActionError(null);
    try {
      await onSelect(loop);
      onClose();
    } catch (error) {
      setActionError(getErrorMessage(error, t("composer.loop_picker_failed")));
    } finally {
      setBusySlug(null);
    }
  }, [busySlug, onClose, onSelect, t]);

  return {
    actions: {
      clearFilters: () => {
        setCategory(ALL_LOOP_CATEGORIES);
        setQuery("");
      },
      retryLoad: () => setLoadRevision((current) => current + 1),
      selectLoop,
      setCategory,
      setQuery,
    },
    refs: { searchInputRef },
    state: {
      busySlug,
      category,
      categoryOptions,
      actionError,
      error: resource.error,
      filteredLoops,
      hasCatalogItems: resource.loops.length > 0,
      hasSnapshot: resource.hasSnapshot && resource.scopeKey === locale,
      isLoading: resource.isLoading,
      query,
    },
  };
}
