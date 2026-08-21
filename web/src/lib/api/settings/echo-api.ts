import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";

const ECHO_API_URL = `${getAgentApiBaseUrl()}/settings/echo`;

export interface EchoSettings {
  enabled: boolean;
}

/** 获取当前用户的 Echo 全局开关。 */
export const getEchoApi = async (): Promise<EchoSettings> =>
  requestApi<EchoSettings>(ECHO_API_URL, { method: "GET" });

/** 更新当前用户的 Echo 全局开关。 */
export const updateEchoApi = async (
  settings: EchoSettings,
): Promise<EchoSettings> => {
  return requestApi<EchoSettings>(ECHO_API_URL, {
    method: "PUT",
    body: JSON.stringify(settings),
  });
};
