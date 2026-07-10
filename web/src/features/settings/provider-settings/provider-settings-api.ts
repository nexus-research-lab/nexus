import {
  createProviderConfigApi,
  createSubscriptionProviderConfigApi,
  deleteProviderConfigApi,
  deleteSubscriptionProviderConfigApi,
  fetchProviderModelsApi,
  fetchSubscriptionProviderModelsApi,
  listProviderConfigsApi,
  listSubscriptionProviderConfigsApi,
  testProviderConfigApi,
  testProviderModelApi,
  testSubscriptionProviderConfigApi,
  testSubscriptionProviderModelApi,
  updateProviderConfigApi,
  updateProviderModelApi,
  updateSubscriptionProviderConfigApi,
  updateSubscriptionProviderModelApi,
} from "@/lib/api/provider-config-api";
import type { ProviderConfigRecord } from "@/types/capability/provider";

import type { ProviderModelActionsApi } from "./use-provider-model-actions";

export interface ProviderSettingsApi {
  listConfigs: () => Promise<ProviderConfigRecord[]>;
  createConfig: typeof createProviderConfigApi;
  updateConfig: typeof updateProviderConfigApi;
  deleteConfig: typeof deleteProviderConfigApi;
  model: ProviderModelActionsApi;
}

const PROVIDER_SETTINGS_APIS: Record<
  ProviderConfigRecord["visibility"],
  ProviderSettingsApi
> = {
  private: {
    listConfigs: listProviderConfigsApi,
    createConfig: createProviderConfigApi,
    updateConfig: updateProviderConfigApi,
    deleteConfig: deleteProviderConfigApi,
    model: {
      fetchModels: fetchProviderModelsApi,
      updateModel: updateProviderModelApi,
      testProvider: testProviderConfigApi,
      testModel: testProviderModelApi,
    },
  },
  public: {
    listConfigs: listSubscriptionProviderConfigsApi,
    createConfig: createSubscriptionProviderConfigApi,
    updateConfig: updateSubscriptionProviderConfigApi,
    deleteConfig: deleteSubscriptionProviderConfigApi,
    model: {
      fetchModels: fetchSubscriptionProviderModelsApi,
      updateModel: updateSubscriptionProviderModelApi,
      testProvider: testSubscriptionProviderConfigApi,
      testModel: testSubscriptionProviderModelApi,
    },
  },
};

export function getProviderSettingsApi(
  visibility: ProviderConfigRecord["visibility"],
): ProviderSettingsApi {
  return PROVIDER_SETTINGS_APIS[visibility];
}
