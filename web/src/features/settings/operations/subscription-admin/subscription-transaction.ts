// INPUT: Subscription overview 读取或单个 mutation，以及对应失败投影函数。
// OUTPUT: 同步互斥的快照提交和证据化反馈回调。
// POS: Subscription Admin 事务协调器；不解释异常或自动重试 mutation。
import type { SubscriptionOverview } from "@/types/settings/subscription";

import type {
  FeedbackState,
  PendingSubscriptionMutation,
} from "./subscription-admin-model";

interface SubscriptionLoadOptions {
  failure: (error: unknown) => FeedbackState;
  onFinish: () => void;
  onStart: () => void;
  onSuccess: () => void;
  request: () => Promise<SubscriptionOverview>;
}

interface SubscriptionMutationOptions {
  failure: (error: unknown) => FeedbackState;
  onSuccess?: () => void;
  pending: PendingSubscriptionMutation;
  request: () => Promise<SubscriptionOverview>;
  success: FeedbackState;
}

interface SubscriptionTransactionCallbacks {
  onCommit: (overview: SubscriptionOverview) => void;
  onFeedback: (feedback: FeedbackState) => void;
  onPendingMutation: (pending: PendingSubscriptionMutation | null) => void;
}

interface TransactionOptions {
  failure: (error: unknown) => FeedbackState;
  onFinish: () => void;
  onStart: () => void;
  onSuccess?: () => void;
  request: () => Promise<SubscriptionOverview>;
  success?: FeedbackState;
}

/** 同步锁先于 React 渲染生效，加载与修改共享同一条服务端事务通道。 */
export class SubscriptionTransactionCoordinator {
  private running = false;

  constructor(private readonly callbacks: SubscriptionTransactionCallbacks) {}

  load(options: SubscriptionLoadOptions): Promise<boolean> {
    return this.execute(options);
  }

  runMutation(options: SubscriptionMutationOptions): Promise<boolean> {
    return this.execute({
      failure: options.failure,
      onFinish: () => this.callbacks.onPendingMutation(null),
      onStart: () => this.callbacks.onPendingMutation(options.pending),
      onSuccess: options.onSuccess,
      request: options.request,
      success: options.success,
    });
  }

  private async execute(options: TransactionOptions): Promise<boolean> {
    if (this.running) {
      return false;
    }
    this.running = true;
    try {
      options.onStart();
      this.callbacks.onCommit(await options.request());
      options.onSuccess?.();
      if (options.success) {
        this.callbacks.onFeedback(options.success);
      }
    } catch (error) {
      this.callbacks.onFeedback(options.failure(error));
    } finally {
      this.running = false;
      options.onFinish();
    }
    return true;
  }
}
