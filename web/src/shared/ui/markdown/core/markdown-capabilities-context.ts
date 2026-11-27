/**
 * INPUT: 消费侧已绑定资源作用域的文件解析、预览 URL、打开命令与领域链接渲染槽。
 * OUTPUT: 链接、图片与行内代码在渲染时读取的当前能力。
 * POS: 让 Markdown 组件表与能力身份解耦；能力变化只重渲染读取它的元素，不替换元素类型、不重挂载正文。
 */
import { createContext, useContext, type ReactNode } from "react";

import type { ResolveWorkspaceFilePath } from "../workspace/markdown-workspace-artifact-model";

export interface MarkdownCapabilities {
  getFilePreviewUrl?: (path: string) => string;
  onOpenWorkspaceFile?: (path: string) => void;
  // 返回 null/undefined 时继续通用安全链接渲染；领域协议放行由消费侧 URL transform 决定。
  renderLink?: (href: string, children: ReactNode) => ReactNode | null;
  resolveFilePath: ResolveWorkspaceFilePath;
}

const NO_MARKDOWN_CAPABILITIES: MarkdownCapabilities = {
  resolveFilePath: () => null,
};

const MarkdownCapabilitiesContext = createContext(NO_MARKDOWN_CAPABILITIES);

export const MarkdownCapabilitiesProvider = MarkdownCapabilitiesContext.Provider;

export function useMarkdownCapabilities(): MarkdownCapabilities {
  return useContext(MarkdownCapabilitiesContext);
}
