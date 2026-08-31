"use client";

import { useCallback, useEffect, useState } from "react";

export interface DialogResourceStatus {
  error: string | null;
  loading: boolean;
  retry: () => void;
}

export interface DialogResource<T> extends DialogResourceStatus {
  items: T[];
}

interface ResourceSnapshot<T> {
  error: string | null;
  items: T[];
  key: string | null;
  loading: boolean;
}

const IGNORE_RETRY = () => undefined;

const IDLE_RESOURCE: DialogResource<never> = {
  error: null,
  items: [],
  loading: false,
  retry: IGNORE_RETRY,
};

export function useDialogResource<T>(
  requestKey: string | null,
  load: (key: string) => Promise<T[]>,
  fallbackError: string,
): DialogResource<T> {
  const [snapshot, setSnapshot] = useState<ResourceSnapshot<T>>({
    error: null,
    items: [],
    key: null,
    loading: false,
  });
  const [retryRevision, setRetryRevision] = useState(0);
  const retry = useCallback(() => {
    setRetryRevision((current) => current + 1);
  }, []);

  useEffect(() => {
    if (!requestKey) {
      return;
    }

    let active = true;
    setSnapshot((current) => current.key === requestKey
      ? { ...current, error: null, loading: true }
      : { error: null, items: [], key: requestKey, loading: true });
    void load(requestKey)
      .then((items) => {
        if (active) {
          setSnapshot({ error: null, items, key: requestKey, loading: false });
        }
      })
      .catch(() => {
        if (active) {
          setSnapshot((current) => current.key === requestKey
            ? { ...current, error: fallbackError, loading: false }
            : {
                error: fallbackError,
                items: [],
                key: requestKey,
                loading: false,
              });
        }
      });

    return () => {
      active = false;
    };
  }, [fallbackError, load, requestKey, retryRevision]);

  if (!requestKey) {
    return IDLE_RESOURCE;
  }
  if (snapshot.key !== requestKey) {
    return { error: null, items: [], loading: true, retry };
  }
  return { ...snapshot, retry };
}
