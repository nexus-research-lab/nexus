import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import {
  applyRuntimeOptions,
  type RuntimeOptionsSource,
} from "@/config/runtime-options";
import { requestApi } from "@/lib/api/core/http";

export async function hydrateRuntimeOptions(): Promise<void> {
  const source = await requestApi<RuntimeOptionsSource>(
    `${getAgentApiBaseUrl()}/runtime/options`,
    {
      method: "GET",
      notify_on_401: false,
    },
  );
  applyRuntimeOptions(source);
}
