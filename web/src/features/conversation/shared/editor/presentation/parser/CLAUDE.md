# PPTX Parser Internals

- `presentation-shape-tree-parser.ts` 负责 Shape Tree 节点分派、占位符继承、组变换、图片资源和预览跳过规则。
- 包入口负责 slide/layout/master 继承顺序；本目录不得读取 `presentation.xml` 或决定幻灯片顺序。

Shape Tree 使用单一可变 ID 上下文保持组内元素顺序。图片 Object URL 只登记到包级资源列表，由包入口统一清理。
