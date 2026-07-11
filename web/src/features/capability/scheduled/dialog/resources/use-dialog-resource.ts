"use client";

import { useEffect, useState } from "react";

import { getErrorMessage } from "@/lib/error-message";

export interface DialogResource<T> {
  error: string | null;
  items: T[];
  loading: boolean;
}

interface ResourceSnapshot<T> extends DialogResource<T> {
  key: string | null;
}

const IDLE_RESOURCE: DialogResource<never> = {
  error: null,
  items: [],
  loading: false,
};

export function useDialogResource<T>(
  requestKey: string | null,
  load: (key: string) => Promise<T[]>,
  fallbackError: string,
): DialogResource<T> {
  const [snapshot, setSnapshot] = useState<ResourceSnapshot<T>>({
    ...IDLE_RESOURCE,
    key: null,
  });

  useEffect(() => {
    if (!requestKey) {
      return;
    }

    let active = true;
    setSnapshot({ error: null, items: [], key: requestKey, loading: true });
    void load(requestKey)
      .then((items) => {
        if (active) {
          setSnapshot({ error: null, items, key: requestKey, loading: false });
        }
      })
      .catch((error: unknown) => {
        if (active) {
          setSnapshot({
            error: getErrorMessage(error, fallbackError),
            items: [],
            key: requestKey,
            loading: false,
          });
        }
      });

    return () => {
      active = false;
    };
  }, [fallbackError, load, requestKey]);

  if (!requestKey) {
    return IDLE_RESOURCE;
  }
  if (snapshot.key !== requestKey) {
    return { error: null, items: [], loading: true };
  }
  return snapshot;
}
