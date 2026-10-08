// INPUT: 构建身份反例和显式指定的真实 macOS helper。
// OUTPUT: 不匹配/替换构建拒绝，签名后清单可校验且修改后失败。
// POS: helper 打包门禁；真实 helper 用例不构成完整 App 签名或发布验收。
import test from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { mkdtemp, mkdir, copyFile, rm, readFile, writeFile, appendFile, symlink, unlink } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { parseBuildIdentity, writeManifest, verifyManifest } from './bootstrap-manifest.mjs'

const version = (await readFile(new URL('../../go.mod', import.meta.url), 'utf8')).match(/github\.com\/nexus-research-lab\/nexus-agent-sdk-bridge\s+(\S+)/)[1]
const info = `\tpath\tgithub.com/nexus-research-lab/nexus-agent-sdk-bridge/cmd/nexus-runtime-bootstrap\n\tmod\tgithub.com/nexus-research-lab/nexus-agent-sdk-bridge\t${version}\th1:test\n\tbuild\tCGO_ENABLED=1\n\tbuild\tGOOS=darwin\n\tbuild\tGOARCH=arm64\n`

test('reject incorrect build identity and unsupported helper builds', () => {
  assert.equal(parseBuildIdentity(info, version), 'arm64')
  for (const candidate of [info.replace(version, '(devel)'), info.replace('/cmd/nexus-runtime-bootstrap', '/cmd/other'), info.replace('CGO_ENABLED=1', 'CGO_ENABLED=0'), info.replace('GOOS=darwin', 'GOOS=linux'), info.replace('GOARCH=arm64', 'GOARCH=unknown'), `${info}\t=>\t/tmp/local\t(devel)\n`]) assert.throws(() => parseBuildIdentity(candidate, version))
  assert.throws(() => parseBuildIdentity(info, 'replacement'))
})

test('signed native helper manifest rejects binary and manifest tampering', { skip: !process.env.NEXUS_BOOTSTRAP_TEST_BINARY }, async () => {
  assert.equal(process.platform, 'darwin')
  const root = await mkdtemp(path.join(os.tmpdir(), 'nexus-helper-manifest-'))
  try {
    const bundle = path.join(root, 'Fixture.app')
    const bin = path.join(bundle, 'Contents', 'Resources', 'bin')
    await mkdir(bin, { recursive: true })
    const binary = path.join(bin, 'nexus-runtime-bootstrap')
    await copyFile(process.env.NEXUS_BOOTSTRAP_TEST_BINARY, binary)
    execFileSync('codesign', ['--force', '--sign', '-', binary], { stdio: 'pipe' })
    const manifest = await writeManifest(bundle, version)
    assert.deepEqual(await verifyManifest(bundle, version), manifest)
    await assert.rejects(writeManifest(bundle, version), /EEXIST/)
    const manifestPath = path.join(bin, '..', 'runtime-bootstrap.json')
    const original = await readFile(manifestPath)
    await writeFile(manifestPath, JSON.stringify({ ...manifest, relative_path: '../elsewhere' }))
    await assert.rejects(verifyManifest(bundle, version), /does not match/)
    await writeFile(manifestPath, original)
    await appendFile(binary, 'changed')
    await assert.rejects(verifyManifest(bundle, version), /does not match/)
    await unlink(manifestPath)
    await symlink(binary, manifestPath)
    await assert.rejects(verifyManifest(bundle, version))
  } finally { await rm(root, { recursive: true, force: true }) }
})
