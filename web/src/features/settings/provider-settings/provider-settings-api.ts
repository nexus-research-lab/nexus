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
import type {
  FetchProviderModelsResponse,
  ProviderConfigRecord,
  ProviderModelRecord,
  ProviderTestResult,
  UpdateProviderModelPayload,
} from "@/types/capability/provider";

export interface ProviderModelApi {
  fetchModels: (provider: string) => Promise<FetchProviderModelsResponse>;
  updateModel: (
    provider: string,
    modelId: string,
    payload: UpdateProviderModelPayload,
  ) => Promise<ProviderModelRecord>;
  testProvider: (provider: string) => Promise<ProviderTestResult>;
  testModel: (provider: string, modelId: string) => Promise<ProviderTestResult>;
}

export interface ProviderSettingsApi {
  listConfigs: () => Promise<ProviderConfigRecord[]>;
  createConfig: typeof createProviderConfigApi;
  updateConfig: typeof updateProviderConfigApi;
  deleteConfig: typeof deleteProviderConfigApi;
  model: ProviderModelApi;
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
