/**
 * INPUT: owner generation、runtime kind、Provider 可用性 loader 与订阅者。
 * OUTPUT: 同 owner/runtime 单飞、跨 owner 清空且迟到结果不可发布的可用性缓存。
 * POS: useProviderAvailability 的无 React 竞态状态机；不读取全局 runtime 或认证状态。
 */

import type { AgentRuntimeKind } from "@/types/settings/preferences";

export interface ProviderAvailabilityEvent {
  generation: number;
  runtimeKind: AgentRuntimeKind;
  value: boolean;
}

interface ProviderAvailabilityResourceOptions {
  isGenerationCurrent: (generation: number) => boolean;
  load: (runtimeKind: AgentRuntimeKind) => Promise<boolean>;
  reportError: (error: unknown) => void;
}

type ProviderAvailabilitySubscriber = (
  event: ProviderAvailabilityEvent,
) => void;

export class ProviderAvailabilityResource {
  private readonly cachedByRuntime = new Map<AgentRuntimeKind, boolean>();
  private readonly inFlightByRuntime = new Map<AgentRuntimeKind, Promise<void>>();
  private readonly subscribers = new Set<ProviderAvailabilitySubscriber>();
  private generation: number | null = null;

  constructor(private readonly options: ProviderAvailabilityResourceOptions) {}

  read(generation: number, runtimeKind: AgentRuntimeKind): boolean | undefined {
    if (!this.options.isGenerationCurrent(generation)) {
      return undefined;
    }
    this.synchronizeGeneration(generation);
    return this.cachedByRuntime.get(runtimeKind);
  }

  invalidate(
    generation: number,
    runtimeKind: AgentRuntimeKind,
  ): Promise<void> {
    if (!this.options.isGenerationCurrent(generation)) {
      return Promise.resolve();
    }
    this.synchronizeGeneration(generation);
    this.cachedByRuntime.delete(runtimeKind);
    return this.fetch(generation, runtimeKind);
  }

  fetch(generation: number, runtimeKind: AgentRuntimeKind): Promise<void> {
    // 迟到的旧 Hook 不能把 resource generation 倒退并清掉新 owner 的缓存/单飞请求。
    if (!this.options.isGenerationCurrent(generation)) {
      return Promise.resolve();
    }
    this.synchronizeGeneration(generation);
    const currentInFlight = this.inFlightByRuntime.get(runtimeKind);
    if (currentInFlight) {
      return currentInFlight;
    }

    let request: Promise<void>;
    request = this.options.load(runtimeKind)
      .then((value) => {
        if (!this.canPublish(generation)) {
          return;
        }
        this.cachedByRuntime.set(runtimeKind, value);
        const event = { generation, runtimeKind, value };
        for (const subscriber of this.subscribers) {
          subscriber(event);
        }
      })
      .catch((error: unknown) => {
        if (this.canPublish(generation)) {
          this.options.reportError(error);
        }
      })
      .finally(() => {
        // 旧 owner promise 的 finally 不能删除同 runtime 的新 owner 请求。
        if (this.inFlightByRuntime.get(runtimeKind) === request) {
          this.inFlightByRuntime.delete(runtimeKind);
        }
      });
    this.inFlightByRuntime.set(runtimeKind, request);
    return request;
  }

  subscribe(subscriber: ProviderAvailabilitySubscriber): () => void {
    this.subscribers.add(subscriber);
    return () => {
      this.subscribers.delete(subscriber);
    };
  }

  private canPublish(generation: number): boolean {
    return this.generation === generation
      && this.options.isGenerationCurrent(generation);
  }

  private synchronizeGeneration(generation: number): void {
    if (this.generation === generation) {
      return;
    }
    this.generation = generation;
    this.cachedByRuntime.clear();
    this.inFlightByRuntime.clear();
  }
}
