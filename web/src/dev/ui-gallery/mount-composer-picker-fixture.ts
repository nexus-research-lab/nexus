// INPUT: 浏览器测试注入的只读工作图目录路由。
// OUTPUT: 挂载实际 Composer 选择器，选择结果只记录在本地 output。
// POS: 仅由浏览器测试加载的夹具；测试须拦截目录读取，不装配真实启动或发送命令。

import { createElement, useState } from "react";
import { createRoot } from "react-dom/client";
import { WorkGraphDistillationPickerDialog } from "@/features/conversation/shared/composer/components/workgraph-distillation-picker/workgraph-distillation-picker-dialog";
import { I18nProvider } from "@/shared/i18n/i18n-provider";

function PickerFixture() {
  const [open, setOpen] = useState(true);
  const [selection, setSelection] = useState("");
  return createElement(I18nProvider, null,
    createElement("output", { "data-composer-picker-result": "workgraph" }, selection),
    createElement(WorkGraphDistillationPickerDialog, { isOpen: open, onClose: () => setOpen(false), onUseCommand: setSelection }),
  );
}

export function mountComposerPickerFixture() {
  const container = document.createElement("div");
  document.body.append(container);
  createRoot(container).render(createElement(PickerFixture));
}
