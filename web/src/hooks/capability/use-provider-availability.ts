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

const cachedHasProviderByRuntime = new Map<AgentRuntimeKind, boolean>();
const subscribers = new Set<(value: boolean) => void>();
const inFlightByRuntime = new Map<AgentRuntimeKind, Promise<void>>();

async function fetchAvailability(runtimeKind = get_default_agent_runtime_kind()): Promise<void> {
  const currentInFlight = inFlightByRuntime.get(runtimeKind);
  if (currentInFlight) return currentInFlight;

  const request = (async () => {
    try {
      const response = await list_provider_options_api(runtimeKind);
      const nextValue = (response?.items ?? []).some((provider) => (provider.models?.length ?? 0) > 0);
      cachedHasProviderByRuntime.set(runtimeKind, nextValue);
      subscribers.forEach((subscriber) => subscriber(nextValue));
    } catch (error) {
      console.warn("Failed to load provider availability:", error);
    } finally {
      inFlightByRuntime.delete(runtimeKind);
    }
  })();

  inFlightByRuntime.set(runtimeKind, request);
  return request;
}

/**
 * 让其它模块（如 Settings 面板的增删改）在变更后主动失效缓存。
 */
export function invalidate_provider_availability(): void {
  const runtimeKind = get_default_agent_runtime_kind();
  cachedHasProviderByRuntime.delete(runtimeKind);
  void fetchAvailability(runtimeKind);
}

/**
 * useProviderAvailability — 轻量缓存 Provider 是否就绪，供 Composer 等位置展示提示。
 * 多个调用者共享同一份请求结果，避免重复打 API。
 */
export function useProviderAvailability(): ProviderAvailabilityState {
  const initialRuntimeKind = get_default_agent_runtime_kind();
  const cachedHasProvider = cachedHasProviderByRuntime.get(initialRuntimeKind);
  const [hasAvailableProvider, setHasAvailableProvider] = useState<boolean>(
    cachedHasProvider ?? true,
  );
  const [isReady, setIsReady] = useState<boolean>(cachedHasProvider !== undefined);

  useEffect(() => {
    const subscriber = (value: boolean) => {
      setHasAvailableProvider(value);
      setIsReady(true);
    };
    subscribers.add(subscriber);

    const currentRuntimeKind = get_default_agent_runtime_kind();
    const cachedValue = cachedHasProviderByRuntime.get(currentRuntimeKind);
    if (cachedValue === undefined) {
      void fetchAvailability(currentRuntimeKind);
    } else {
      setHasAvailableProvider(cachedValue);
      setIsReady(true);
    }

    const refreshCurrentRuntime = () => {
      const runtimeKind = get_default_agent_runtime_kind();
      cachedHasProviderByRuntime.delete(runtimeKind);
      void fetchAvailability(runtimeKind);
    };
    const handleVisibility = () => {
      if (document.visibilityState === "visible") refreshCurrentRuntime();
    };
    document.addEventListener("visibilitychange", handleVisibility);
    window.addEventListener("focus", handleVisibility);
    window.addEventListener(USER_PREFERENCES_CHANGED_EVENT, refreshCurrentRuntime);

    return () => {
      subscribers.delete(subscriber);
      document.removeEventListener("visibilitychange", handleVisibility);
      window.removeEventListener("focus", handleVisibility);
      window.removeEventListener(USER_PREFERENCES_CHANGED_EVENT, refreshCurrentRuntime);
    };
  }, []);

  const refresh = useCallback(async () => {
    const runtimeKind = get_default_agent_runtime_kind();
    cachedHasProviderByRuntime.delete(runtimeKind);
    await fetchAvailability(runtimeKind);
  }, []);

  return { has_available_provider: hasAvailableProvider, is_ready: isReady, refresh };
}
