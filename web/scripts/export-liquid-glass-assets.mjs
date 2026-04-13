/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：export-liquid-glass-assets.mjs
# @Date   ：2026-04-12 23:44
# @Author ：leemysw
# 2026-04-12 23:44   Create
# =====================================================
*/

import fs from "node:fs";
import path from "node:path";

import { PNG } from "pngjs";

class LiquidGlassAssetExporter {
  constructor(options) {
    this.options = options;
  }

  static clamp(value, min, max) {
    return Math.min(Math.max(value, min), max);
  }

  static smootherstep(value) {
    const normalized = LiquidGlassAssetExporter.clamp(value, 0, 1);
    return normalized * normalized * normalized * (normalized * (normalized * 6 - 15) + 10);
  }

  static squircleSurfaceProfile(value) {
    const normalized = LiquidGlassAssetExporter.clamp(value, 0, 1);
    return Math.pow(1 - Math.pow(1 - normalized, 4), 0.25);
  }

  static lipSurfaceProfile(value) {
    const normalized = LiquidGlassAssetExporter.clamp(value, 0, 1);
    const convex = LiquidGlassAssetExporter.squircleSurfaceProfile(1 - normalized);
    const concave = LiquidGlassAssetExporter.squircleSurfaceProfile(normalized);
    const blend = LiquidGlassAssetExporter.smootherstep(normalized);
    return convex * (1 - blend) - concave * blend * 0.28;
  }

  static getRoundedRectSdf(x, y, width, height, radius) {
    const halfWidth = width / 2;
    const halfHeight = height / 2;
    const dx = Math.abs(x - halfWidth) - (halfWidth - radius);
    const dy = Math.abs(y - halfHeight) - (halfHeight - radius);
    const outerX = Math.max(dx, 0);
    const outerY = Math.max(dy, 0);
    return Math.hypot(outerX, outerY) + Math.min(Math.max(dx, dy), 0) - radius;
  }

  static getSdfNormal(x, y, width, height, radius) {
    const epsilon = 0.85;
    const dx = LiquidGlassAssetExporter.getRoundedRectSdf(x + epsilon, y, width, height, radius)
      - LiquidGlassAssetExporter.getRoundedRectSdf(x - epsilon, y, width, height, radius);
    const dy = LiquidGlassAssetExporter.getRoundedRectSdf(x, y + epsilon, width, height, radius)
      - LiquidGlassAssetExporter.getRoundedRectSdf(x, y - epsilon, width, height, radius);
    const length = Math.hypot(dx, dy);

    if (length < 0.0001) {
      return { x: 0, y: -1 };
    }

    return {
      x: dx / length,
      y: dy / length,
    };
  }

  static encodeVectorChannel(value) {
    return Math.round(LiquidGlassAssetExporter.clamp(128 + value * 127, 0, 255));
  }

  createDisplacementAndSpecularPng() {
    const {
      width,
      height,
      radius,
      bezel,
      surfaceProfile,
      lightAngleDeg,
      specularPower,
      specularOpacity,
    } = this.options;
    const displacement = new PNG({ width, height });
    const specular = new PNG({ width, height });
    const lightRadians = lightAngleDeg * (Math.PI / 180);
    const lightDirection = {
      x: Math.cos(lightRadians),
      y: Math.sin(lightRadians),
    };

    for (let y = 0; y < height; y += 1) {
      for (let x = 0; x < width; x += 1) {
        const pixelIndex = (y * width + x) * 4;
        const signedDistance = LiquidGlassAssetExporter.getRoundedRectSdf(
          x + 0.5,
          y + 0.5,
          width,
          height,
          radius,
        );

        displacement.data[pixelIndex] = 128;
        displacement.data[pixelIndex + 1] = 128;
        displacement.data[pixelIndex + 2] = 128;
        displacement.data[pixelIndex + 3] = 255;
        specular.data[pixelIndex] = 255;
        specular.data[pixelIndex + 1] = 255;
        specular.data[pixelIndex + 2] = 255;
        specular.data[pixelIndex + 3] = 0;

        if (signedDistance > 0) {
          continue;
        }

        const distanceFromEdge = -signedDistance;
        if (distanceFromEdge > bezel * 1.18) {
          continue;
        }

        const outwardNormal = LiquidGlassAssetExporter.getSdfNormal(
          x + 0.5,
          y + 0.5,
          width,
          height,
          radius,
        );
        const inwardNormal = { x: -outwardNormal.x, y: -outwardNormal.y };
        const normalizedBezelPosition = LiquidGlassAssetExporter.clamp(distanceFromEdge / bezel, 0, 1);
        const profileStrength = surfaceProfile === "lip"
          ? LiquidGlassAssetExporter.lipSurfaceProfile(normalizedBezelPosition)
          : LiquidGlassAssetExporter.squircleSurfaceProfile(1 - normalizedBezelPosition);
        const displacementStrength = profileStrength * (0.82 + (1 - normalizedBezelPosition) * 0.18);

        displacement.data[pixelIndex] = LiquidGlassAssetExporter.encodeVectorChannel(
          inwardNormal.x * displacementStrength,
        );
        displacement.data[pixelIndex + 1] = LiquidGlassAssetExporter.encodeVectorChannel(
          inwardNormal.y * displacementStrength,
        );

        const lightFacing = Math.max(
          0,
          outwardNormal.x * lightDirection.x + outwardNormal.y * lightDirection.y,
        );
        const rimStrength = Math.pow(1 - normalizedBezelPosition, 2.35);
        const diffuseGlow = Math.pow(1 - normalizedBezelPosition, 3.8) * 0.18;
        const highlightAlpha = LiquidGlassAssetExporter.clamp(
          (Math.pow(lightFacing, specularPower) * rimStrength + diffuseGlow)
            * specularOpacity
            * 255,
          0,
          255,
        );
        specular.data[pixelIndex + 3] = Math.round(highlightAlpha);
      }
    }

    return { displacement, specular };
  }

  export() {
    const { outputDir, basename } = this.options;
    const displacementPath = path.join(outputDir, `${basename}-displacement.png`);
    const specularPath = path.join(outputDir, `${basename}-specular.png`);
    const metadataPath = path.join(outputDir, `${basename}-metadata.json`);
    const { displacement, specular } = this.createDisplacementAndSpecularPng();

    fs.mkdirSync(outputDir, { recursive: true });
    fs.writeFileSync(displacementPath, PNG.sync.write(displacement));
    fs.writeFileSync(specularPath, PNG.sync.write(specular));
    fs.writeFileSync(
      metadataPath,
      `${JSON.stringify(
        {
          width: this.options.width,
          height: this.options.height,
          radius: this.options.radius,
          bezel: this.options.bezel,
          surface_profile: this.options.surfaceProfile,
          light_angle_deg: this.options.lightAngleDeg,
          specular_power: this.options.specularPower,
          specular_opacity: this.options.specularOpacity,
        },
        null,
        2,
      )}\n`,
    );

    console.log(`displacement: ${displacementPath}`);
    console.log(`specular: ${specularPath}`);
    console.log(`metadata: ${metadataPath}`);
  }
}

function parseNumber(value, key) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    throw new Error(`参数 ${key} 必须是数字，当前值：${value}`);
  }
  return parsed;
}

function parseArguments(argv) {
  const defaults = {
    width: 146,
    height: 92,
    radius: 46,
    bezel: 24,
    surfaceProfile: "lip",
    lightAngleDeg: -48,
    specularPower: 2.2,
    specularOpacity: 1,
    outputDir: "./public/liquid-glass/generated",
    basename: "glass",
  };
  const options = { ...defaults };

  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    const value = argv[index + 1];

    if (!token.startsWith("--")) {
      continue;
    }

    if (value === undefined) {
      throw new Error(`参数 ${token} 缺少值`);
    }

    switch (token) {
      case "--width":
        options.width = parseNumber(value, token);
        break;
      case "--height":
        options.height = parseNumber(value, token);
        break;
      case "--radius":
        options.radius = parseNumber(value, token);
        break;
      case "--bezel":
        options.bezel = parseNumber(value, token);
        break;
      case "--surface-profile":
        if (value !== "convex" && value !== "lip") {
          throw new Error(`参数 ${token} 仅支持 convex 或 lip，当前值：${value}`);
        }
        options.surfaceProfile = value;
        break;
      case "--light-angle":
        options.lightAngleDeg = parseNumber(value, token);
        break;
      case "--specular-power":
        options.specularPower = parseNumber(value, token);
        break;
      case "--specular-opacity":
        options.specularOpacity = parseNumber(value, token);
        break;
      case "--output-dir":
        options.outputDir = value;
        break;
      case "--basename":
        options.basename = value;
        break;
      default:
        throw new Error(`未知参数：${token}`);
    }

    index += 1;
  }

  return options;
}

function main() {
  const options = parseArguments(process.argv.slice(2));
  new LiquidGlassAssetExporter(options).export();
}

main();
