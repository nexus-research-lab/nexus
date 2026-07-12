# 演示文稿预览

- `presentation-preview-model.ts` 定义预览结果、元素和解析上下文，不读取 XML 或 React 状态。
- `presentation-xml-utils.ts` 负责 ZIP 路径、关系和 DrawingML XML 基础访问。
- `presentation-pptx-parser.ts` 分阶段编排幻灯片继承链、形状投影、跳过规则和图片资源生命周期。
- `presentation-shape-style.ts` 读取坐标、组变换、几何、填充和描边，并投影组内样式。
- `presentation-text-parser.ts` 只解析文本段落与 Run。
- `presentation-slide-canvas.tsx` 和 `presentation-file-preview.tsx` 只消费归一化后的预览模型。

XML 属性默认值和 EMU 单位转换必须经由单一读取入口。解析层不得持有 React 状态，视图层不得重新解释 DrawingML。
