// INPUT: 已组装/签名的 helper、固定 Bridge 模块版本和 App 根。
// OUTPUT: 签名后内容摘要与构建身份清单，或完整一致性校验失败。
// POS: App 封签前生成、打包前核验；清单自身真实性依赖 App 签名封装。
import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { constants } from 'node:fs'
import { open, writeFile, lstat } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const modulePath = 'github.com/nexus-research-lab/nexus-agent-sdk-bridge'
const relativePath = 'bin/nexus-runtime-bootstrap'
const manifestName = 'runtime-bootstrap.json'

export function parseBuildIdentity(text, expectedVersion) {
  if (!/^v\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/.test(expectedVersion)) throw new Error('canonical Bridge version required')
  const lines = text.split('\n').map(line => line.trim().split(/\s+/))
  const entries = lines.filter(fields => fields[0] === 'path')
  if (entries.length !== 1 || entries[0][1] !== `${modulePath}/cmd/nexus-runtime-bootstrap`) throw new Error('wrong bootstrap command entrypoint')
  const modules = lines.filter(fields => fields[0] === 'mod')
  if (modules.length !== 1 || modules[0][1] !== modulePath || modules[0][2] !== expectedVersion || lines.some(fields => fields[0] === '=>')) throw new Error('helper Bridge build identity mismatch')
  const build = Object.fromEntries(lines.filter(fields => fields[0] === 'build').map(fields => fields.slice(1).join(' ').split('=')))
  if (build.CGO_ENABLED !== '1' || build.GOOS !== 'darwin' || !['arm64', 'amd64'].includes(build.GOARCH)) throw new Error('helper must be a native macOS cgo build')
  return build.GOARCH
}

async function inspectHelper(bundle, expectedVersion) {
  const resources = path.join(bundle, 'Contents', 'Resources')
  for (const directory of [bundle, path.join(bundle, 'Contents'), resources, path.join(resources, 'bin')]) {
    if (!(await lstat(directory)).isDirectory()) throw new Error('bundle helper directory must not be a link')
  }
  const helper = path.join(resources, relativePath)
  const file = await open(helper, constants.O_RDONLY | constants.O_NOFOLLOW)
  try {
    const before = await file.stat()
    if (!before.isFile() || before.nlink !== 1 || !(before.mode & 0o111)) throw new Error('helper must be a single-link executable file')
    const bytes = await file.readFile()
    const architecture = parseBuildIdentity(execFileSync('go', ['version', '-m', helper], { encoding: 'utf8', env: { ...process.env, GOWORK: 'off' }, maxBuffer: 1024 * 1024 }), expectedVersion)
    const after = await lstat(helper)
    if (!after.isFile() || before.ino !== after.ino || before.dev !== after.dev || before.size !== after.size || before.mtimeMs !== after.mtimeMs) throw new Error('helper changed during verification')
    return { version: 1, relative_path: relativePath, bridge_version: expectedVersion, architecture, sha256: createHash('sha256').update(bytes).digest('hex') }
  } finally { await file.close() }
}

export async function writeManifest(bundle, expectedVersion) {
  const manifest = await inspectHelper(bundle, expectedVersion)
  // 组装目录的新文件，不覆盖旧清单或跟随链接；重新构建先重建 App bundle。
  await writeFile(path.join(bundle, 'Contents', 'Resources', manifestName), `${JSON.stringify(manifest, null, 2)}\n`, { flag: 'wx', mode: 0o644 })
  return manifest
}

export async function verifyManifest(bundle, expectedVersion) {
  const target = path.join(bundle, 'Contents', 'Resources', manifestName)
  const file = await open(target, constants.O_RDONLY | constants.O_NOFOLLOW)
  let manifest
  try {
    const stat = await file.stat()
    if (!stat.isFile() || stat.nlink !== 1 || stat.size > 4096) throw new Error('invalid helper manifest file')
    const bytes = Buffer.alloc(4097)
    const { bytesRead } = await file.read(bytes, 0, bytes.length, 0)
    if (bytesRead > 4096) throw new Error('helper manifest exceeds limit')
    try { manifest = JSON.parse(bytes.subarray(0, bytesRead).toString('utf8')) }
    catch { throw new Error('invalid helper manifest JSON') }
    if (!manifest || typeof manifest !== 'object' || Array.isArray(manifest)) throw new Error('invalid helper manifest object')
  } finally { await file.close() }
  const expected = await inspectHelper(bundle, expectedVersion)
  if (JSON.stringify(Object.keys(manifest).sort()) !== JSON.stringify(Object.keys(expected).sort()) || Object.keys(expected).some(key => manifest[key] !== expected[key])) throw new Error('helper manifest does not match signed binary')
  return manifest
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const [mode, bundle, version] = process.argv.slice(2)
  if (!['write', 'verify'].includes(mode) || !bundle || !version || process.argv.length !== 5) throw new Error('usage: bootstrap-manifest.mjs write|verify APP BRIDGE_VERSION')
  const result = await (mode === 'write' ? writeManifest : verifyManifest)(path.resolve(bundle), version)
  process.stdout.write(`${JSON.stringify(result)}\n`)
}
