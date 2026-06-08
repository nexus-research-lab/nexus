#!/usr/bin/env node
"use strict";

const crypto = require("crypto");
const fs = require("fs");
const http = require("http");
const https = require("https");
const os = require("os");
const path = require("path");
const zlib = require("zlib");

const DEFAULT_RELEASE = "nxs-v0.1.1";
const DEFAULT_REPO = "nexus-research-lab/nexus-agent-sdk-bridge";
const USER_AGENT = "nexus-desktop-nxs-fetcher";

async function main() {
  const args = parseArgs(process.argv.slice(2));
  if (args.help) {
    printHelp();
    return;
  }

  const repo = args.repo || env("NEXUS_DESKTOP_NXS_RELEASE_REPO") || DEFAULT_REPO;
  const release = normalizeRelease(
    args.release ||
      env("NEXUS_DESKTOP_NXS_RELEASE") ||
      env("NEXUS_NXS_RUNTIME_RELEASE") ||
      DEFAULT_RELEASE,
  );
  const manifestURL =
    args.manifestURL ||
    env("NEXUS_DESKTOP_NXS_MANIFEST_URL") ||
    env("NEXUS_NXS_RUNTIME_MANIFEST_URL") ||
    `https://github.com/${repo}/releases/download/${release}/nxs-manifest.json`;
  const goos = args.goos || env("NEXUS_DESKTOP_NXS_GOOS") || nodePlatformToGOOS(process.platform);
  const goarch = args.goarch || env("NEXUS_DESKTOP_NXS_GOARCH") || nodeArchToGOARCH(process.arch);
  if (!args.output) {
    throw new Error("--output is required");
  }
  if (!goos || !goarch) {
    throw new Error(`unsupported platform ${process.platform}/${process.arch}; pass --goos and --goarch`);
  }

  const client = new GitHubReleaseClient({
    token:
      env("NEXUS_DESKTOP_NXS_DOWNLOAD_TOKEN") ||
      env("NEXUS_NXS_RUNTIME_GITHUB_TOKEN") ||
      env("GH_TOKEN") ||
      env("GITHUB_TOKEN"),
    repo,
    release,
  });
  const manifest = JSON.parse((await client.downloadManifest(manifestURL)).toString("utf8"));
  const asset = selectAsset(manifest, goos, goarch);
  const archiveBytes = await client.downloadAsset(asset.url, asset.filename);
  verifySHA256(archiveBytes, asset.sha256, asset.filename);
  const executableName = goos === "windows" ? "nxs.exe" : "nxs";
  const runtimeBytes = extractExecutable(archiveBytes, archiveKind(asset), executableName);

  fs.mkdirSync(path.dirname(args.output), { recursive: true });
  fs.writeFileSync(args.output, runtimeBytes, { mode: 0o755 });
  fs.chmodSync(args.output, 0o755);

  console.log(`nxs runtime: ${args.output}`);
  console.log(`version: ${manifest.version || release}`);
  console.log(`asset: ${asset.filename}`);
}

function env(name) {
  return String(process.env[name] || "").trim();
}

function parseArgs(values) {
  const args = {};
  for (let index = 0; index < values.length; index += 1) {
    const value = values[index];
    switch (value) {
      case "--help":
      case "-h":
        args.help = true;
        break;
      case "--output":
        args.output = requireValue(values, ++index, value);
        break;
      case "--goos":
        args.goos = requireValue(values, ++index, value);
        break;
      case "--goarch":
        args.goarch = requireValue(values, ++index, value);
        break;
      case "--release":
        args.release = requireValue(values, ++index, value);
        break;
      case "--repo":
        args.repo = requireValue(values, ++index, value);
        break;
      case "--manifest-url":
        args.manifestURL = requireValue(values, ++index, value);
        break;
      default:
        throw new Error(`unknown argument: ${value}`);
    }
  }
  return args;
}

function requireValue(values, index, flag) {
  const value = values[index];
  if (!value || value.startsWith("--")) {
    throw new Error(`${flag} requires a value`);
  }
  return value;
}

function printHelp() {
  console.log(`Usage: node scripts/desktop/fetch-nxs-runtime.js --output <path> [--goos darwin] [--goarch arm64]

Downloads the nxs runtime from the bridge release manifest, verifies sha256, and writes the executable.

Environment:
  NEXUS_DESKTOP_NXS_RELEASE          Release tag, default ${DEFAULT_RELEASE}
  NEXUS_DESKTOP_NXS_RELEASE_REPO     Release repo, default ${DEFAULT_REPO}
  NEXUS_DESKTOP_NXS_MANIFEST_URL     Manifest URL override
  NEXUS_DESKTOP_NXS_DOWNLOAD_TOKEN   Optional GitHub token for private release assets
`);
}

function normalizeRelease(value) {
  const trimmed = String(value || "").trim();
  if (!trimmed) {
    return DEFAULT_RELEASE;
  }
  if (trimmed.startsWith("nxs-v")) {
    return trimmed;
  }
  if (trimmed.startsWith("v")) {
    return `nxs-${trimmed}`;
  }
  return `nxs-v${trimmed}`;
}

function nodePlatformToGOOS(platform) {
  switch (platform) {
    case "darwin":
      return "darwin";
    case "linux":
      return "linux";
    case "win32":
      return "windows";
    default:
      return "";
  }
}

function nodeArchToGOARCH(arch) {
  switch (arch) {
    case "x64":
      return "amd64";
    case "arm64":
      return "arm64";
    default:
      return "";
  }
}

class GitHubReleaseClient {
  constructor({ token, repo, release }) {
    this.token = token;
    this.repo = repo;
    this.release = release;
    this.releaseAssets = null;
  }

  async downloadManifest(manifestURL) {
    try {
      return await this.downloadURL(manifestURL);
    } catch (error) {
      if (!this.token || error.statusCode !== 404) {
        throw error;
      }
      const asset = await this.findReleaseAsset("nxs-manifest.json");
      return this.downloadURL(asset.url, "application/octet-stream");
    }
  }

  async downloadAsset(assetURL, filename) {
    try {
      return await this.downloadURL(assetURL);
    } catch (error) {
      if (!this.token || error.statusCode !== 404) {
        throw error;
      }
      const asset = await this.findReleaseAsset(filename);
      return this.downloadURL(asset.url, "application/octet-stream");
    }
  }

  async findReleaseAsset(filename) {
    const assets = await this.listReleaseAssets();
    const asset = assets.find((candidate) => candidate.name === filename);
    if (!asset) {
      throw new Error(`release ${this.release} has no asset ${filename}`);
    }
    return asset;
  }

  async listReleaseAssets() {
    if (this.releaseAssets) {
      return this.releaseAssets;
    }
    const metadataURL = `https://api.github.com/repos/${this.repo}/releases/tags/${this.release}`;
    const metadata = JSON.parse((await this.downloadURL(metadataURL, "application/vnd.github+json")).toString("utf8"));
    this.releaseAssets = Array.isArray(metadata.assets) ? metadata.assets : [];
    return this.releaseAssets;
  }

  async downloadURL(rawURL, accept = "application/octet-stream") {
    return downloadURL(rawURL, {
      accept,
      token: this.token,
      redirectsRemaining: 5,
    });
  }
}

function selectAsset(manifest, goos, goarch) {
  const assets = Array.isArray(manifest.assets) ? manifest.assets : [];
  const asset = assets.find((candidate) => candidate.goos === goos && candidate.goarch === goarch);
  if (!asset) {
    throw new Error(`nxs runtime asset ${goos}-${goarch} is not available`);
  }
  if (!asset.url) {
    throw new Error(`nxs runtime asset ${asset.filename || `${goos}-${goarch}`} has no url`);
  }
  if (!asset.sha256) {
    throw new Error(`nxs runtime asset ${asset.filename || `${goos}-${goarch}`} has no sha256`);
  }
  return asset;
}

function downloadURL(rawURL, options) {
  const url = new URL(rawURL);
  const transport = url.protocol === "http:" ? http : https;
  const headers = {
    Accept: options.accept,
    "User-Agent": USER_AGENT,
  };
  if (options.token && isGitHubHost(url.hostname)) {
    headers.Authorization = `Bearer ${options.token}`;
  }
  return new Promise((resolve, reject) => {
    const request = transport.get(url, { headers }, (response) => {
      const statusCode = response.statusCode || 0;
      const location = response.headers.location;
      if (statusCode >= 300 && statusCode < 400 && location) {
        response.resume();
        if (options.redirectsRemaining <= 0) {
          reject(new Error(`too many redirects while downloading ${rawURL}`));
          return;
        }
        downloadURL(new URL(location, url).toString(), {
          ...options,
          redirectsRemaining: options.redirectsRemaining - 1,
        })
          .then(resolve)
          .catch(reject);
        return;
      }

      const chunks = [];
      response.on("data", (chunk) => chunks.push(chunk));
      response.on("end", () => {
        const body = Buffer.concat(chunks);
        if (statusCode < 200 || statusCode >= 300) {
          const error = new Error(`unexpected http status ${statusCode} for ${rawURL}`);
          error.statusCode = statusCode;
          error.body = body.toString("utf8", 0, Math.min(body.length, 300));
          reject(error);
          return;
        }
        resolve(body);
      });
    });
    request.on("error", reject);
  });
}

function isGitHubHost(hostname) {
  return hostname === "github.com" || hostname === "api.github.com";
}

function verifySHA256(data, expected, filename) {
  const actual = crypto.createHash("sha256").update(data).digest("hex");
  if (actual.toLowerCase() !== String(expected).trim().toLowerCase()) {
    throw new Error(`${filename} sha256 mismatch: got ${actual}, want ${expected}`);
  }
}

function archiveKind(asset) {
  if (asset.archive) {
    return asset.archive;
  }
  const filename = String(asset.filename || "").toLowerCase();
  if (filename.endsWith(".tar.gz")) {
    return "tar.gz";
  }
  if (filename.endsWith(".tgz")) {
    return "tgz";
  }
  if (filename.endsWith(".zip")) {
    return "zip";
  }
  return "raw";
}

function extractExecutable(data, archive, executableName) {
  switch (archive) {
    case "raw":
      return data;
    case "tar.gz":
    case "tgz":
      return extractTarGzip(data, executableName);
    case "zip":
      return extractZip(data, executableName);
    default:
      throw new Error(`unsupported nxs runtime archive type ${archive}`);
  }
}

function extractTarGzip(data, executableName) {
  const tarData = zlib.gunzipSync(data);
  let offset = 0;
  while (offset + 512 <= tarData.length) {
    const header = tarData.subarray(offset, offset + 512);
    if (header.every((value) => value === 0)) {
      break;
    }
    const name = readTarString(header, 0, 100);
    const prefix = readTarString(header, 345, 155);
    const fullName = prefix ? `${prefix}/${name}` : name;
    const size = Number.parseInt(readTarString(header, 124, 12).trim() || "0", 8);
    const typeFlag = String.fromCharCode(header[156] || 0);
    const contentStart = offset + 512;
    const contentEnd = contentStart + size;
    if ((typeFlag === "0" || typeFlag === "\0") && path.posix.basename(fullName) === executableName) {
      return tarData.subarray(contentStart, contentEnd);
    }
    offset = contentStart + Math.ceil(size / 512) * 512;
  }
  throw new Error(`nxs executable ${executableName} not found in tar.gz`);
}

function readTarString(buffer, start, length) {
  const end = start + length;
  const zero = buffer.indexOf(0, start);
  return buffer.toString("utf8", start, zero >= start && zero < end ? zero : end);
}

function extractZip(data, executableName) {
  const eocdOffset = findEndOfCentralDirectory(data);
  const entryCount = data.readUInt16LE(eocdOffset + 10);
  let centralOffset = data.readUInt32LE(eocdOffset + 16);
  for (let index = 0; index < entryCount; index += 1) {
    if (data.readUInt32LE(centralOffset) !== 0x02014b50) {
      throw new Error("invalid zip central directory");
    }
    const compressionMethod = data.readUInt16LE(centralOffset + 10);
    const compressedSize = data.readUInt32LE(centralOffset + 20);
    const uncompressedSize = data.readUInt32LE(centralOffset + 24);
    const filenameLength = data.readUInt16LE(centralOffset + 28);
    const extraLength = data.readUInt16LE(centralOffset + 30);
    const commentLength = data.readUInt16LE(centralOffset + 32);
    const localOffset = data.readUInt32LE(centralOffset + 42);
    const filename = data.toString("utf8", centralOffset + 46, centralOffset + 46 + filenameLength);
    if (path.posix.basename(filename.replace(/\\/g, "/")) === executableName) {
      return readZipEntry(data, localOffset, compressionMethod, compressedSize, uncompressedSize);
    }
    centralOffset += 46 + filenameLength + extraLength + commentLength;
  }
  throw new Error(`nxs executable ${executableName} not found in zip`);
}

function findEndOfCentralDirectory(data) {
  const minOffset = Math.max(0, data.length - 65557);
  for (let offset = data.length - 22; offset >= minOffset; offset -= 1) {
    if (data.readUInt32LE(offset) === 0x06054b50) {
      return offset;
    }
  }
  throw new Error("invalid zip: end of central directory not found");
}

function readZipEntry(data, localOffset, compressionMethod, compressedSize, uncompressedSize) {
  if (data.readUInt32LE(localOffset) !== 0x04034b50) {
    throw new Error("invalid zip local header");
  }
  const filenameLength = data.readUInt16LE(localOffset + 26);
  const extraLength = data.readUInt16LE(localOffset + 28);
  const contentStart = localOffset + 30 + filenameLength + extraLength;
  const compressed = data.subarray(contentStart, contentStart + compressedSize);
  let output;
  switch (compressionMethod) {
    case 0:
      output = compressed;
      break;
    case 8:
      output = zlib.inflateRawSync(compressed);
      break;
    default:
      throw new Error(`unsupported zip compression method ${compressionMethod}`);
  }
  if (uncompressedSize !== 0xffffffff && output.length !== uncompressedSize) {
    throw new Error(`zip entry size mismatch: got ${output.length}, want ${uncompressedSize}`);
  }
  return output;
}

main().catch((error) => {
  console.error(`fetch nxs runtime failed: ${error.message}`);
  process.exit(1);
});
