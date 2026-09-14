// INPUT: 窄窗辅助页、Thread 或任务内容、可读标题与关闭命令。
// OUTPUT: 统一的全屏模态材质、纵向骨架、焦点/键盘和滚动锁行为。
// POS: Room 专注模式的领域外壳；平台页头归内容，模态协议归共享 Dialog。

import { useRef, type ReactNode } from "react";
import { cn } from "@/shared/ui/class-name";
import { useDialogModalBehavior } from "@/shared/ui/dialog/dialog-behavior";
import { getUiOverlayLayerClassName } from "@/shared/ui/overlay/layer-styles";

export function RoomMobileOverlayFrame({ children, label, onClose }: {
  children: ReactNode;
  label: string;
  onClose: () => void;
}) {
  const rootRef = useRef<HTMLDivElement>(null);
  useDialogModalBehavior({ onClose, rootRef });
  return <div
    ref={rootRef}
    role="dialog"
    aria-modal="true"
    aria-label={label}
    data-modal-root="true"
    tabIndex={-1}
    className={cn(
      "fixed inset-0 flex min-h-0 min-w-0 flex-col overflow-hidden [background:var(--surface-popover-background)] backdrop-blur-2xl",
      getUiOverlayLayerClassName("dialog"),
    )}
  >{children}</div>;
}
