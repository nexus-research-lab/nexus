# Liquid Glass 资产说明

这个目录里的 `glass-switch`、`liquid-glass-panel` 和 `liquid-glass-engine` 共享同一套液态玻璃思路：

- 几何参数先定义玻璃曲面
- 位移图负责折射方向
- 高光图负责 specular / rim light
- 最终通过 SVG filter 的 `feDisplacementMap + feBlend` 落到浏览器里

## 1. 那两张 PNG 是干什么的

- `displacement` 图：
  - 给 `feDisplacementMap` 使用
  - `R/G` 通道编码 X/Y 位移向量
  - 没有它就只有模糊，不会有折射
- `specular` 图：
  - 给高光混合链使用
  - 主要靠 alpha 通道控制边缘高光分布
  - 没有它就会缺少玻璃边缘的灵动感

## 2. 当前仓库怎么生成自定义 PNG

仓库已经补了离线导出脚本：

```bash
cd /Users/leemysw/Projects/nexus/web
pnpm run export:liquid-glass -- \
  --width 146 \
  --height 92 \
  --radius 46 \
  --bezel 24 \
  --surface-profile lip \
  --light-angle -48 \
  --specular-power 2.2 \
  --specular-opacity 1 \
  --basename switch-thumb \
  --output-dir ./public/liquid-glass/generated
```

生成结果：

- `./public/liquid-glass/generated/switch-thumb-displacement.png`
- `./public/liquid-glass/generated/switch-thumb-specular.png`
- `./public/liquid-glass/generated/switch-thumb-metadata.json`

## 3. 参数含义

- `width`
  - 位移图和高光图的宽度
  - 通常应与 `feImage width` 保持一致
- `height`
  - 位移图和高光图的高度
  - 通常应与 `feImage height` 保持一致
- `radius`
  - 圆角半径
  - 对 rounded rect / switch thumb 的外轮廓影响最大
- `bezel`
  - 玻璃边缘厚度
  - 越大，折射过渡区域越宽
- `surface-profile`
  - `convex` 适合普通玻璃按钮
  - `lip` 更适合 switch 这类外圈明显、中心回收的轮廓
- `light-angle`
  - 光照方向，单位是角度
- `specular-power`
  - 高光集中度
  - 越大，高光越尖锐
- `specular-opacity`
  - 高光强度

## 4. 什么时候需要重新生成 PNG

以下参数变化时，应重新生成：

- `width`
- `height`
- `radius`
- `bezel`
- `surface-profile`
- `light-angle`
- `specular-power`
- `specular-opacity`

以下参数通常不需要重新生成 PNG，只改 SVG filter 即可：

- `feDisplacementMap scale`
- `feGaussianBlur stdDeviation`
- `feColorMatrix saturate`
- `feFuncA slope`

## 5. 在组件里怎么接

以 `glass-switch.tsx` 为例，滤镜链结构应保持如下关系：

```tsx
<feImage href="/liquid-glass/generated/switch-thumb-displacement.png" result="displacement_map" />
<feDisplacementMap in="blurred_source" in2="displacement_map" />
<feImage href="/liquid-glass/generated/switch-thumb-specular.png" result="specular_layer" />
```

资源尺寸要和 `feImage width / height` 对齐，不要一边生成 146×92，一边在滤镜里按别的尺寸读。

## 6. 当前实现的边界

- Chrome / Safari 都会尝试这条 SVG filter 路线
- Firefox 仍然不走 true liquid glass
- Safari 即使支持，也可能和 Chrome 视觉上不完全一致
- 如果要做完全一致的跨浏览器折射，下一层方案是 Canvas / WebGL，而不是继续堆 CSS
