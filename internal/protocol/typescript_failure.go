// INPUT: FailureCore 的 Go wire 契约。
// OUTPUT: protocol TypeScript 生成器复用的失败协议定义。
// POS: FailureCore 的前端类型生成片段；不得手写第二份封闭业务错误码清单。
package protocol

const failureTypeScriptDefinitions = `
export type KnownFailureCategory =
  | 'validation'
  | 'authentication'
  | 'authorization'
  | 'not_found'
  | 'conflict'
  | 'rate_limited'
  | 'unavailable'
  | 'timeout'
  | 'canceled'
  | 'internal';

export type FailureCategory =
  | KnownFailureCategory
  | (string & Record<never, never>);

export type KnownFailureEffect =
  | 'not_applicable'
  | 'not_applied'
  | 'accepted'
  | 'committed'
  | 'unknown';

export type FailureEffect =
  | KnownFailureEffect
  | (string & Record<never, never>);

export type KnownFailureRecoveryActor =
  | 'user'
  | 'system'
  | 'external'
  | 'none';

export type FailureRecoveryActor =
  | KnownFailureRecoveryActor
  | (string & Record<never, never>);

export interface FailureResolution {
  actor: FailureRecoveryActor;
  action: string;
}

export interface FailureCore {
  version: number;
  code: string;
  category: FailureCategory;
  effect: FailureEffect;
  transport_request_id?: string;
  retry_after_ms?: number;
  resolution?: FailureResolution;
}
`
