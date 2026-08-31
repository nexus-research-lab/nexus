/**
 * INPUT: 当前 owner 的 runtime/options GET 与 auth owner generation。
 * OUTPUT: 仅在请求仍属于当前 owner 时发布的运行时默认值。
 * POS: Auth bootstrap 的运行时配置读取边界；旧 owner 响应不得写全局默认值。
 */
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import {
  applyRuntimeOptions,
  type RuntimeOptionsSource,
} from "@/config/runtime-options";
import { requestApi } from "@/lib/api/core/http";
import {
  assertAuthOwnerScopeGenerationCurrent,
  captureAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";

export async function hydrateRuntimeOptions(): Promise<void> {
  const ownerScopeGeneration = captureAuthOwnerScopeGeneration();
  const source = await requestApi<RuntimeOptionsSource>(
    `${getAgentApiBaseUrl()}/runtime/options`,
    {
      method: "GET",
      notify_on_401: false,
    },
  );
  assertAuthOwnerScopeGenerationCurrent(ownerScopeGeneration);
  applyRuntimeOptions(source);
}
