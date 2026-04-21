"use client";

import { useCallback, useEffect, useState } from "react";

import { list_provider_options_api } from "@/lib/api/provider-config-api";

interface ProviderAvailabilityState {
  has_available_provider: boolean;
  is_ready: boolean;
  refresh: () => Promise<void>;
}

let cached_has_provider: boolean | null = null;
const subscribers = new Set<(value: boolean) => void>();
let in_flight: Promise<void> | null = null;

async function fetch_availability(): Promise<void> {
  if (in_flight) return in_flight;

  in_flight = (async () => {
    try {
      const response = await list_provider_options_api();
      const next_value = (response?.items?.length ?? 0) > 0;
      cached_has_provider = next_value;
      subscribers.forEach((subscriber) => subscriber(next_value));
    } catch (error) {
      console.warn("Failed to load provider availability:", error);
    } finally {
      in_flight = null;
    }
  })();

  return in_flight;
}

/**
 * 让其它模块（如 Settings 面板的增删改）在变更后主动失效缓存。
 */
export function invalidate_provider_availability(): void {
  cached_has_provider = null;
  void fetch_availability();
}

/**
 * useProviderAvailability — 轻量缓存 Provider 是否就绪，供 Composer 等位置展示提示。
 * 多个调用者共享同一份请求结果，避免重复打 API。
 */
export function useProviderAvailability(): ProviderAvailabilityState {
  const [has_available_provider, set_has_available_provider] = useState<boolean>(
    cached_has_provider ?? true,
  );
  const [is_ready, set_is_ready] = useState<boolean>(cached_has_provider !== null);

  useEffect(() => {
    const subscriber = (value: boolean) => {
      set_has_available_provider(value);
      set_is_ready(true);
    };
    subscribers.add(subscriber);

    if (cached_has_provider === null) {
      void fetch_availability();
    } else {
      set_is_ready(true);
    }

    const handle_visibility = () => {
      if (document.visibilityState === "visible") {
        cached_has_provider = null;
        void fetch_availability();
      }
    };
    document.addEventListener("visibilitychange", handle_visibility);
    window.addEventListener("focus", handle_visibility);

    return () => {
      subscribers.delete(subscriber);
      document.removeEventListener("visibilitychange", handle_visibility);
      window.removeEventListener("focus", handle_visibility);
    };
  }, []);

  const refresh = useCallback(async () => {
    cached_has_provider = null;
    await fetch_availability();
  }, []);

  return { has_available_provider, is_ready, refresh };
}
