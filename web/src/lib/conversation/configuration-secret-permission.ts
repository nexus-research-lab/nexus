/**
 * INPUT: 当前权限 request、服务端声明的敏感配置槽位与 Composer 内存草稿。
 * OUTPUT: request-scoped 草稿切换、完整性校验，以及只含声明槽位的精确敏感配置载荷。
 * POS: 配置权限的人机交互边界；敏感值只存在当前请求的前端内存中，不进入普通工具输入或持久状态。
 */
import type {
  ConfigurationSecrets,
  ConfigurationSecretSlot,
} from "@/types/conversation/interaction/permission";

export interface ConfigurationSecretDraft {
  requestId: string;
  values: ConfigurationSecrets;
}

export function createConfigurationSecretDraft(
  requestId: string,
): ConfigurationSecretDraft {
  return {
    requestId: requestId.trim(),
    values: {},
  };
}

export function getConfigurationSecretDraftValues(
  draft: ConfigurationSecretDraft,
  requestId: string,
): ConfigurationSecrets {
  return draft.requestId === requestId.trim() ? draft.values : {};
}

export function updateConfigurationSecretDraft(
  draft: ConfigurationSecretDraft,
  requestId: string,
  slotId: string,
  value: string,
): ConfigurationSecretDraft {
  const normalizedRequestId = requestId.trim();
  const normalizedSlotId = slotId.trim();
  const currentValues = draft.requestId === normalizedRequestId
    ? draft.values
    : {};
  if (!normalizedSlotId) {
    return {
      requestId: normalizedRequestId,
      values: { ...currentValues },
    };
  }
  return {
    requestId: normalizedRequestId,
    values: {
      ...currentValues,
      [normalizedSlotId]: value,
    },
  };
}

export function hasCompleteConfigurationSecrets(
  slots: readonly ConfigurationSecretSlot[],
  values: Readonly<ConfigurationSecrets>,
): boolean {
  return slots.every((slot) => {
    const slotId = slot.id.trim();
    return slotId !== "" && (values[slotId] ?? "").trim() !== "";
  });
}

export function selectConfigurationSecrets(
  slots: readonly ConfigurationSecretSlot[],
  values: Readonly<ConfigurationSecrets>,
): ConfigurationSecrets | undefined {
  if (slots.length === 0 || !hasCompleteConfigurationSecrets(slots, values)) {
    return undefined;
  }
  const selected: ConfigurationSecrets = {};
  for (const slot of slots) {
    const slotId = slot.id.trim();
    selected[slotId] = values[slotId];
  }
  return selected;
}
