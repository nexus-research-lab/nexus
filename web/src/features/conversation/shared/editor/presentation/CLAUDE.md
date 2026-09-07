# 演示文稿预览

- `presentation-preview-model.ts` 只定义跨解析器和视图消费的预览结果与元素，不暴露解析器内部状态。
- `presentation-xml-utils.ts` 负责 ZIP 路径、关系和 DrawingML XML 基础访问。
- `presentation-pptx-parser.ts` 作为包入口编排幻灯片继承链、共享 Part 缓存和图片资源生命周期。
- `parser/` 独立负责 Shape Tree、占位符、组变换、图片和预览跳过规则。
- `presentation-shape-style.ts` 读取坐标、组变换、几何、填充和描边，并投影组内样式。
- `presentation-text-parser.ts` 只解析文本段落与 Run。
- `presentation-slide-canvas.tsx` 和 `presentation-file-preview.tsx` 只消费归一化后的预览模型；缩略图选择使用中性 `UiChoiceButton`，前后翻页使用本地化的共享 `UiIconButton`，页码、标题和等待文案使用语义 Typography，不得重建私有按钮、字号或圆角。
- Canvas 内部按形状、段落和 Run 分层渲染，段落布局由纯样式投影生成。
- Canvas 正文与缩略图使用完全相同的内容几何和文字内边距，缩略图只缩放整个 SVG 并保留外部缩略图阴影；不按缩略图重排内容。已解析段落与 Run 是不可变顺序，位置即渲染身份，允许重复正文，不得把文本内容拼成 React key。纸面圆角、源字体、形状与颜色属于文档内容，不强套 App 控件样式。
- 文件正文加载/失败复用上层状态所有者，失败面独立滚动，Header 仅投影已加载页数；缩略图、翻页和资源释放仍由本目录负责。入口消费上层 Office scope；文件/账号切换清空页码与内容，迟到的解析结果必须释放自身 object URL，不能释放当前文稿资源；卸载和替换继续释放已提交的资源。

XML 属性默认值和 EMU 单位转换必须经由单一读取入口。形状树节点由处理器表分派，新增 DrawingML 节点时扩展对应处理器，不在遍历层增加分支。解析层不得持有 React 状态，视图层不得重新解释 DrawingML。
