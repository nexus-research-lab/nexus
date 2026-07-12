# 演示文稿预览

- `presentation-preview-model.ts` 只定义跨解析器和视图消费的预览结果与元素，不暴露解析器内部状态。
- `presentation-xml-utils.ts` 负责 ZIP 路径、关系和 DrawingML XML 基础访问。
- `presentation-pptx-parser.ts` 作为包入口编排幻灯片继承链、共享 Part 缓存和图片资源生命周期。
- `parser/` 独立负责 Shape Tree、占位符、组变换、图片和预览跳过规则。
- `presentation-shape-style.ts` 读取坐标、组变换、几何、填充和描边，并投影组内样式。
- `presentation-text-parser.ts` 只解析文本段落与 Run。
- `presentation-slide-canvas.tsx` 和 `presentation-file-preview.tsx` 只消费归一化后的预览模型。
- Canvas 内部按形状、段落和 Run 分层渲染，段落布局由纯样式投影生成。

XML 属性默认值和 EMU 单位转换必须经由单一读取入口。形状树节点由处理器表分派，新增 DrawingML 节点时扩展对应处理器，不在遍历层增加分支。解析层不得持有 React 状态，视图层不得重新解释 DrawingML。
