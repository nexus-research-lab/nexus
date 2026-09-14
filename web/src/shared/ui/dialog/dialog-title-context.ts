// INPUT: 当前 Dialog 提供的标题注册入口。
// OUTPUT: Header 到最近模态根的实例内标题关联，不跨嵌套 Dialog 共享。
// POS: Dialog 内部结构协议；业务使用 Header.title 或 Backdrop 的显式命名属性。

import { createContext, type Dispatch, type SetStateAction } from "react";

export const DIALOG_TITLE_CONTEXT = createContext<Dispatch<SetStateAction<string | undefined>> | null>(null);
