import {
  createProviderConfigApi,
  createSubscriptionProviderConfigApi,
  deleteProviderModelApi,
  deleteProviderConfigApi,
  deleteSubscriptionProviderModelApi,
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
} from "@/lib/api/settings/provider-api";
import type {
  DeleteProviderModelResponse,
  FetchProviderModelsResponse,
  ProviderConfigRecord,
  ProviderModelRecord,
  ProviderTestResult,
  UpdateProviderModelPayload,
} from "@/types/capability/provider";

export interface ProviderModelApi {
  fetchModels: (provider: string) => Promise<FetchProviderModelsResponse>;
  deleteModel: (
    provider: string,
    modelId: string,
  ) => Promise<DeleteProviderModelResponse>;
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
      deleteModel: deleteProviderModelApi,
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
      deleteModel: deleteSubscriptionProviderModelApi,
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
