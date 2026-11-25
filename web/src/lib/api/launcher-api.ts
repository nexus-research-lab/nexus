/**
 * INPUT: 可选 AbortSignal 与运行时 API 根地址。
 * OUTPUT: 保持 wire 兼容的首屏目录 bootstrap 响应。
 * POS: 共享目录快照的无状态 HTTP 协议客户端；请求合并归调用方资源状态机。
 */

import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type { LauncherBootstrapResponse } from "@/types/app/launcher";

export async function getLauncherBootstrapApi(
  signal?: AbortSignal,
): Promise<LauncherBootstrapResponse> {
  return requestApi<LauncherBootstrapResponse>(
    `${getAgentApiBaseUrl()}/launcher/bootstrap`,
    {
      method: "GET",
      signal,
    },
  );
}
