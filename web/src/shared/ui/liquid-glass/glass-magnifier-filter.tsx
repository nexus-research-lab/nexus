interface GlassMagnifierFilterProps {
  filterId: string;
  height: number;
  width: number;
}

const MAGNIFYING_SCALE = 24;
const DISPLACEMENT_SCALE = 98.24713343067756;
const SATURATION_VALUE = 9;
const SPECULAR_FADE_SLOPE = 0.5;
const MAGNIFYING_MAP_URL = "/liquid-glass/magnifier-magnifying-map.png";
const DISPLACEMENT_MAP_URL = "/liquid-glass/magnifier-displacement-map.png";
const SPECULAR_MAP_URL = "/liquid-glass/magnifier-specular-map.png";

/** 放大镜滤镜只描述 SVG 资源链，不参与悬停动画生命周期。 */
export function GlassMagnifierFilter({
  filterId,
  height,
  width,
}: GlassMagnifierFilterProps) {
  return (
    <svg
      aria-hidden="true"
      className="hidden"
      colorInterpolationFilters="sRGB"
      focusable="false"
    >
      <defs>
        <filter id={filterId}>
          <feImage
            height={height}
            href={MAGNIFYING_MAP_URL}
            result="magnifying_displacement_map"
            width={width}
            x={0}
            y={0}
          />
          <feDisplacementMap
            in="SourceGraphic"
            in2="magnifying_displacement_map"
            result="magnified_source"
            scale={MAGNIFYING_SCALE}
            xChannelSelector="R"
            yChannelSelector="G"
          />
          <feGaussianBlur
            in="magnified_source"
            result="blurred_source"
            stdDeviation={0}
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
            scale={DISPLACEMENT_SCALE}
            xChannelSelector="R"
            yChannelSelector="G"
          />
          <feColorMatrix
            in="displaced"
            result="displaced_saturated"
            type="saturate"
            values={String(SATURATION_VALUE)}
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
            <feFuncA type="linear" slope={SPECULAR_FADE_SLOPE} />
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
