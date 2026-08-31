/**
 * INPUT: Auth owner scope 的同步推进与完成清理后的发布。
 * OUTPUT: 用于丢弃旧回调并令存活消费者重建资源的进程内 generation capture/fence。
 * POS: 客户端 owner 事件边界；不参与服务端身份、资源 ID、路由或持久化。
 */

let authOwnerScopeGeneration = 0;
const authOwnerScopeGenerationListeners = new Set<() => void>();

export class AuthOwnerScopeSupersededError extends Error {
  constructor() {
    super("Auth owner scope changed before the request completed");
    this.name = "AuthOwnerScopeSupersededError";
  }
}

/** 捕获当前订阅所属的 owner 代次。 */
export function captureAuthOwnerScopeGeneration(): number {
  return authOwnerScopeGeneration;
}

/** 只有仍属于同一 owner 代次的异步回调才允许提交。 */
export function isAuthOwnerScopeGenerationCurrent(
  generation: number,
): boolean {
  return generation === authOwnerScopeGeneration;
}

/** 在任何异步结果产生副作用前执行；失效属于静默取消，不是用户可重试错误。 */
export function assertAuthOwnerScopeGenerationCurrent(
  generation: number,
): void {
  if (!isAuthOwnerScopeGenerationCurrent(generation)) {
    throw new AuthOwnerScopeSupersededError();
  }
}

export function isAuthOwnerScopeSupersededError(
  error: unknown,
): error is AuthOwnerScopeSupersededError {
  return error instanceof AuthOwnerScopeSupersededError;
}

/** 身份边界清空前先推进代次，使已挂载订阅立即失去提交资格。 */
export function advanceAuthOwnerScopeGeneration(): void {
  authOwnerScopeGeneration += 1;
}

/** 订阅代次推进；发布必须发生在旧共享连接已经摘除之后。 */
export function subscribeAuthOwnerScopeGeneration(
  listener: () => void,
): () => void {
  authOwnerScopeGenerationListeners.add(listener);
  return () => {
    authOwnerScopeGenerationListeners.delete(listener);
  };
}

/** Auth 装配层完成同步清理后，通知仍挂载的消费者重建当前 owner 资源。 */
export function publishAuthOwnerScopeGeneration(): void {
  for (const listener of authOwnerScopeGenerationListeners) {
    listener();
  }
}
