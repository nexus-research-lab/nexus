import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { listLoopsApi } from "@/lib/api/capability/loop-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { LoopCatalogItem } from "@/types/capability/loop";

import {
  ALL_LOOP_CATEGORIES,
  buildLoopCategoryOptions,
  filterLoops,
} from "./loop-picker-model";

interface LoopPickerResource {
  error: string | null;
  isLoading: boolean;
  loops: LoopCatalogItem[];
}

const INITIAL_RESOURCE: LoopPickerResource = {
  error: null,
  isLoading: true,
  loops: [],
};

export function useLoopPickerController({
  onClose,
  onSelect,
}: {
  onClose: () => void;
  onSelect: (loop: LoopCatalogItem) => void | Promise<void>;
}) {
  const { locale, t } = useI18n();
  const [resource, setResource] = useState(INITIAL_RESOURCE);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState(ALL_LOOP_CATEGORIES);
  const [busySlug, setBusySlug] = useState<string | null>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    searchInputRef.current?.focus();
  }, []);

  useEffect(() => {
    let active = true;
    setResource(INITIAL_RESOURCE);
    void listLoopsApi(locale)
      .then((loops) => {
        if (!active) {
          return;
        }
        setResource({ error: null, isLoading: false, loops });
      })
      .catch((error: unknown) => {
        if (!active) {
          return;
        }
        setResource({
          error: getLoopPickerError(error, t("composer.loop_picker_failed")),
          isLoading: false,
          loops: [],
        });
      });
    return () => {
      active = false;
    };
  }, [locale, t]);

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
    setResource((current) => ({ ...current, error: null }));
    try {
      await onSelect(loop);
      onClose();
    } catch (error) {
      setResource((current) => ({
        ...current,
        error: getLoopPickerError(
          error,
          t("composer.loop_picker_failed"),
        ),
      }));
    } finally {
      setBusySlug(null);
    }
  }, [busySlug, onClose, onSelect, t]);

  return {
    actions: { selectLoop, setCategory, setQuery },
    refs: { searchInputRef },
    state: {
      busySlug,
      category,
      categoryOptions,
      error: resource.error,
      filteredLoops,
      isLoading: resource.isLoading,
      query,
    },
  };
}

function getLoopPickerError(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}
