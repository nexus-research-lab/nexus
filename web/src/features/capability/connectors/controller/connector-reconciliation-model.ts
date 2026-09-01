/**
 * INPUT: 已进入结果对账的 Connector 反馈与精确 connector ID。
 * OUTPUT: 不覆盖其他 Connector 的不可变恢复反馈映射。
 * POS: Connector 恢复队列纯模型；不执行请求、导航或凭证操作。
 */
import type { ConnectorFeedback } from "./connector-controller-types";

export function upsertConnectorReconciliationFeedback(
  current: ReadonlyMap<string, ConnectorFeedback>,
  connectorId: string,
  feedback: ConnectorFeedback,
): Map<string, ConnectorFeedback> {
  const next = new Map(current);
  next.set(connectorId, feedback);
  return next;
}

export function removeConnectorReconciliationFeedback(
  current: ReadonlyMap<string, ConnectorFeedback>,
  connectorId: string,
): Map<string, ConnectorFeedback> {
  const next = new Map(current);
  next.delete(connectorId);
  return next;
}
