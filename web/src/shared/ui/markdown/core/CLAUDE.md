# Markdown 核心

- `markdown-code-model.ts`: 将语法节点投影为内联、代码块或 Mermaid 状态。
- `markdown-components.tsx`: 按可判别状态路由正文元素；受控 `renderLink` 先交消费侧解释自有 URI，普通链接继续由共享安全模型处理。图片预览 URL 和文件打开命令只消费已经绑定作用域的闭包。
- `markdown-summary-components.tsx`: 只用于紧凑摘要的内联组件表。
- `markdown-link-model.ts`: 统一链接协议、工作区目标、尾部标点、显示文本和流式 URL 尾部策略。
- `markdown-renderer-shared.tsx`: 组织 React Markdown 插件、受保护区域和共享渲染配置；内部领域 URI 的放行属于消费侧 URL transform。
- `markdown-fence.ts`: Fence 边界识别。
- `markdown-text-plugins.ts`: 用有序、无状态的节点转换规则处理换行和受控内联 HTML。

摘要可以复用正文组件作为基线，但不得把摘要专用分支塞回正文组件表。代码和链接识别必须先投影为可判别状态，组件只负责状态到视图的路由。URL 规范化必须先经过协议白名单；流式尾部匹配按 Markdown 链接、自动链接、裸链接的优先级执行。文本插件的遍历层只执行替换，不得混入标签配对或正则状态管理。

- `markdown-latex-syntax.ts` 拥有显式 LaTeX 括号分隔的 micromark text/flow 扩展；服从代码、链接目标和容器语法，未闭合不伪造结束符。
- `markdown-math.ts` 拥有公式 token 到 MDAST 的投影、已有美元公式的未闭合/歧义处理及 KaTeX 视口焦点；KaTeX 的 trust 保持关闭，局部错误由现有 rehype-katex 收口。
- 同文件的 `remarkMathSummary` 只把识别出的数学节点和显式 math fence 降为内联文本标记；价格歧义保留 literal 标识，普通代码不转换。不通过 CSS 隐藏已排版的公式，也不重新用正则解析整段摘要。
- `markdown-math-ranges.ts` 只为预处理提供保守的源文保护范围，不代替语法解析或修正公式。`markdown-renderer-shared.tsx` 在所有标识符/文件/流式 URL 预处理前应用该边界。
