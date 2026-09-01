/**
 * INPUT: 当前 owner generation、runtime 默认值与 Provider options API。
 * OUTPUT: 同 owner/runtime 共享且跨 owner 不复用响应的 Composer Provider readiness。
 * POS: Provider 配置到聊天入口之间的轻量只读门禁；不保存 Provider 详情。
 */
"use client";

import {
  useCallback,
  useEffect,
  useState,
  useSyncExternalStore,
} from "react";

import {
  getDefaultAgentRuntimeKind,
  USER_PREFERENCES_CHANGED_EVENT,
} from "@/config/runtime-options";
import { listProviderOptionsApi } from "@/lib/api/settings/provider-api";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";

import {
  ProviderAvailabilityResource,
  type ProviderAvailabilityEvent,
} from "./provider-availability-resource";

interface ProviderAvailabilityState {
  hasAvailableProvider: boolean;
  isReady: boolean;
  refresh: () => Promise<void>;
}

const providerAvailabilityResource = new ProviderAvailabilityResource({
  isGenerationCurrent: isAuthOwnerScopeGenerationCurrent,
  load: async (runtimeKind) => {
    const response = await listProviderOptionsApi(runtimeKind);
    // 只有能解析到当前用户真正选择的 Provider/Model，聊天才具备启动条件。
    // 仅检查模型列表会把“有模型但没有默认模型”的半配置状态误报为可用。
    const selection = response?.default_selection;
    return Boolean(
      selection
      && (response?.items ?? []).some((provider) => (
        provider.provider === selection.provider
        && provider.models.some((model) => model.model_id === selection.model)
      )),
    );
  },
  reportError: (error) => {
    console.warn("Failed to load provider availability:", error);
  },
});

/**
 * 让其它模块（如 Settings 面板的增删改）在变更后主动失效缓存。
 */
export function invalidateProviderAvailability(): void {
  const ownerGeneration = captureAuthOwnerScopeGeneration();
  const runtimeKind = getDefaultAgentRuntimeKind();
  void providerAvailabilityResource.invalidate(ownerGeneration, runtimeKind);
}

/**
 * useProviderAvailability — 轻量缓存 Provider 是否就绪，供 Composer 等位置展示提示。
 * 多个调用者共享同一份请求结果，避免重复打 API。
 */
export function useProviderAvailability(): ProviderAvailabilityState {
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  const initialRuntimeKind = getDefaultAgentRuntimeKind();
  const cachedHasProvider = providerAvailabilityResource.read(
    ownerGeneration,
    initialRuntimeKind,
  );
  const [snapshot, setSnapshot] = useState<ProviderAvailabilityEvent | null>(
    cachedHasProvider === undefined
      ? null
      : { generation: ownerGeneration, runtimeKind: initialRuntimeKind, value: cachedHasProvider },
  );

  useEffect(() => {
    const subscriber = (event: ProviderAvailabilityEvent) => {
      if (
        event.generation === ownerGeneration
        && event.runtimeKind === getDefaultAgentRuntimeKind()
      ) {
        setSnapshot(event);
      }
    };
    const unsubscribe = providerAvailabilityResource.subscribe(subscriber);

    const currentRuntimeKind = getDefaultAgentRuntimeKind();
    const cachedValue = providerAvailabilityResource.read(
      ownerGeneration,
      currentRuntimeKind,
    );
    if (cachedValue === undefined) {
      setSnapshot(null);
      void providerAvailabilityResource.fetch(
        ownerGeneration,
        currentRuntimeKind,
      );
    } else {
      setSnapshot({
        generation: ownerGeneration,
        runtimeKind: currentRuntimeKind,
        value: cachedValue,
      });
    }

    const refreshCurrentRuntime = () => {
      const generation = captureAuthOwnerScopeGeneration();
      const runtimeKind = getDefaultAgentRuntimeKind();
      setSnapshot(null);
      void providerAvailabilityResource.invalidate(generation, runtimeKind);
    };
    const handleVisibility = () => {
      if (document.visibilityState === "visible") refreshCurrentRuntime();
    };
    document.addEventListener("visibilitychange", handleVisibility);
    window.addEventListener("focus", handleVisibility);
    window.addEventListener(USER_PREFERENCES_CHANGED_EVENT, refreshCurrentRuntime);

    return () => {
      unsubscribe();
      document.removeEventListener("visibilitychange", handleVisibility);
      window.removeEventListener("focus", handleVisibility);
      window.removeEventListener(USER_PREFERENCES_CHANGED_EVENT, refreshCurrentRuntime);
    };
  }, [ownerGeneration]);

  const refresh = useCallback(async () => {
    const generation = captureAuthOwnerScopeGeneration();
    const runtimeKind = getDefaultAgentRuntimeKind();
    setSnapshot(null);
    await providerAvailabilityResource.invalidate(generation, runtimeKind);
  }, []);

  const currentRuntimeKind = getDefaultAgentRuntimeKind();
  const isCurrentSnapshot = snapshot?.generation === ownerGeneration
    && snapshot.runtimeKind === currentRuntimeKind;
  return {
    hasAvailableProvider: isCurrentSnapshot ? snapshot.value : true,
    isReady: isCurrentSnapshot,
    refresh,
  };
}
