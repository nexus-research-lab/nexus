interface GlassSwitchFilterProps {
  filterId: string;
  height: number;
  width: number;
}

const FILTER_BLUR = 0.2;
const FILTER_SATURATION = "6";
const FILTER_SPECULAR_FADE = 0.5;
const FILTER_DISPLACEMENT_SCALE = 22.26064761799501;
const DISPLACEMENT_MAP_URL = "/liquid-glass/displacement-map.png";
const SPECULAR_MAP_URL = "/liquid-glass/specular-map.png";

/** 滤镜资源与渲染节点独立，开关组件只负责交互和几何投影。 */
export function GlassSwitchFilter({
  filterId,
  height,
  width,
}: GlassSwitchFilterProps) {
  return (
    <svg
      aria-hidden="true"
      className="pointer-events-none absolute h-0 w-0 overflow-hidden"
      colorInterpolationFilters="sRGB"
      focusable="false"
    >
      <defs>
        <filter id={filterId}>
          <feGaussianBlur
            in="SourceGraphic"
            result="blurred_source"
            stdDeviation={FILTER_BLUR}
          />
          <feImage
            height={height}
            href={DISPLACEMENT_MAP_URL}
            result="displacement_map"
            width={width}
            x={0}
            y={0}
          />
          <feDisplacementMap
            in="blurred_source"
            in2="displacement_map"
            result="displaced"
            scale={FILTER_DISPLACEMENT_SCALE}
            xChannelSelector="R"
            yChannelSelector="G"
          />
          <feColorMatrix
            in="displaced"
            result="displaced_saturated"
            type="saturate"
            values={FILTER_SATURATION}
          />
          <feImage
            height={height}
            href={SPECULAR_MAP_URL}
            result="specular_layer"
            width={width}
            x={0}
            y={0}
          />
          <feComposite
            in="displaced_saturated"
            in2="specular_layer"
            operator="in"
            result="specular_saturated"
          />
          <feComponentTransfer in="specular_layer" result="specular_faded">
            <feFuncA type="linear" slope={FILTER_SPECULAR_FADE} />
          </feComponentTransfer>
          <feBlend
            in="specular_saturated"
            in2="displaced"
            mode="normal"
            result="with_saturation"
          />
          <feBlend in="specular_faded" in2="with_saturation" mode="normal" />
        </filter>
      </defs>
    </svg>
  );
}
