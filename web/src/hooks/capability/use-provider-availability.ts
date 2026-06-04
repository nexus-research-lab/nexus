"use client";

import { useCallback, useEffect, useState } from "react";

import {
  get_default_agent_runtime_kind,
  USER_PREFERENCES_CHANGED_EVENT,
} from "@/config/options";
import { list_provider_options_api } from "@/lib/api/provider-config-api";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

interface ProviderAvailabilityState {
  has_available_provider: boolean;
  is_ready: boolean;
  refresh: () => Promise<void>;
}

const cached_has_provider_by_runtime = new Map<AgentRuntimeKind, boolean>();
const subscribers = new Set<(value: boolean) => void>();
const in_flight_by_runtime = new Map<AgentRuntimeKind, Promise<void>>();

async function fetch_availability(runtime_kind = get_default_agent_runtime_kind()): Promise<void> {
  const current_in_flight = in_flight_by_runtime.get(runtime_kind);
  if (current_in_flight) return current_in_flight;

  const request = (async () => {
    try {
      const response = await list_provider_options_api(runtime_kind);
      const next_value = (response?.items ?? []).some((provider) => (provider.models?.length ?? 0) > 0);
      cached_has_provider_by_runtime.set(runtime_kind, next_value);
      subscribers.forEach((subscriber) => subscriber(next_value));
    } catch (error) {
      console.warn("Failed to load provider availability:", error);
    } finally {
      in_flight_by_runtime.delete(runtime_kind);
    }
  })();

  in_flight_by_runtime.set(runtime_kind, request);
  return request;
}

/**
 * 让其它模块（如 Settings 面板的增删改）在变更后主动失效缓存。
 */
export function invalidate_provider_availability(): void {
  const runtime_kind = get_default_agent_runtime_kind();
  cached_has_provider_by_runtime.delete(runtime_kind);
  void fetch_availability(runtime_kind);
}

/**
 * useProviderAvailability — 轻量缓存 Provider 是否就绪，供 Composer 等位置展示提示。
 * 多个调用者共享同一份请求结果，避免重复打 API。
 */
export function useProviderAvailability(): ProviderAvailabilityState {
  const initial_runtime_kind = get_default_agent_runtime_kind();
  const cached_has_provider = cached_has_provider_by_runtime.get(initial_runtime_kind);
  const [has_available_provider, set_has_available_provider] = useState<boolean>(
    cached_has_provider ?? true,
  );
  const [is_ready, set_is_ready] = useState<boolean>(cached_has_provider !== undefined);

  useEffect(() => {
    const subscriber = (value: boolean) => {
      set_has_available_provider(value);
      set_is_ready(true);
    };
    subscribers.add(subscriber);

    const current_runtime_kind = get_default_agent_runtime_kind();
    const cached_value = cached_has_provider_by_runtime.get(current_runtime_kind);
    if (cached_value === undefined) {
      void fetch_availability(current_runtime_kind);
    } else {
      set_has_available_provider(cached_value);
      set_is_ready(true);
    }

    const refresh_current_runtime = () => {
      const runtime_kind = get_default_agent_runtime_kind();
      cached_has_provider_by_runtime.delete(runtime_kind);
      void fetch_availability(runtime_kind);
    };
    const handle_visibility = () => {
      if (document.visibilityState === "visible") refresh_current_runtime();
    };
    document.addEventListener("visibilitychange", handle_visibility);
    window.addEventListener("focus", handle_visibility);
    window.addEventListener(USER_PREFERENCES_CHANGED_EVENT, refresh_current_runtime);

    return () => {
      subscribers.delete(subscriber);
      document.removeEventListener("visibilitychange", handle_visibility);
      window.removeEventListener("focus", handle_visibility);
      window.removeEventListener(USER_PREFERENCES_CHANGED_EVENT, refresh_current_runtime);
    };
  }, []);

  const refresh = useCallback(async () => {
    const runtime_kind = get_default_agent_runtime_kind();
    cached_has_provider_by_runtime.delete(runtime_kind);
    await fetch_availability(runtime_kind);
  }, []);

  return { has_available_provider, is_ready, refresh };
}
