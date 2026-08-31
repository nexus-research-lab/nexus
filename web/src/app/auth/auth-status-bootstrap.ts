/**
 * INPUT: 当前 auth owner generation、AuthStatus loader 与唯一受理回调。
 * OUTPUT: 同 generation 单飞、跨 generation 强制新读且旧结果静默失效的 Promise。
 * POS: AuthProvider 的 bootstrap 并发边界；不持有身份、cookie 或 React 状态。
 */

import type { AuthStatus } from "@/lib/api/account/auth-api";
import {
  assertAuthOwnerScopeGenerationCurrent,
  captureAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";

interface ActiveAuthStatusBootstrap {
  generation: number;
  promise: Promise<AuthStatus>;
}

let activeAuthStatusBootstrap: ActiveAuthStatusBootstrap | null = null;

/** 同一 owner 代次共用完整受理流程；新代次绝不复用旧请求。 */
export function runAuthStatusBootstrap(
  load: () => Promise<AuthStatus>,
  accept: (status: AuthStatus) => Promise<AuthStatus>,
): Promise<AuthStatus> {
  const generation = captureAuthOwnerScopeGeneration();
  if (activeAuthStatusBootstrap?.generation === generation) {
    return activeAuthStatusBootstrap.promise;
  }

  let promise: Promise<AuthStatus>;
  const loaded = load().then(
    (status) => {
      assertAuthOwnerScopeGenerationCurrent(generation);
      return status;
    },
    (error: unknown) => {
      // 旧 owner 的失败同样不能覆盖新请求的 loading/error 状态。
      assertAuthOwnerScopeGenerationCurrent(generation);
      throw error;
    },
  );
  promise = loaded
    .then(accept)
    .finally(() => {
      if (activeAuthStatusBootstrap?.promise === promise) {
        activeAuthStatusBootstrap = null;
      }
    });
  activeAuthStatusBootstrap = { generation, promise };
  return promise;
}
