# INPUT: 已核验临时账号 fixture、本地 SDK 源码与独立报告目录。
# OUTPUT: 固定十项 LogonW 组合的真实退出码、逐项结果和可追溯二进制摘要。
# POS: Windows P3 原生实验；任何失败/skip/缺失均非验收通过，不启用产品能力。
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$SDKSource,
    [Parameter(Mandatory)][string]$FixtureRoot,
    [Parameter(Mandatory)][string]$ReportDirectory
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
if ([Security.Principal.WindowsPrincipal]::new($identity).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'Run the comparison as the ordinary host owner, not an elevated administrator.'
}
$FixtureRoot = [IO.Path]::GetFullPath($FixtureRoot).TrimEnd('\')
$fixtureBase = [IO.Path]::GetFullPath((Join-Path $env:ProgramData 'NexusSandboxTests'))
if ((Split-Path -Parent $FixtureRoot) -ine $fixtureBase -or (Split-Path -Leaf $FixtureRoot) -notmatch '^[a-f0-9]{32}$') {
    throw 'FixtureRoot must be the dedicated GUID child of ProgramData\NexusSandboxTests.'
}
$manifestPath = Join-Path $FixtureRoot 'control\fixture.json'
foreach ($path in @($fixtureBase, $FixtureRoot, (Join-Path $FixtureRoot 'control'), (Join-Path $FixtureRoot 'bin'), $manifestPath)) {
    if (((Get-Item -LiteralPath $path -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw 'Fixture path contains a reparse point.' }
}
$manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
if ($manifest.version -ne 1 -or $manifest.ownerSid -ne $identity.User.Value -or $manifest.fixtureID -ne (Split-Path -Leaf $FixtureRoot)) {
    throw 'Fixture owner or generation does not match.'
}
$account = Get-LocalUser -Name $manifest.accountName
if ($account.SID.Value -ne $manifest.accountSid -or $account.Description -ne "Nexus test $($manifest.fixtureID)") {
    throw 'Fixture account identity changed.'
}
$SDKSource = (Resolve-Path -LiteralPath $SDKSource).Path
$ReportDirectory = [IO.Path]::GetFullPath($ReportDirectory)
if (Test-Path -LiteralPath $ReportDirectory) { throw 'Use a new report directory to preserve previous evidence.' }
New-Item -ItemType Directory -Path $ReportDirectory | Out-Null
$names = @('powershell', 'descendant', 'named_event', 'runner_process_and_token', 'runner_thread_boundary',
    'runner_future_thread_boundary', 'host_process_and_token', 'host_thread_boundary', 'inherited_handle_boundary', 'filesystem_write_scope')
$tests = @('TestWindowsCodexLogonBootstrap', 'TestWindowsCapabilityOnlyLogonBootstrap', 'TestWindowsCapabilityLogonBootstrap')
$savedEnvironment = @{}
foreach ($key in @('GOWORK', 'NEXUS_WINDOWS_TEST_ACCOUNT', 'NEXUS_WINDOWS_TEST_PASSWORD', 'NEXUS_WINDOWS_TEST_FIXTURE')) {
    $savedEnvironment[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
}
$credential = $null
$results = @()
try {
    $env:GOWORK = 'off'
    Push-Location $SDKSource
    try {
        $sourceCommit = (& git rev-parse HEAD).Trim()
        if ($LASTEXITCODE -ne 0) { throw 'SDK source must be a Git checkout.' }
        $sourceDirty = @(& git status --porcelain).Count -ne 0
        $builtBinary = Join-Path $ReportDirectory 'sandbox-components.test.exe'
        & go test -c -o $builtBinary ./internal/tool/builtin/bash/sandboxexec *> (Join-Path $ReportDirectory 'build.log')
        if ($LASTEXITCODE -ne 0) { throw 'Native SDK test build failed; see build.log.' }
    } finally { Pop-Location }
    $binary = Join-Path $FixtureRoot 'bin\sandbox-components.test.exe'
    Copy-Item -LiteralPath $builtBinary -Destination $binary
    $binaryHash = (Get-FileHash -LiteralPath $builtBinary -Algorithm SHA256).Hash.ToLowerInvariant()
    if ((Get-FileHash -LiteralPath $binary -Algorithm SHA256).Hash.ToLowerInvariant() -ne $binaryHash) { throw 'Fixture test binary copy changed.' }
    $manifest.binarySHA256 = $binaryHash
    $manifest | ConvertTo-Json | Set-Content -LiteralPath $manifestPath -Encoding utf8
    $credential = Import-Clixml -LiteralPath (Join-Path $FixtureRoot 'control\credential.xml')
    if ($credential.UserName -ne ".\$($manifest.accountName)") { throw 'Fixture credential identity changed.' }
    $env:NEXUS_WINDOWS_TEST_ACCOUNT = $manifest.accountName
    $env:NEXUS_WINDOWS_TEST_PASSWORD = $credential.GetNetworkCredential().Password
    $env:NEXUS_WINDOWS_TEST_FIXTURE = $FixtureRoot
    foreach ($test in $tests) {
        $logName = $test + '.log'
        $logPath = Join-Path $ReportDirectory $logName
        & $binary "-test.run=^$test`$" '-test.v' '-test.timeout=3m' *> $logPath
        $testExit = $LASTEXITCODE
        $body = Get-Content -LiteralPath $logPath -Raw
        $checks = @()
        foreach ($name in $names) {
            $pattern = '(?m)^\s*--- (PASS|FAIL|SKIP): ' + [regex]::Escape("$test/$name") + ' \('
            $matches = [regex]::Matches($body, $pattern)
            $state = if ($matches.Count -eq 1) { $matches[0].Groups[1].Value.ToLowerInvariant() } else { 'missing_or_ambiguous' }
            $checks += @{ name = $name; result = $state }
        }
        $passed = $testExit -eq 0 -and @($checks | Where-Object { $_.result -ne 'pass' }).Count -eq 0
        $results += @{ test = $test; exitCode = $testExit; passed = $passed; checks = $checks; log = $logName;
            logSHA256 = (Get-FileHash -LiteralPath $logPath -Algorithm SHA256).Hash.ToLowerInvariant() }
        Write-Output "$test exit=$testExit verified=$passed"
    }
    @{
        schemaVersion = 1; capturedAt = [DateTime]::UtcNow.ToString('o'); sdkCommit = $sourceCommit
        sourceDirty = $sourceDirty; binarySHA256 = $binaryHash; platform = [Environment]::OSVersion.VersionString
        hostElevated = $false; results = $results; releaseAccepted = $false
    } | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $ReportDirectory 'report.json') -Encoding utf8
} finally {
    foreach ($key in $savedEnvironment.Keys) { [Environment]::SetEnvironmentVariable($key, $savedEnvironment[$key], 'Process') }
    $credential = $null
}
if (@($results | Where-Object { -not $_.passed }).Count -ne 0) { exit 1 }
