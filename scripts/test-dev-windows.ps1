#requires -Version 7.0
$ErrorActionPreference = 'Stop'
$launcher = Join-Path $PSScriptRoot 'dev-windows.ps1'
$listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
$listener.Start()
try {
    $port = $listener.LocalEndpoint.Port
    $output = & pwsh -NoProfile -File $launcher -Action run-web -WebPort $port 2>&1
    if ($LASTEXITCODE -eq 0 -or "$output" -notmatch 'already in use') { throw "Port guard failed: $output" }
} finally { $listener.Stop() }
$output = & pwsh -NoProfile -File $launcher -Action dev -WebPort 18001 -BackendPort 18001 2>&1
if ($LASTEXITCODE -eq 0 -or "$output" -notmatch 'must be distinct') { throw "Distinct ports guard failed: $output" }
$output = & pwsh -NoProfile -File $launcher -Action run-control -ControlRoot "$PSScriptRoot/nonexistent-control" 2>&1
if ($LASTEXITCODE -eq 0 -or "$output" -notmatch 'nexus-control not found') { throw "Missing Control guard failed: $output" }

# A forcibly terminated launcher must also release its still-running descendants.
$temp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
[void][IO.Directory]::CreateDirectory($temp)
$helper = Join-Path $temp 'job parent.ps1'
$pidFile = Join-Path $temp 'child.pid'
@'
param($JobSource, $PidFile)
Add-Type -Path $JobSource
[NexusDevelopmentJob]::Attach()
$info = [Diagnostics.ProcessStartInfo]::new('pwsh')
$info.UseShellExecute = $false
$info.CreateNoWindow = $true
foreach ($arg in @('-NoProfile','-Command','Start-Sleep -Seconds 120')) { $info.ArgumentList.Add($arg) }
$child = [NexusDevelopmentJob]::Start($info)
[IO.File]::WriteAllText($PidFile, "$($child.Id)")
Start-Sleep -Seconds 120
'@ | Set-Content -LiteralPath $helper
$parent = $null
$childID = $null
try {
    $info = [Diagnostics.ProcessStartInfo]::new('pwsh')
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    foreach ($arg in @('-NoProfile','-File',$helper,(Join-Path $PSScriptRoot 'dev-windows-job.cs'),$pidFile)) { $info.ArgumentList.Add($arg) }
    $parent = [Diagnostics.Process]::Start($info)
    $deadline = [DateTime]::UtcNow.AddSeconds(30)
    while (-not (Test-Path -LiteralPath $pidFile)) {
        if ($parent.HasExited -or [DateTime]::UtcNow -gt $deadline) { throw 'Job test child failed to start.' }
        Start-Sleep -Milliseconds 100
    }
    $childID = [int](Get-Content -LiteralPath $pidFile)
    $parent.Kill()
    $parent.WaitForExit()
    Start-Sleep -Milliseconds 500
    if (Get-Process -Id $childID -ErrorAction SilentlyContinue) { throw 'Job leaked its child after launcher termination.' }
} finally {
    if ($parent -and -not $parent.HasExited) { $parent.Kill($true) }
    if ($childID) { Stop-Process -Id $childID -ErrorAction SilentlyContinue }
    Remove-Item -LiteralPath $helper, $pidFile -ErrorAction SilentlyContinue
    [IO.Directory]::Delete($temp)
}
Write-Host 'Windows development guards and process cleanup passed.'
