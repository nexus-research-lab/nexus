// INPUT: Subscription 管理视图共用的字段与资源状态文案。
// OUTPUT: 领域内输入几何、加载和空状态视图。
// POS: Subscription 局部 UI pattern；按钮视觉由 shared UiButton 负责。

import { Loader2 } from "lucide-react";

export function SubscriptionLoadingState({ label }: { label: string }) {
  return (
    <div className="flex items-center justify-center gap-2 px-4 py-10 text-compact text-(--text-soft)">
      <Loader2 className="h-4 w-4 animate-spin" />
      {label}
    </div>
  );
}

export function SubscriptionEmptyState({ label }: { label: string }) {
  return (
    <div className="px-4 py-10 text-center text-compact text-(--text-soft)">
      {label}
    </div>
  );
}
