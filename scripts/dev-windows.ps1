#requires -Version 7.0
[CmdletBinding()]
param(
    [ValidateSet('dev','dev-nxs','run-control','run-backend','run-web','prepare-dev-runtime-cli','install')]
    [string]$Action = 'dev',
    [int]$BackendPort = 8010, [int]$WebPort = 3000, [int]$ControlPort = 8020,
    [string]$ControlRoot = (Join-Path $PSScriptRoot '../../nexus-control'),
    [string]$ControlDataDir,
    [string]$RuntimePath,
    [string]$CliBinDir = (Join-Path $PSScriptRoot '../cache/dev-runtime-cli'),
    [string]$Pnpm = 'pnpm',
    [int]$StartupTimeoutSeconds = 180
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
Set-StrictMode -Version Latest
if (-not $IsWindows) { throw 'This launcher requires Windows.' }
if ($MyInvocation.InvocationName -eq '.') { throw 'Run this script in a dedicated pwsh -File process.' }
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
Set-Location -LiteralPath $root
Add-Type -Path (Join-Path $PSScriptRoot 'dev-windows-job.cs')
[NexusDevelopmentJob]::Attach()
$children = [Collections.Generic.List[Diagnostics.Process]]::new()

function Default-Env([string]$Name, [string]$Value) {
    if (-not [Environment]::GetEnvironmentVariable($Name)) {
        [Environment]::SetEnvironmentVariable($Name, $Value)
    }
}
function Start-Owned([string]$Command, [string[]]$Arguments, [string]$Directory = $root) {
    $info = [Diagnostics.ProcessStartInfo]::new()
    $info.FileName = (Get-Command $Command -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
    $info.WorkingDirectory = $Directory
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    foreach ($argument in $Arguments) { $info.ArgumentList.Add($argument) }
    $process = [NexusDevelopmentJob]::Start($info)
    $children.Add($process)
    return $process
}
function Invoke-Go([string[]]$Arguments, [string]$Directory = $root) {
    $process = Start-Owned 'go' $Arguments $Directory
    $process.WaitForExit()
    if ($process.ExitCode -ne 0) { throw "go $($Arguments[0]) failed (exit $($process.ExitCode))." }
    [void]$children.Remove($process)
    $process.Dispose()
}
function Prepare-Cli {
    [void][IO.Directory]::CreateDirectory($CliBinDir)
    Invoke-Go @('build','-o', "$CliBinDir/", './cmd/nexusctl','./cmd/nexuscfg')
}
function Wait-Healthy([Diagnostics.Process]$Process, [string]$Url) {
    $deadline = [DateTime]::UtcNow.AddSeconds($StartupTimeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        if ($Process.HasExited) { throw "Service exited before becoming healthy (exit $($Process.ExitCode))." }
        try {
            $response = Invoke-WebRequest -Uri $Url -NoProxy -TimeoutSec 2
            if ($response.StatusCode -eq 200) { return }
        } catch { }
        Start-Sleep -Milliseconds 250
    }
    throw "Service did not become healthy within $StartupTimeoutSeconds seconds: $Url"
}
try {
    if ($Action -eq 'install') {
        $env:GIT_TERMINAL_PROMPT = '0'
        Invoke-Go @('mod','tidy')
        Push-Location (Join-Path $root 'web')
        try {
            # PNPM also supports the repository's default: corepack pnpm@9.15.2.
            $pnpmParts = $Pnpm -split '\s+'
            $pnpmArgs = @($pnpmParts | Select-Object -Skip 1) + @('install')
            & $pnpmParts[0] @pnpmArgs
            if ($LASTEXITCODE -ne 0) { throw 'pnpm install failed.' }
        }
        finally { Pop-Location }
        exit 0
    }
    if ($Action -eq 'prepare-dev-runtime-cli') { Prepare-Cli; exit 0 }
    $all = $Action -in @('dev','dev-nxs')
    $control = $all -or $Action -eq 'run-control'
    $backend = $all -or $Action -eq 'run-backend'
    $web = $all -or $Action -eq 'run-web'
    $ports = @()
    if ($control) { $ports += $ControlPort }
    if ($backend) { $ports += $BackendPort }
    if ($web) { $ports += $WebPort }
    if (@($ports | Sort-Object -Unique).Count -ne $ports.Count) { throw 'Development ports must be distinct.' }
    $listeners = [Net.NetworkInformation.IPGlobalProperties]::GetIPGlobalProperties().GetActiveTcpListeners()
    foreach ($port in $ports) {
        if ($port -lt 1 -or $port -gt 65535) { throw "Invalid port: $port" }
        if ($listeners.Port -contains $port) { throw "Port $port is already in use. Select another port in the make command." }
    }
    if ($control -and -not (Test-Path -LiteralPath "$ControlRoot/go.mod")) { throw "nexus-control not found: $ControlRoot" }
    $vite = Join-Path $root 'web/node_modules/vite/bin/vite.js'
    if ($web -and -not (Test-Path -LiteralPath $vite)) { throw 'Frontend dependencies missing. Run make install.' }
    if ($web) { $null = Get-Command node -CommandType Application -ErrorAction Stop }
    Default-Env 'NEXUS_STATE_ROOT' (Join-Path ([Environment]::GetFolderPath('UserProfile')) '.nexus')
    if (-not $ControlDataDir) { $ControlDataDir = Join-Path $env:NEXUS_STATE_ROOT 'control' }
    if ($Action -eq 'dev-nxs') {
        Default-Env 'NEXUS_NXS_COMMAND_PATH' $RuntimePath
        if (-not $env:NEXUS_NXS_COMMAND_PATH -or -not (Test-Path -LiteralPath $env:NEXUS_NXS_COMMAND_PATH)) {
            throw 'nxs runtime missing. Run make -C ../nexus-agent-sdk-go build-nxs.'
        }
        $env:NEXUS_AGENT_RUNTIME_KIND = 'nxs'
    }
    if ($backend) { Prepare-Cli }
    if ($control) {
        $env:CONTROL_ADDRESS = "127.0.0.1:$ControlPort"
        $env:CONTROL_DATA_DIR = $ControlDataDir
        Default-Env 'CONTROL_DATABASE_DRIVER' 'sqlite'
        Default-Env 'CONTROL_DATABASE_URL' "$ControlDataDir/data/control.db"
        $env:CONTROL_SERVICE_TOKEN_FILE = "$ControlDataDir/control-service.token"
        $env:CONTROL_SIGNING_KEY_FILE = "$ControlDataDir/control-signing.key"
        $env:CONTROL_SIGNING_PUBLIC_KEY_FILE = "$ControlDataDir/control-signing.pub"
        Default-Env 'CONTROL_SESSION_TTL_HOURS' $(if ($env:AUTH_SESSION_TTL_HOURS) { $env:AUTH_SESSION_TTL_HOURS } else { '24' })
        $service = Start-Owned 'go' @('run','./cmd/nexus-control') $ControlRoot
        Wait-Healthy $service "http://127.0.0.1:$ControlPort/api/control/v1/health"
    }
    if ($backend) {
        Default-Env 'CONNECTOR_CREDENTIALS_HOST_KEY_MODE' 'auto'
        $env:NEXUSCTL_COMMAND_PATH = "$CliBinDir/nexusctl.exe"
        $env:NEXUSCFG_COMMAND_PATH = "$CliBinDir/nexuscfg.exe"
        Default-Env 'NEXUS_CONTROL_URL' "http://127.0.0.1:$ControlPort"
        Default-Env 'NEXUS_CONTROL_SERVICE_TOKEN' $env:CONTROL_SERVICE_TOKEN
        Default-Env 'NEXUS_CONTROL_SERVICE_TOKEN_FILE' "$ControlDataDir/control-service.token"
        Default-Env 'NEXUS_CONTROL_PRINCIPAL_PUBLIC_KEY_FILE' "$ControlDataDir/control-signing.pub"
        Default-Env 'NEXUS_APP_ROOT' $root
        $env:PORT = "$BackendPort"
        $null = Start-Owned 'go' @('run','./cmd/nexus-server')
    }
    if ($web) {
        $env:VITE_BACKEND_PORT = "$BackendPort"
        $env:VITE_CONTROL_PORT = "$ControlPort"
        $null = Start-Owned 'node' @($vite,'--host','0.0.0.0','--port',"$WebPort") (Join-Path $root 'web')
    }
    Write-Host "Development services started. Frontend: http://localhost:$WebPort — Ctrl+C stops this session."
    while ($true) {
        foreach ($child in $children) {
            if ($child.HasExited) { throw "Development service $($child.Id) exited (exit $($child.ExitCode))." }
        }
        Start-Sleep -Milliseconds 250
    }
} catch {
    Write-Error $_ -ErrorAction Continue
    exit 1
} finally {
    foreach ($child in $children) {
        if (-not $child.HasExited) { try { $child.Kill($true) } catch { } }
        $child.Dispose()
    }
}
